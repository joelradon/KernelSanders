# Use the official Golang image for building
FROM golang:1.22 AS builder
WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the main application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

# Build the diagnostic script
RUN CGO_ENABLED=0 GOOS=linux go build -o diagnostic ./utility_scripts/diagnostic.go

# Use a minimal base image for the final stage
FROM debian:stable-slim

# Install CA certificates and create a non-root user
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    useradd -ms /bin/bash appuser


# Set the working directory
WORKDIR /home/appuser/

# Copy the main application and diagnostic binaries from the builder
COPY --from=builder /app/main ./main
COPY --from=builder /app/diagnostic ./diagnostic

# Ensure binaries are executable
RUN chmod +x ./main ./diagnostic

# Switch to the non-root user
USER appuser

# Expose the port
EXPOSE 8080

# Add a HEALTHCHECK to monitor the application's health
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/ || exit 1

# Run the diagnostic script and then the main application
CMD ["sh", "-c", "./diagnostic && ./main"]
