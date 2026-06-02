-- +goose Up
CREATE TABLE source_files (
    file_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       UUID REFERENCES job_tracking(job_id),
    file_path    TEXT,
    checksum     TEXT UNIQUE,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS source_files CASCADE;
