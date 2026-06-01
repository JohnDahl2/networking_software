# go-sniffer

A high-performance pcap extraction and query API built in Go. Designed as a forensic analysis tool — drop pcap files into a directory, trigger an extraction job via the API, and query the resulting packet data with flexible filtering, sorting, and cursor-based pagination.

Built as a faster, more analytical replacement for Wireshark's data layer.

---

## Architecture

```
pcap files → extraction pipeline → TimescaleDB → REST API → frontend / analytics
```

- **Extraction pipeline** — parallel worker pool reads pcap files, batches packets, and bulk-inserts into Postgres via `COPY`
- **Storage** — TimescaleDB (Postgres extension) hypertable partitioned by time for fast time-series queries
- **API** — Chi-based REST API with dynamic field selection, filtering, cursor pagination, and job tracking

---

## Requirements

- Docker + Docker Compose (recommended)
- OR: Go 1.25+, libpcap, PostgreSQL with TimescaleDB extension

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/sniffer?sslmode=disable` | Postgres connection string |
| `PCAP_SOURCE_DIR` | `data/dumb_data` | Directory containing `.pcap` files to extract |
| `DEMO_MODE` | `false` | Set to `true` to auto-generate dummy pcap files on startup |
| `LOG_LEVEL` | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |

---

## Running with Docker

Build and start:

```bash
docker build -t go-sniffer .
docker run -p 3000:3000 \
  -e DATABASE_URL=postgres://postgres:postgres@host.docker.internal:5432/sniffer?sslmode=disable \
  -v /path/to/your/pcaps:/app/data/pcaps \
  -e PCAP_SOURCE_DIR=data/pcaps \
  go-sniffer
```

To generate dummy test data on startup:

```bash
docker run -p 3000:3000 -e DEMO_MODE=true go-sniffer
```

---

## Running Locally

```bash
# Install dependencies
go mod download

# Run database migrations
goose -dir migrations postgres "$DATABASE_URL" up

# Start the server
go run ./cmd/sniffer
```

---

## Database Migrations

Migrations are managed with [Goose](https://github.com/pressly/goose) and run automatically on startup.

To run manually:

```bash
goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/sniffer?sslmode=disable" up
```

To rollback:

```bash
goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/sniffer?sslmode=disable" down
```

---

## API Reference

Base URL: `http://localhost:3000/api/v1`

---

### Packets

#### `GET /packets`

Query extracted packet data with optional filtering, field selection, sorting, and pagination.

**Query Parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | 100 | Number of results (max 100) |
| `fields` | string | all except stream_id | Comma-separated list of fields to return |
| `order` | string | `asc` | Sort order: `asc` or `desc` |
| `cursor` | string | — | RFC3339 timestamp for cursor-based pagination |
| `filter` | string | — | Filter expression: `field:op:value` (repeatable) |

**Valid fields:** `time`, `src_ip`, `dst_ip`, `src_port`, `dst_port`, `protocol`, `length`, `tcp_flags`, `stream_id`

**Valid filter operators:** `eq`, `ne`, `gt`, `lt`, `gte`, `lte`

**Examples**

```bash
# Default — last 100 packets
GET /api/v1/packets

# Select specific fields
GET /api/v1/packets?fields=src_ip,dst_ip,protocol,length

# Filter by protocol
GET /api/v1/packets?filter=protocol:eq:TCP

# Multiple filters
GET /api/v1/packets?filter=protocol:eq:UDP&filter=length:gt:1000

# Paginate — pass next_cursor from previous response
GET /api/v1/packets?cursor=2026-03-07T14:31:40Z&limit=100

# Combined
GET /api/v1/packets?fields=src_ip,dst_ip,length&filter=protocol:eq:TCP&order=desc&limit=10
```

**Response**

```json
{
  "data": [
    {
      "time": "2026-03-07T14:30:00Z",
      "src_ip": "10.5.7.113",
      "dst_ip": "54.90.39.78",
      "protocol": "TCP",
      "length": 512
    }
  ],
  "next_cursor": "2026-03-07T14:31:40Z"
}
```

`next_cursor` is `null` when you have reached the last page.

---

### Jobs

Extraction jobs process pcap files from the configured source directory and insert packets into the database. Each packet is tagged with the job's UUID via `stream_id`, allowing you to query packets by job.

#### `GET /jobs`

List all extraction jobs ordered by most recent first.

```bash
GET /api/v1/jobs
```

**Response**

```json
[
  {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "COMPLETED",
    "progress_pct": 100,
    "started_at": "2026-05-31T21:30:58Z",
    "completed_at": "2026-05-31T21:35:12Z",
    "source_dir": "data/dumb_data",
    "total_files": 10,
    "files_read": 10
  }
]
```

**Job statuses:** `PROCESSING`, `COMPLETED`, `FAILED`

---

#### `POST /jobs`

Start a new extraction job. Returns immediately with `202 Accepted` while extraction runs in the background.

```bash
POST /api/v1/jobs
POST /api/v1/jobs?source_dir=data/my_captures
```

**Query Parameters**

| Parameter | Default | Description |
|---|---|---|
| `source_dir` | `data/dumb_data` | Directory containing `.pcap` files to extract |

**Response** — `202 Accepted`

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "PROCESSING",
  "progress_pct": 0,
  "started_at": "2026-05-31T21:30:58Z",
  "completed_at": null,
  "source_dir": "data/dumb_data",
  "total_files": 10,
  "files_read": 0
}
```

---

#### `GET /jobs/{job_id}`

Poll the status of a specific extraction job.

```bash
GET /api/v1/jobs/550e8400-e29b-41d4-a716-446655440000
```

---

#### `DELETE /jobs/{job_id}`

Cancel a running job (if still in progress) and delete all associated packets and the job record.

```bash
DELETE /api/v1/jobs/550e8400-e29b-41d4-a716-446655440000
```

**Response** — `204 No Content`

---

## Querying Packets by Job

Since every packet is tagged with the job's UUID via `stream_id`, you can query all packets from a specific extraction:

```bash
GET /api/v1/packets?fields=src_ip,dst_ip,protocol,length,stream_id&filter=stream_id:eq:550e8400-e29b-41d4-a716-446655440000
```

---

## Running Tests

```bash
go test ./...
```

---

## Project Structure

```
cmd/sniffer/        — application entrypoint
internal/
  api/              — HTTP handlers, routing, request validation
  storage/          — database layer, migrations, job tracking
  worker/           — pcap reader workers, packet saver workers, pipeline orchestration
  pcap/             — pcap file generation (demo/testing only)
migrations/         — Goose SQL migration files
```

---

## Known Limitations & Future Work

- **Duplicate file detection** — no checksum validation yet; submitting the same pcap file twice will insert duplicate packets
- **File source tracking** — a `source_files` table to track which files have been processed is planned
- **Data retention** — TimescaleDB `drop_chunks()` policy for automatic data expiry is not yet configured
- **Authentication** — no auth layer; intended for local/trusted network use only
