-- Keep the expanded constraint and history so rolling back cannot invalidate saved plans.
ALTER TABLE practice_reminders ALTER COLUMN lead_minutes SET DEFAULT 10;
