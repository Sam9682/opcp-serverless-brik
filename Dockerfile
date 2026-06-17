# Stage 1: Build static binary
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /worker \
    ./cmd/worker/

# Stage 2: Minimal runtime image with shell access
FROM alpine:3.20

# Install ca-certificates for HTTPS and create non-root user
RUN apk --no-cache add ca-certificates

COPY --from=builder /worker /worker

# Run as non-root (nobody:nobody)
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/worker"]
