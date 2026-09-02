ALTER TABLE interview_session_items ADD COLUMN IF NOT EXISTS options JSONB NOT NULL DEFAULT '[]';
