# Stage 1: Build the binaries
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Install build dependencies if needed (e.g., git)
RUN apk add --no-cache git

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# BUILD WITH CACHE MOUNTS
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o auth-server ./cmd/server/main.go && \
    go build -o migrator ./cmd/migrator/main.go

# Stage 2: Final lightweight image
FROM alpine:latest
WORKDIR /root/

# Copy binaries from the builder stage
COPY --from=builder /app/auth-server .
COPY --from=builder /app/migrator .

# Copy configuration and migration files
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/migrations ./migrations

# Expose gRPC and HTTP ports
EXPOSE 50001 8081

# Command to run
CMD ["./auth-server"]