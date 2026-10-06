-- Append-only log of created trainings. Plan quotas count rows here, so
-- deleting an interview does not give the training back.
CREATE TABLE IF NOT EXISTS interview_quota_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    interview_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_interview_quota_log_user_created
    ON interview_quota_log (user_id, created_at);

INSERT INTO interview_quota_log (user_id, interview_id, created_at)
SELECT user_id, id, created_at FROM interviews;
