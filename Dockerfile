# Use the official Go image as the base image
FROM golang:latest AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Build the Go application with optimizations
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o main .

# Use a minimal base image for the final container
FROM scratch

# Copy the built binary from the builder stage
COPY --from=builder /app/main /

# Set the entry point for the container
ENTRYPOINT ["/main"]
