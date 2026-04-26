-- +goose Up
SELECT 'up SQL query';
CREATE TABLE IF NOT EXISTS packet_summary (
    time        TIMESTAMPTZ       NOT NULL,
    length      INTEGER,
    info        TEXT
);

SELECT create_hypertable('packet_summary', 'time', if_not_exists => TRUE);

-- +goose Down
SELECT 'down SQL query';
