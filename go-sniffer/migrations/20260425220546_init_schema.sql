-- +goose Up
SELECT 'up SQL query';
CREATE TABLE packet_logs (
    time        TIMESTAMPTZ NOT NULL,
    src_ip      INET,
    dst_ip      INET,
    src_port    INTEGER,
    dst_port    INTEGER,
    protocol    VARCHAR(10),
    length      INTEGER NOT NULL,
    tcp_flags   SMALLINT,
    stream_id   UUID
);

-- This transforms standard Postgres into a high-performance time-series hypertable
SELECT create_hypertable('packet_logs', 'time');

-- Create a composite index for lighting fast traffic filtering
CREATE INDEX idx_ip_search ON packet_logs (src_ip, dst_ip, time DESC);

-- +goose Down
SELECT 'down SQL query';
