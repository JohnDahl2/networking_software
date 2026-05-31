-- +goose Up
CREATE TABLE job_tracking (
    job_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status          text,
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    source_dir      TEXT,
    total_files     INTEGER,
    files_read      INTEGER
);

-- +goose Down
DROP TABLE IF EXISTS job_tracking CASCADE;
