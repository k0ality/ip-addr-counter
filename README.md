### How I approached the task

1) Re-read info on IPv4 representation (32-bit integer) and limits (2^32) to estimate maximum unique count.
Took note to convert each line to minimize space: ~15 bytes (IP + newline) to 4 bytes (uint32).
2) Downloaded test file and checked the number of lines (wc -l) = 8B (8,000,000,000)
3) Assumed constant for all solutions:
   - I/O time = file size / SSD throughput = 120 GB / 500 MB/s = 240 seconds = 4 minutes
   - Additional memory for Go runtime, file buffer, stack/heap = ~16 MB
4) Did back of the envelope calculations for space needed for exact counting (Hash Set):
   - Memory = records * uint32 size * hash table overhead (~2) = 8B * 4 * 2 = 64 GB
5) Did back of the envelope calculations for bitmap approach:
   - Possible IPv4 count = 2^32 
   - Memory = records * 1 bit / 8 (to convert to bytes) = 2^32 * 1 / 8 = 536,870,912 bytes = 512 MB
   - CPU usage (single-threaded)
   Parsing a string, calculating the bit position, and inserting into a map are simple operations that will take nanoseconds. Let's assume we will be able to process 30M records per second.
   CPU Time = 8B / 30M = 267 sec = ~ 5 min with realistic overhead.\
   Feasible solution. But I felt like there should be a better trade-off.
6) Dug deeper. First thought about Bloom filters (not applicable here), from there headed off to linear counting algorithm -> LogLog -> HyperLogLog.
   To calculate HLL memory usage, I had to take the precision parameter into account.
   Precision determines the tradeoff between accuracy and memory.\
   b < 4 useless estimates since the error is too high\
   b > 18 diminishing returns, increased cache misses - better to switch to a bitmap solution, since we arrive at 512 MB

   | Precision (b) | Registers (m) | Memory    | Standard Error | 
   |---------------|---------------|-----------|----------------|
   | 4             | 16            | 16 B      | ~26%           |
   | 10            | 1,024         | 1 KB      | ~3.2%          |
   | 12            | 4,096         | 4 KB      | ~1.6%          |
   | **14**        | **16,384**    | **16 KB** | **~0.81%**     |
   | 16            | 65,536        | 64 KB     | ~0.40%         |
   | 18            | 262,144       | 256 KB    | ~0.20%         |
   - CPU usage (single-threaded)
   There are more operations than in the bitmap solution, and they are more complex, specifically hashing. Let's assume we will be able to process only 10M records per second.
   CPU Time = 8B / 10M = 800 sec = ~ 14 min with realistic overhead
7) Optimized for multi-threaded processing:

| Mode            | Workers | Time (estimate) | Memory | Speedup |
|-----------------|---------|-----------------|--------|---------|
| Single-threaded | 1       | ~14 min         | 16 KB  | 1x      |
| Parallel        | 4       | ~4-5 min        | 64 KB  | 3-3.5x  |
| Parallel        | 8       | ~2-3 min        | 128 KB | 5-6x    |

\
P.S. Streaming HLL & HLL++ optimizations seem to be out of scope for the given example

## Usage
```bash
make help
```

### Local
```bash
go build -o ip-counter main.go
./ip-counter input.txt 14
```

### Docker
```bash
docker build -t ip-counter .
docker run -v $(pwd)/data:/data ip-counter /data/ips.txt 14 --parallel --workers 8
```

