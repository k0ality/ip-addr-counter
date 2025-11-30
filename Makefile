.PHONY: build run test docker-build docker-run clean generate-test help

# Build the main application
build:
	go build -o ip-counter main.go

# Build the test data generator
build-generator:
	go build -o generate-test generate_test_data.go

# Build both
build-all: build build-generator

# Generate small test file (1M lines, 50% unique, 15MB)
generate-test-small: build-generator
	mkdir -p data
	./generate-test 1000000 0.5 data/test_small.txt

# Generate medium test file (10M lines, 30% unique, ~150MB)
generate-test-medium: build-generator
	mkdir -p data
	./generate-test 10000000 0.3 data/test_medium.txt

# Generate large test file (100M lines, 40% unique, ~1.5GB)
generate-test-large: build-generator
	mkdir -p data
	./generate-test 100000000 0.4 data/test_large.txt

# Run test with small file
test-small: build generate-test-small
	./ip-counter data/test_small.txt 14

# Run test with medium file
test-medium: build generate-test-medium
	./ip-counter data/test_medium.txt 14

# Run test with large file
test-large: build generate-test-large
	./ip-counter data/test_large.txt 14

# Docker build
docker-build:
	docker build -t ip-counter .

# Docker run (requires data/input.txt)
docker-run: docker-build
	mkdir -p data
	docker run -v $(PWD)/data:/data ip-counter /data/input.txt 14

# Generate test data and run in Docker
docker-test: docker-build build-generator
	mkdir -p data
	./generate-test 1000000 0.5 data/input.txt
	docker run -v $(PWD)/data:/data ip-counter /data/input.txt 14

# Clean build artifacts and test files
clean:
	rm -f ip-counter generate-test
	rm -rf data/

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build the IP counter"
	@echo "  build-generator    - Build the test data generator"
	@echo "  build-all          - Build both applications"
	@echo ""
	@echo "  generate-test-small   - Generate 1M line test file"
	@echo "  generate-test-medium  - Generate 10M line test file"
	@echo "  generate-test-large   - Generate 100M line test file"
	@echo ""
	@echo "  test-small         - Run test with small file"
	@echo "  test-medium        - Run test with medium file"
	@echo "  test-large         - Run test with large file"
	@echo ""
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run in Docker"
	@echo "  docker-test        - Generate test data and run in Docker"
	@echo ""
	@echo "  clean              - Remove build artifacts and test files"
	@echo "  help               - Show this help"