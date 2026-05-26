# ==========================================
# STAGE 1: Build the Go Application & Goose
# ==========================================
FROM golang:1.25-bookworm AS builder

# Install libpcap headers needed to COMPILE gopacket
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Leverage Docker caching: Copy and download dependencies first.
# This prevents downloading your modules again if your source code changes!
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your Go source code
COPY . .

# Build a highly optimized, statically-linked production binary
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o sniffer ./cmd/sniffer

# Download the Goose binary separately so we can grab it in the next stage
RUN GOBIN=/app/bin go install github.com/pressly/goose/v3/cmd/goose@v3.21.1

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
COPY --from=builder /app/bin/goose /usr/local/bin/goose

# Ensure the executable permissions are correct
RUN chmod +x ./sniffer

CMD ["./sniffer"]