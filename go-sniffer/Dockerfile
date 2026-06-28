# ==========================================
# STAGE 1: Build the Go Application
# ==========================================
FROM golang:1.25-bookworm AS builder

# Install libpcap headers needed to COMPILE gopacket
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy shared module first (build context is repo root)
COPY shared/ ./shared/

# Copy go-sniffer and download dependencies
COPY go-sniffer/go.mod go-sniffer/go.sum ./go-sniffer/
RUN cd go-sniffer && go mod download

# Copy the rest of go-sniffer source
COPY go-sniffer/ ./go-sniffer/

# Build from the go-sniffer directory
RUN cd go-sniffer && CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o ../sniffer ./cmd/sniffer

# ==========================================
# STAGE 2: Lightweight Runtime Environment
# ==========================================
FROM debian:bookworm-slim

# Install ONLY the runtime libpcap library needed to RUN the application
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap0.8 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy only the compiled artifacts from the builder stage
COPY --from=builder /app/sniffer .

# Ensure the executable permissions are correct
RUN chmod +x ./sniffer

CMD ["./sniffer"]