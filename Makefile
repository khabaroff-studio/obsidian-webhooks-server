.PHONY: test test-verbose test-coverage vet fmt lint build clean

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run go vet
vet:
	go vet ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Run all quality checks
check: vet test

# Build the application
build:
	go build -o bin/obsidian-webhooks-server .

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "To install golangci-lint, run: brew install golangci-lint"
