# Stage 1: Build static binary
FROM golang:1.25rc2-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /worker ./cmd/worker/

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static-debian12

COPY --from=builder /worker /worker

# Run as non-root (nobody:nobody)
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/worker"]
