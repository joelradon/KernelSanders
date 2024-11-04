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

# Update CA certs
RUN apt-get update && apt-get install -y ca-certificates
