package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

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