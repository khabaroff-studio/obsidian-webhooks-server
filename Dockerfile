# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o webhooks-server .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/webhooks-server .
COPY schema.sql .
COPY src/templates ./src/templates
COPY plugin/release ./plugin/release
COPY static ./static

# Expose port
EXPOSE 8080

# Run the server
CMD ["./webhooks-server"]
