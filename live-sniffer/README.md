# live-sniffer (`nls`)

A live network packet capture tool that streams packets off a network interface and saves them to a TimescaleDB database. Designed as a daemon-based CLI — a background process manages all capture sessions and a thin client sends commands to it over a Unix socket.

## What it does

- Captures live packets from a network interface using `gopacket`
- Saves packets to TimescaleDB with session tracking (start time, end time, packets saved)
- Runs as a background daemon — multiple capture pipelines can run simultaneously
- CLI client communicates with the daemon via a Unix domain socket (`~/.config/nls/nls.sock`)

## Requirements

- Go 1.21+
- TimescaleDB (or PostgreSQL) running and accessible
- `libpcap` installed (required by `gopacket`)
  - macOS: `brew install libpcap`
  - Linux: `sudo apt install libpcap-dev`
- Root/sudo or `cap_net_raw` capability to capture packets

## Installation

```bash
# Clone and enter the project
cd live-sniffer

# Install the binary
go install ./cmd/nls/...

# Add Go's bin directory to your PATH (add this to ~/.zshrc or ~/.bashrc to make it permanent)
export PATH=$PATH:~/go/bin
```

## Setup

### 1. Configure the database URL and default interface

```bash
nls config set db-url "postgres://user:password@localhost:5432/yourdb"
nls config set interface en0
```

Config is saved to `~/.config/nls/config.yaml`.

To find your network interface on macOS:
```bash
ifconfig | grep "^[a-z]"
```
`en0` is typically WiFi on a Mac.

### 2. Start the daemon

The daemon must be running before you can start any captures. It blocks the terminal, so run it in a dedicated terminal or background it.

```bash
nls daemon start
```

## Usage

All commands communicate with the running daemon.

### Start a capture pipeline

```bash
nls start -i en0 -w 2
```

- `-i` — network interface (defaults to configured value)
- `-w` — number of parallel saver workers (1–10, default 2)

Returns a session ID you can use to stop a specific pipeline.

### List all pipelines

```bash
nls list
```

Shows all sessions in the database with their status, interface, and start time.

### Stop a specific pipeline

```bash
nls stop -s <session-id>
```

### Stop the daemon (and all pipelines)

```bash
nls daemon stop
```

This stops all running capture pipelines and shuts down the daemon.

## Architecture

```
nls <command>          nls daemon start
      |                      |
  daemon.Send()          Daemon.Start()
      |                      |
  Unix socket  <-------->  Accept()
  (~/.config/nls/nls.sock)  |
                         handleRequest()
                              |
                         pipeline.Launcher
                              |
                         gopacket → workers → TimescaleDB
```

- **Daemon** — long-running process, owns all pipeline goroutines
- **Launcher** — manages start/stop of individual capture pipelines
- **Workers** — parallel goroutines that drain the packet channel and batch-insert into the DB
- **Client** — thin wrapper that dials the socket, sends a JSON request, reads the JSON response
