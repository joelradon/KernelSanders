# Use a specific Go version as the builder stage
FROM golang:1.20-bookworm AS builder

# Set the working directory inside the container
WORKDIR /usr/src/app

# Copy the go.mod and go.sum files first for dependency resolution
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application files
COPY . .

# Build the application
RUN go build -o /run-app ./cmd

# Use a lightweight image for the final stage
FROM debian:bookworm
WORKDIR /
COPY --from=builder /run-app /run-app

# Command to run the application
CMD ["/run-app"]

<<<<<<< Updated upstream
# Update CA certs
RUN apt-get update && apt-get install -y ca-certificates
=======
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

# Set the BOT_USERNAME environment variable
ENV BOT_USERNAME=KernelSandersBot

# Expose the port
EXPOSE 8080

# Add a HEALTHCHECK to monitor the application's health
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/ || exit 1

# Run the diagnostic script and then the main application
CMD ["sh", "-c", "./diagnostic && ./main"]
>>>>>>> Stashed changes
