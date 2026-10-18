ALTER TABLE practice_reminders ADD COLUMN IF NOT EXISTS email_retry_at TIMESTAMPTZ;
ALTER TABLE practice_reminders ADD COLUMN IF NOT EXISTS email_error TEXT NOT NULL DEFAULT '';
