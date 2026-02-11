# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies if needed
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# Disable CGO for a static binary (already using pure Go SQLite)
RUN CGO_ENABLED=0 GOOS=linux go build -o opencron .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary and static files from the builder stage
COPY --from=builder /app/opencron .
COPY --from=builder /app/static ./static

# Expose the default port
EXPOSE 8080

# Set environment variables
ENV PORT=8080

# Command to run the application
# Note: The database 'opencron.db' and 'logs' directory will be created in /app. 
# You should mount volumes for 'opencron.db' and 'logs' for persistence.
CMD ["./opencron"]
