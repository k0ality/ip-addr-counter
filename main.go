package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

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