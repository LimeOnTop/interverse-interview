CREATE TABLE IF NOT EXISTS review_training_requests (
 user_id BIGINT NOT NULL,
 request_id UUID NOT NULL,
 source_report_id UUID NOT NULL,
 interview_id UUID REFERENCES interviews(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,request_id)
);
CREATE INDEX IF NOT EXISTS review_training_report_user ON review_training_requests(user_id,source_report_id);
