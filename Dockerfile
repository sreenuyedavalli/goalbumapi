# Build stage
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with support for multiple architectures
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# Set build arguments for cross-compilation
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=0

# Build the application
RUN go build -o goalbumapi

# Final stage
FROM --platform=$TARGETPLATFORM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/goalbumapi .

# Copy static files
COPY --from=builder /app/views ./views

# Expose port
EXPOSE 3000

# Set environment variables
ENV GIN_MODE=debug

# Run the application
CMD ["./goalbumapi"] 