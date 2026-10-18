CREATE TABLE IF NOT EXISTS practice_reminders (
 id UUID PRIMARY KEY, user_id BIGINT NOT NULL, target_key TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('interview','preparation','review','draft')),
 target_id TEXT NOT NULL, step_index INT NOT NULL DEFAULT 0, title TEXT NOT NULL,
 payload JSONB NOT NULL DEFAULT '{}', scheduled_at TIMESTAMPTZ NOT NULL,
 timezone TEXT NOT NULL DEFAULT 'Europe/Moscow', email_enabled BOOLEAN NOT NULL DEFAULT FALSE,
 lead_minutes INT NOT NULL DEFAULT 10 CHECK(lead_minutes IN (0,10,30)),
 state TEXT NOT NULL DEFAULT 'scheduled' CHECK(state IN ('scheduled','started','dismissed')),
 revision INT NOT NULL DEFAULT 1, email_sent_at TIMESTAMPTZ, email_attempts INT NOT NULL DEFAULT 0,
 email_failed BOOLEAN NOT NULL DEFAULT FALSE, started_interview_id UUID,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(user_id,target_key)
);
CREATE INDEX IF NOT EXISTS practice_reminders_due ON practice_reminders(scheduled_at) WHERE state='scheduled';
CREATE TABLE IF NOT EXISTS user_news_reads (
 user_id BIGINT NOT NULL, news_id TEXT NOT NULL, read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(user_id,news_id)
);

CREATE TABLE IF NOT EXISTS practice_reminder_deliveries(user_id BIGINT NOT NULL,reminder_id UUID NOT NULL,revision INT NOT NULL,sent_at TIMESTAMPTZ NOT NULL,PRIMARY KEY(reminder_id,revision));
CREATE INDEX IF NOT EXISTS practice_reminder_deliveries_user ON practice_reminder_deliveries(user_id,sent_at);
