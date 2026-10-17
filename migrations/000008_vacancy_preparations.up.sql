CREATE TABLE IF NOT EXISTS vacancy_preparations (
 id UUID PRIMARY KEY, user_id BIGINT NOT NULL, vacancy_key TEXT NOT NULL,
 payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(user_id, vacancy_key)
);
CREATE TABLE IF NOT EXISTS vacancy_preparation_attempts (
 preparation_id UUID NOT NULL REFERENCES vacancy_preparations(id) ON DELETE CASCADE,
 step_index INT NOT NULL CHECK(step_index >= 0), request_id UUID NOT NULL,
 interview_id UUID REFERENCES interviews(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(preparation_id, request_id)
);
CREATE INDEX IF NOT EXISTS vacancy_preparation_attempt_interview ON vacancy_preparation_attempts(interview_id);
CREATE INDEX IF NOT EXISTS vacancy_preparations_user ON vacancy_preparations(user_id, created_at DESC);
