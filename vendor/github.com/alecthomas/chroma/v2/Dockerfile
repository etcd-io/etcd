# Multi-stage Dockerfile for chromad Go application using Hermit-managed tools

# Build stage
FROM ubuntu:26.04 AS builder

# Install system dependencies
RUN apt-get update && apt-get install -y \
	curl \
	git \
	ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy the entire project (including bin directory with Hermit tools)
COPY . .

# Make Hermit tools executable and add to PATH
ENV PATH="/app/bin:${PATH}"

# Set Go environment variables for static compilation
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Build the application using just
RUN just chromad

# Runtime stage
FROM alpine:3.23 AS runtime

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates curl

# Create a non-root user
RUN addgroup -g 1001 chromad && \
	adduser -D -s /bin/sh -u 1001 -G chromad chromad

# Set working directory
WORKDIR /app

# Copy the binary from build stage
COPY --from=builder /app/build/chromad /app/chromad

# Change ownership to non-root user
RUN chown chromad:chromad /app/chromad

# Switch to non-root user
USER chromad

# Expose port (default is 8080, but can be overridden via PORT env var)
EXPOSE 8080

# Set default environment variables
ENV PORT=8080
ENV CHROMA_CSRF_KEY="testtest"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
	CMD curl -fsSL http://127.0.0.1:8080/ > /dev/null

# Run the application
CMD ["sh", "-c", "./chromad --csrf-key=$CHROMA_CSRF_KEY --bind=0.0.0.0:$PORT"]
