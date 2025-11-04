# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
# Copy gen directory for local modules
COPY interview-service/gen/ ./gen/

COPY interview-service/go.mod interview-service/go.sum ./
RUN go mod download

# Copy source code
COPY interview-service/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o interview-service .

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh appuser
USER appuser

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/interview-service .

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:50052 || exit 1

EXPOSE 50052

CMD ["./interview-service"]
