ALTER TABLE interviews DROP COLUMN IF EXISTS candidate_id;
ALTER TABLE interviews RENAME COLUMN interviewer_id TO user_id;

CREATE TABLE IF NOT EXISTS interview_session_items (
    id UUID PRIMARY KEY,
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    question_id UUID,
    item_type VARCHAR(20) NOT NULL CHECK (item_type IN ('question', 'task')),
    sort_order INT NOT NULL,
    text TEXT NOT NULL,
    technology VARCHAR(255),
    difficulty VARCHAR(50),
    category VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (interview_id, item_type, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_interview_session_items_interview_id ON interview_session_items(interview_id);
