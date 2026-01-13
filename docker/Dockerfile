# Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build


# Build Api server
FROM golang:1.24.5-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy frontend build output into embed path
COPY --from=frontend-builder /frontend/dist ./internal/frontend/dist

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o whatomate ./cmd/whatomate


# Final stage
FROM alpine:3.19
WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/whatomate .

# Copy config example (will be overridden if mounted file exists or by env vars in production)
COPY --from=builder /app/config.example.toml ./config.toml

# Create uploads directory
RUN mkdir -p /app/uploads

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./whatomate", "server", "-config", "config.toml", "-migrate"]
