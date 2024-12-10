# Makefile for AskMe CLI

# Define the binary name and output directory
BINARY_NAME=askme
OUTPUT_DIR=bin

# Default target
all: build

# Build the binary and move it to the bin/ folder
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(OUTPUT_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete. Binary is located at $(OUTPUT_DIR)/$(BINARY_NAME)"

# Clean the bin/ directory
clean:
	@echo "Cleaning up..."
	@rm -f $(OUTPUT_DIR)/$(BINARY_NAME)
	@echo "Cleanup complete."

# Run the binary
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(OUTPUT_DIR)/$(BINARY_NAME)

.PHONY: all build clean run

