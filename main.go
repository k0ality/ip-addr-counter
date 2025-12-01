package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HyperLogLog struct {
	precision uint8   // Number of bits used for bucketing (b)
	m         uint32  // Number of counters (or "registers")
	registers []uint8 // Array of registers
	alphaMM   float64 // Bias correction constant * harmonic mean transformation across m buckets
}

func NewHyperLogLog(precision uint8) *HyperLogLog {
	if precision < 4 || precision > 18 {
		panic("precision must be between 4 and 18.")
	}

	m := uint32(1 << precision)

	var alpha float64
	switch m {
	case 16:
		alpha = 0.673
	case 32:
		alpha = 0.697
	case 64:
		alpha = 0.709
	default:
		alpha = 0.7213 / (1 + 1.079 / float64(m))
	}

	return &HyperLogLog{
		precision: precision,
		m:         m,
		registers: make([]uint8, m),
		alphaMM:   alpha * float64(m) * float64(m),
	}
}

func (h *HyperLogLog) Add(item uint32) {
	hash := hash32(item)
	bucketIdx := hash >> (32 - h.precision)
	remainingBits := hash << h.precision
	leadingZeros := uint8(bits.LeadingZeros32(remainingBits)) + 1
	if leadingZeros > h.registers[bucketIdx] {
		h.registers[bucketIdx] = leadingZeros
	}
}

func (h *HyperLogLog) Merge(other *HyperLogLog) error {
	if h.precision != other.precision {
		return fmt.Errorf("cannot merge HLLs with different precision")
	}

	for i := range h.registers {
		if other.registers[i] > h.registers[i] {
			h.registers[i] = other.registers[i]
		}
	}

	return nil
}

func (h *HyperLogLog) Count() uint64 {
	sum := 0.0
	zeros := 0

	for _, val := range h.registers {
		sum += 1.0 / float64(uint64(1)<<val)
		if val == 0 {
			zeros++
		}
	}

	estimate := h.alphaMM / sum

	if estimate <= 2.5 * float64(h.m) {
		if zeros != 0 {
			estimate = float64(h.m) * math.Log(float64(h.m) / float64(zeros))
		}
	} else if estimate <= (1.0 / 30.0) * math.Pow(2, 32) {
	} else {
		estimate = -math.Pow(2, 32) * math.Log(1 - estimate / math.Pow(2, 32))
	}

	return uint64(estimate)
}

func hash32(key uint32) uint32 {
	// MurmurHash3 finalizer mix
	key ^= key >> 16
	key *= 0x85ebca6b
	key ^= key >> 13
	key *= 0xc2b2ae35
	key ^= key >> 16
	return key
}

func convertIPv4(ip string) (uint32, bool) {
	var octets [4]byte
	octetIdx := 0
	currentOctet := 0

	for i := 0; i < len(ip); i++ {
		c := ip[i]
		if c >= '0' && c <= '9' {
			currentOctet = currentOctet * 10 + int(c - '0')
			if currentOctet > 255 {
				return 0, false
			}
		} else if c == '.' {
			if octetIdx >= 3 {
				return 0, false
			}
			octets[octetIdx] = byte(currentOctet)
			octetIdx++
			currentOctet = 0
		} else {
			return 0, false
		}
	}

	if octetIdx != 3 {
		return 0, false
	}
	octets[3] = byte(currentOctet)

	return binary.BigEndian.Uint32(octets[:]), true
}

func processFileChunk(filename string, startPos, endPos int64, precision uint8, workerID int,
	totalLines *uint64, mu *sync.Mutex) (*HyperLogLog, error) {

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hll := NewHyperLogLog(precision)

	if _, err := file.Seek(startPos, 0); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024 * 1024)
	scanner.Buffer(buf, 1024 * 1024)

	if startPos != 0 {
		scanner.Scan()
	}

	currentPos := startPos
	if startPos != 0 && scanner.Text() != "" {
		currentPos += int64(len(scanner.Text())) + 1
	}

	localCount := uint64(0)

	for scanner.Scan() {
		lineStart := currentPos
		line := scanner.Text()
		lineLen := int64(len(line)) + 1
		currentPos += lineLen

		if lineStart >= endPos {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		ip, valid := convertIPv4(line)
		if valid {
			hll.Add(ip)
			localCount++

			if localCount % 1000000 == 0 {
				mu.Lock()
				*totalLines += 1000000
				mu.Unlock()
			}
		}
	}

	remainder := localCount % 1000000
	if remainder > 0 {
		mu.Lock()
		*totalLines += remainder
		mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return hll, nil
}

func processFileParallel(filename string, precision uint8, numWorkers int) (uint64, uint64, time.Duration, error) {
	startTime := time.Now()

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, 0, 0, err
	}
	fileSize := fileInfo.Size()

	if numWorkers < 1 {
		numWorkers = 1
	}

	chunkSize := fileSize / int64(numWorkers)
	if chunkSize == 0 {
		chunkSize = fileSize
		numWorkers = 1
	}

	fmt.Printf("Processing %d bytes with %d worker(s) (%.2f MB per worker)...\n",
		fileSize, numWorkers, float64(chunkSize) / (1024 * 1024))

	var wg sync.WaitGroup
	results := make([]*HyperLogLog, numWorkers)
	errors := make([]error, numWorkers)
	processedLines := uint64(0)
	var mu sync.Mutex

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				current := processedLines
				mu.Unlock()
				if current > 0 {
					elapsed := time.Since(startTime)
					rate := float64(current) / elapsed.Seconds()
					fmt.Printf("  Progress: %d lines processed (%.2f M lines/sec)\n",
						current, rate / 1000000)
				}
			case <-done:
				return
			}
		}
	}()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)

		startPos := int64(i) * chunkSize
		endPos := startPos + chunkSize
		if i == numWorkers - 1 {
			endPos = fileSize
		}

		go func(workerID int, start, end int64) {
			defer wg.Done()
			hll, err := processFileChunk(filename, start, end, precision, workerID, &processedLines, &mu)
			results[workerID] = hll
			errors[workerID] = err
		}(i, startPos, endPos)
	}

	wg.Wait()
	close(done)

	for i, err := range errors {
		if err != nil {
			return 0, 0, 0, fmt.Errorf("worker %d error: %w", i, err)
		}
	}

	fmt.Println("\nMerging results...")
	startMerge := time.Now()

	finalHLL := results[0]
	for i := 1; i < len(results); i++ {
		if results[i] != nil {
			finalHLL.Merge(results[i])
		}
	}

	mergeTime := time.Since(startMerge)
	elapsed := time.Since(startTime)

	fmt.Printf("Merge completed in %v\n", mergeTime)

	mu.Lock()
	totalLines := processedLines
	mu.Unlock()

	return finalHLL.Count(), totalLines, elapsed, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input_file> [precision] [--workers N]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nParameters:\n")
		fmt.Fprintf(os.Stderr, "  input_file  - path to file with IPv4 addresses (one per line)\n")
		fmt.Fprintf(os.Stderr, "  precision   - HyperLogLog precision (default: 14)\n")
		fmt.Fprintf(os.Stderr, "  --workers N - number of workers (default: 1 for single-threaded)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s data.txt                  # Single worker (sequential)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s data.txt 16               # Single worker, precision=16\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s data.txt --workers 4      # 4 workers (parallel)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s data.txt 14 --workers 8   # 8 workers, precision=14\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	precision := uint8(14)
	numWorkers := 1

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--workers":
			if i + 1 < len(os.Args) {
				w, err := strconv.Atoi(os.Args[i + 1])
				if err == nil && w > 0 {
					numWorkers = w
					i++
				}
			}
		default:
			p, err := strconv.Atoi(os.Args[i])
			if err == nil && p >= 4 && p <= 18 {
				precision = uint8(p)
			}
		}
	}

	m := uint32(1 << precision)
	fmt.Printf("HyperLogLog initialized with precision=%d (%d registers, ~%d bytes)\n",
		precision, m, m)
	fmt.Printf("Expected standard error: ~%.2f%%\n", 104.0 / math.Sqrt(float64(m)))
	fmt.Printf("Workers: %d\n", numWorkers)
	fmt.Printf("Memory: ~%d KB per worker, ~%d KB total\n\n", m / 1024, m * uint32(numWorkers) / 1024)

	uniqueCount, totalLines, elapsed, err := processFileParallel(filename, precision, numWorkers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	const maxIPv4 = uint64(1) << 32
	if uniqueCount > maxIPv4 {
	    panic("uniqueCount can not be over IPv4 upper bound.")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("RESULTS")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total lines processed:    %d\n", totalLines)
	fmt.Printf("Unique IP addresses:      %d\n", uniqueCount)
	fmt.Printf("Processing time:          %v\n", elapsed)
	fmt.Printf("Processing rate:          %.2f M lines/sec\n", float64(totalLines) / elapsed.Seconds() / 1000000)
	fmt.Printf("Workers used:             %d\n", numWorkers)
	fmt.Printf("Memory used:              ~%d KB\n", m * uint32(numWorkers) / 1024)
	fmt.Println(strings.Repeat("=", 60))
}