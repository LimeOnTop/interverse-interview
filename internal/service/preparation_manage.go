package service

import (
	"context"
	"database/sql"
	"encoding/json"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
)

func ensurePreparationStepIDs(p *Preparation) {
	ns, e := uuid.Parse(p.ID)
	if e != nil {
		ns = uuid.NameSpaceOID
	}
	for i := range p.Steps {
		if p.Steps[i].ID == "" {
			p.Steps[i].ID = uuid.NewSHA1(ns, []byte(strconv.Itoa(i))).String()
		}
	}
}

func (s *PreparationService) manage(ctx context.Context, r *pb.VacancyPreparationRequest) (Preparation, error) {
	var desired, old Preparation
	if _, e := uuid.Parse(r.PreparationId); e != nil {
		return old, status.Error(codes.NotFound, "Подготовка не найдена")
	}
	if json.Unmarshal([]byte(r.PayloadJson), &desired) != nil {
		return old, status.Error(codes.InvalidArgument, "Некорректные параметры")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return old, e
	}
	defer tx.Rollback()
	// Same order as a start through a reminder: reminder -> training -> preparation.
	for _, key := range []string{"reminder:" + r.UserId, "training-quota:" + r.UserId} {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); e != nil {
			return old, e
		}
	}
	var raw []byte
	if e = tx.QueryRowContext(ctx, `SELECT payload FROM vacancy_preparations WHERE id=$1 AND user_id=$2 FOR UPDATE`, r.PreparationId, r.UserId).Scan(&raw); e == sql.ErrNoRows {
		return old, status.Error(codes.NotFound, "Подготовка не найдена")
	} else if e != nil {
		return old, e
	}
	if e = json.Unmarshal(raw, &old); e != nil {
		return old, e
	}
	ensurePreparationStepIDs(&old)
	if desired.Revision != old.Revision {
		return old, status.Error(codes.Aborted, "Подготовка изменилась. Обновите страницу")
	}
	next := old
	if r.Action == "archive" {
		next.Archived = desired.Archived
		if next.Archived {
			_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET state='dismissed',revision=revision+1,updated_at=NOW() WHERE user_id=$1 AND state='scheduled' AND ((kind='preparation' AND target_id=$2) OR (kind='review' AND payload->>'source_interview_id' IN (SELECT interview_id::text FROM vacancy_preparation_attempts WHERE preparation_id=$2::uuid)))`, r.UserId, old.ID)
			if e != nil {
				return old, e
			}
		}
	} else {
		desired.ID = old.ID
		desired.VacancyKey = old.VacancyKey
		if e = validatePreparation(&desired); e != nil {
			return old, e
		}
		oldIndex := map[string]int{}
		for i, st := range old.Steps {
			oldIndex[st.ID] = i
		}
		attempted := map[int]bool{}
		rows, e := tx.QueryContext(ctx, `SELECT DISTINCT step_index FROM vacancy_preparation_attempts WHERE preparation_id=$1`, old.ID)
		if e != nil {
			return old, e
		}
		for rows.Next() {
			var i int
			if e = rows.Scan(&i); e != nil {
				rows.Close()
				return old, e
			}
			attempted[i] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return old, e
		}
		if len(attempted) > 0 && desired.Level != old.Level {
			return old, status.Error(codes.FailedPrecondition, "Уровень начатой подготовки нельзя менять: это изменит смысл сохранённых результатов")
		}
		remap := map[int]int{}
		seen := map[string]bool{}
		for i := range desired.Steps {
			st := &desired.Steps[i]
			if st.ID == "" {
				st.ID = uuid.NewString()
			}
			if _, e = uuid.Parse(st.ID); e != nil || seen[st.ID] {
				return old, status.Error(codes.InvalidArgument, "Некорректный идентификатор этапа")
			}
			seen[st.ID] = true
			oldI, exists := oldIndex[st.ID]
			unchanged := exists && old.Steps[oldI].Technology == st.Technology && old.Steps[oldI].Specialization == st.Specialization && desired.Level == old.Level
			if exists {
				remap[oldI] = i
				if attempted[oldI] && !unchanged {
					return old, status.Error(codes.FailedPrecondition, "Технологию начатого этапа нельзя заменять. Добавьте новый этап — результаты сохранятся")
				}
			}
			if !unchanged {
				availability, e := s.interview.practiceAvailability(ctx, PracticePayload{Technologies: []string{st.Technology}, Specialization: st.Specialization, Level: desired.Level, TaskMode: "theory"}, false)
				if e != nil {
					return old, e
				}
				if !availability.Available {
					return old, status.Error(codes.FailedPrecondition, "Для "+st.Technology+" недостаточно вопросов. Выберите другую технологию или уровень")
				}
			}
		}
		for i := range attempted {
			if _, ok := remap[i]; !ok {
				return old, status.Error(codes.FailedPrecondition, "Начатый этап нельзя удалять вместе с результатами. Можно переместить его или архивировать подготовку")
			}
		}
		// Each attempt retains its technology membership when stages are reordered.
		cases := "CASE step_index"
		args := []any{old.ID}
		for from, to := range remap {
			args = append(args, from, to)
			cases += " WHEN $" + strconv.Itoa(len(args)-1) + " THEN $" + strconv.Itoa(len(args))
		}
		cases += " ELSE step_index END"
		if len(remap) > 0 {
			if _, e = tx.ExecContext(ctx, `UPDATE vacancy_preparation_attempts SET step_index=`+cases+` WHERE preparation_id=$1`, args...); e != nil {
				return old, e
			}
		}
		type reminderRow struct {
			id    string
			index int
		}
		reminders := []reminderRow{}
		rows, e = tx.QueryContext(ctx, `SELECT id::text,step_index FROM practice_reminders WHERE user_id=$1 AND kind='preparation' AND target_id=$2 FOR UPDATE`, r.UserId, old.ID)
		if e != nil {
			return old, e
		}
		for rows.Next() {
			var rr reminderRow
			if e = rows.Scan(&rr.id, &rr.index); e != nil {
				rows.Close()
				return old, e
			}
			reminders = append(reminders, rr)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return old, e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET target_key='reindex:'||id::text,revision=revision+1,updated_at=NOW() WHERE user_id=$1 AND kind='preparation' AND target_id=$2`, r.UserId, old.ID); e != nil {
			return old, e
		}
		for _, rr := range reminders {
			if to, ok := remap[rr.index]; ok {
				_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET step_index=$2,target_key=$3,title=$4 WHERE id=$1`, rr.id, to, "preparation:"+old.ID+":"+strconv.Itoa(to), desired.Steps[to].Technology+" · "+desired.Title)
			} else {
				_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET state='dismissed' WHERE id=$1`, rr.id)
			}
			if e != nil {
				return old, e
			}
		}
		next.Title = desired.Title
		next.Company = desired.Company
		next.URL = desired.URL
		next.Level = desired.Level
		next.Steps = desired.Steps
	}
	next.Revision++
	raw, e = json.Marshal(next)
	if e != nil {
		return old, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE vacancy_preparations SET payload=$2 WHERE id=$1`, old.ID, string(raw)); e != nil {
		return old, e
	}
	return next, tx.Commit()
}
