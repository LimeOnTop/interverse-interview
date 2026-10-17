ALTER TABLE interviews ADD COLUMN IF NOT EXISTS task_mode TEXT NOT NULL DEFAULT 'few' CHECK (task_mode IN ('theory', 'few', 'many'));
