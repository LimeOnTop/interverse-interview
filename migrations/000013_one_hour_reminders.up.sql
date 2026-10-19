ALTER TABLE practice_reminders DROP CONSTRAINT IF EXISTS practice_reminders_lead_minutes_check;
ALTER TABLE practice_reminders ADD CONSTRAINT practice_reminders_lead_minutes_check CHECK(lead_minutes IN (0,10,30,60));
ALTER TABLE practice_reminders ALTER COLUMN lead_minutes SET DEFAULT 60;
-- Preserve delivery history. Do not send a newly moved reminder immediately when the hour has passed.
UPDATE practice_reminders SET
 lead_minutes=60,
 email_enabled=CASE WHEN email_sent_at IS NULL AND scheduled_at<=NOW()+INTERVAL '1 hour' THEN FALSE ELSE email_enabled END,
 revision=revision+1,updated_at=NOW()
WHERE state='scheduled' AND lead_minutes<>60;
