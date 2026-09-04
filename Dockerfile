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

ARG TARGETARCH
ENV MIGRATIONS_TABLE=interview_schema_migrations
ENV MIGRATE_PATH=/migrations

RUN apk --no-cache add ca-certificates tzdata curl \
	&& curl -fsSL "https://github.com/golang-migrate/migrate/releases/download/v4.18.3/migrate.linux-${TARGETARCH}.tar.gz" \
		| tar -xz -C /usr/local/bin \
	&& chmod +x /usr/local/bin/migrate \
	&& apk del curl \
	&& adduser -D -s /bin/sh appuser

WORKDIR /app

COPY migrations/ /migrations/
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=builder /app/interview-service .

RUN chmod +x /docker-entrypoint.sh \
	&& chown -R appuser:appuser /app /migrations

USER appuser

EXPOSE 50052
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["./interview-service"]
