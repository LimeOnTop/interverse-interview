# syntax=docker/dockerfile:1.4

FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
COPY --from=contracts . /contracts
RUN go mod edit -replace=github.com/LimeOnTop/interverse-contracts=/contracts
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -o interview-service ./cmd

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata \
	&& adduser -D -s /bin/sh appuser

WORKDIR /app

COPY --from=builder /app/interview-service .

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 50052
CMD ["./interview-service"]
