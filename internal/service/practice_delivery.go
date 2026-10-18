package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

// Only errors known to precede acceptance may be retried automatically.
type PracticeDeliveryError struct {
	Err                  error
	Retryable, Uncertain bool
}

func (e *PracticeDeliveryError) Error() string { return e.Err.Error() }
func (e *PracticeDeliveryError) Unwrap() error { return e.Err }

func (s *PracticeStateService) retryMail(ctx context.Context, user string, p PracticeReminder) (any, error) {
	if _, e := uuid.Parse(p.ID); e != nil {
		return nil, status.Error(codes.NotFound, "Напоминание не найдено")
	}
	if s.sender == nil {
		return nil, status.Error(codes.FailedPrecondition, "Письма временно недоступны")
	}
	r, e := s.db.ExecContext(ctx, `UPDATE practice_reminders SET email_attempts=0,email_failed=FALSE,email_retry_at=NOW(),email_error='',revision=revision+1,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND revision=$3 AND state='scheduled' AND email_enabled AND email_failed AND email_sent_at IS NULL AND scheduled_at>NOW()-INTERVAL '30 minutes'`, p.ID, user, p.Revision)
	if e != nil {
		return nil, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return nil, status.Error(codes.FailedPrecondition, "Письмо уже отправлено, план изменился или время прошло. Обновите список либо перенесите практику")
	}
	return map[string]bool{"ok": true}, nil
}

func (s *PracticeStateService) DispatchOne(ctx context.Context, now time.Time) (bool, error) {
	if s.sender == nil {
		return false, nil
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT `+reminderColumns+`,user_id FROM practice_reminders WHERE state='scheduled' AND email_enabled AND email_sent_at IS NULL AND NOT email_failed AND email_attempts<3 AND (email_retry_at IS NULL OR email_retry_at<=$1) AND scheduled_at-lead_minutes*INTERVAL '1 minute'<=$1 AND scheduled_at>=$1-INTERVAL '30 minutes' ORDER BY scheduled_at FOR UPDATE SKIP LOCKED LIMIT 1`, now)
	var p PracticeReminder
	var raw []byte
	var user string
	e = row.Scan(&p.ID, &p.Kind, &p.TargetID, &p.StepIndex, &p.Title, &raw, &p.ScheduledAt, &p.Timezone, &p.EmailEnabled, &p.LeadMinutes, &p.Revision, &p.EmailFailed, &p.EmailSent, &p.EmailRetryAt, &p.EmailError, &p.EmailAttempts, &user)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	// Avoid the row/user lock inversion with a concurrent reschedule.
	var locked bool
	if e = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))`, "reminder:"+user).Scan(&locked); e != nil {
		return false, e
	}
	if !locked {
		return false, nil
	}
	var sent int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM practice_reminder_deliveries WHERE user_id=$1 AND sent_at>$2`, user, now.Add(-24*time.Hour)).Scan(&sent); e != nil {
		return false, e
	}
	attempts := p.EmailAttempts + 1
	failed := false
	reason := ""
	var retryAt *time.Time
	if sent >= 5 {
		failed = true
		reason = "limit"
	} else if sendErr := s.sender.Send(ctx, PracticeMail{UserID: user, Title: p.Title, Path: "/dashboard?reminder=" + p.ID, ScheduledAt: p.ScheduledAt, Timezone: p.Timezone}); sendErr != nil {
		reason = "delivery_failed"
		var delivery *PracticeDeliveryError
		if errors.As(sendErr, &delivery) {
			if delivery.Uncertain {
				reason = "uncertain"
			}
			if delivery.Retryable && attempts < 3 {
				next := now.Add(time.Duration(attempts) * time.Minute)
				if next.Before(p.ScheduledAt.Add(30 * time.Minute)) {
					retryAt = &next
					reason = "retrying"
				}
			}
		}
		failed = retryAt == nil
	} else {
		if _, e = tx.ExecContext(ctx, `INSERT INTO practice_reminder_deliveries(user_id,reminder_id,revision,sent_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, user, p.ID, p.Revision, now); e != nil {
			return false, e
		}
		_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET email_attempts=$2,email_sent_at=$3,email_failed=FALSE,email_retry_at=NULL,email_error='' WHERE id=$1`, p.ID, attempts, now)
		if e != nil {
			return false, e
		}
		return true, tx.Commit()
	}
	_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET email_attempts=$2,email_failed=$3,email_retry_at=$4,email_error=$5 WHERE id=$1`, p.ID, attempts, failed, retryAt, reason)
	if e != nil {
		return false, e
	}
	return true, tx.Commit()
}
