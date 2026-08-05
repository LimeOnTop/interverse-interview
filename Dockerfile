# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY interverse-interview/go.mod interverse-interview/go.sum ./
COPY interverse-contracts /interverse-contracts
RUN go mod edit -replace=github.com/LimeOnTop/interverse-contracts=/interverse-contracts
RUN go mod download

COPY interverse-interview/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o interview-service .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
RUN adduser -D -s /bin/sh appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/interview-service .
EXPOSE 50052
CMD ["./interview-service"]
