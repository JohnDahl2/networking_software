-- Enforce that every job always has a status
ALTER TABLE job_tracking
    ALTER COLUMN status SET NOT NULL;

-- Link packets to their job; cascade so DeleteJob cleans up automatically
ALTER TABLE packet_logs
    ADD CONSTRAINT fk_packet_logs_job_id
    FOREIGN KEY (job_id) REFERENCES job_tracking(job_id)
    ON DELETE CASCADE;

-- Index job_id so filtering packets by job is fast
CREATE INDEX idx_packet_logs_job_id ON packet_logs (job_id);
