CREATE TABLE capture_sessions (
    session_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    interface    TEXT NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL,
    ended_at     TIMESTAMPTZ,
    status       TEXT NOT NULL,
    packets_saved BIGINT DEFAULT 0
);