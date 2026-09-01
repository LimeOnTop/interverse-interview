DROP TABLE IF EXISTS interview_session_items;

ALTER TABLE interviews RENAME COLUMN user_id TO interviewer_id;
ALTER TABLE interviews ADD COLUMN IF NOT EXISTS candidate_id UUID;
