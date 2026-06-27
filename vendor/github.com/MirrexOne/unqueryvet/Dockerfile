# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binaries
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /unqueryvet ./cmd/unqueryvet
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /unqueryvet-lsp ./cmd/unqueryvet-lsp

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 unqueryvet
USER unqueryvet

WORKDIR /workspace

# Copy binaries
COPY --from=builder /unqueryvet /usr/local/bin/
COPY --from=builder /unqueryvet-lsp /usr/local/bin/

# Default command
ENTRYPOINT ["unqueryvet"]
CMD ["./..."]

# Labels
LABEL org.opencontainers.image.title="Unqueryvet"
LABEL org.opencontainers.image.description="SQL SELECT * linter for Go"
LABEL org.opencontainers.image.source="https://github.com/MirrexOne/unqueryvet"
LABEL org.opencontainers.image.licenses="MIT"
