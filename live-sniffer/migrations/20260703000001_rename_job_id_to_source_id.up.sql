-- Drop the FK constraint tying packet_logs to job_tracking only
ALTER TABLE packet_logs DROP CONSTRAINT fk_packet_logs_job_id;

-- Drop the old job_id index
DROP INDEX IF EXISTS idx_packet_logs_job_id;

-- Rename job_id to source_id — now accepts either a job UUID or a session UUID
ALTER TABLE packet_logs RENAME COLUMN job_id TO source_id;

-- Recreate index under new name
CREATE INDEX idx_packet_logs_source_id ON packet_logs (source_id);
