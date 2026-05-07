# Build stage
FROM golang:1.26-alpine AS builder

# Install git and ca-certificates (needed for downloading dependencies)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download


# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime dependencies.
RUN apk --no-cache add ca-certificates git

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .


# Expose port
EXPOSE 4001

# Run the binary
CMD ["./main"]
