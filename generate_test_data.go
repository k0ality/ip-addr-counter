package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

func generateTestData(numLines int, uniqueRatio float64, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	numUnique := int(float64(numLines) * uniqueRatio)
	if numUnique == 0 {
		numUnique = 1
	}

	uniqueIPs := make([]string, numUnique)
	for i := 0; i < numUnique; i++ {
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rng.Intn(256),
			rng.Intn(256),
			rng.Intn(256),
			rng.Intn(256))
		uniqueIPs[i] = ip
	}

	fmt.Printf("Generating %d lines with %d unique IPs (%.1f%% unique)...\n",
		numLines, numUnique, uniqueRatio*100)

	for i := 0; i < numLines; i++ {
		idx := rng.Intn(numUnique)
		writer.WriteString(uniqueIPs[idx])
		writer.WriteByte('\n')

		if (i+1)%1000000 == 0 {
			fmt.Printf("  Generated %dM lines...\n", (i+1)/1000000)
		}
	}

	fmt.Printf("Done! File: %s\n", filename)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <num_lines> [unique_ratio] [output_file]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s 1000000                    # 1M lines, 50%% unique, output to test.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 10000000 0.3               # 10M lines, 30%% unique\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 100000000 0.8 large.txt    # 100M lines, 80%% unique, custom filename\n", os.Args[0])
		os.Exit(1)
	}

	numLines, err := strconv.Atoi(os.Args[1])
	if err != nil || numLines <= 0 {
		fmt.Fprintf(os.Stderr, "Error: num_lines must be a positive integer\n")
		os.Exit(1)
	}

	uniqueRatio := 0.5
	if len(os.Args) >= 3 {
		uniqueRatio, err = strconv.ParseFloat(os.Args[2], 64)
		if err != nil || uniqueRatio <= 0 || uniqueRatio > 1 {
			fmt.Fprintf(os.Stderr, "Error: unique_ratio must be between 0 and 1\n")
			os.Exit(1)
		}
	}

	filename := "test.txt"
	if len(os.Args) >= 4 {
		filename = os.Args[3]
	}

	startTime := time.Now()
	err = generateTestData(numLines, uniqueRatio, filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Generation took: %v\n", elapsed)

	info, _ := os.Stat(filename)
	fmt.Printf("File size: %.2f MB\n", float64(info.Size())/(1024*1024))
}