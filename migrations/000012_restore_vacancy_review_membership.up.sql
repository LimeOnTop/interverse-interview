-- Restore membership of reviews created before stage reviews were quota-free.
-- Keep all interviews, reports and original quota audit rows intact.
DO $$
DECLARE affected integer;
BEGIN
  IF to_regclass('reports') IS NULL THEN RETURN; END IF;
  LOOP
    INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id,created_at)
    SELECT DISTINCT ON (rr.interview_id) a.preparation_id,a.step_index,rr.request_id,rr.interview_id,rr.created_at
    FROM review_training_requests rr
    JOIN reports r ON r.id=rr.source_report_id
    JOIN vacancy_preparation_attempts a ON a.interview_id=r.interview_id
    JOIN vacancy_preparations p ON p.id=a.preparation_id AND p.user_id=rr.user_id
    JOIN interviews child ON child.id=rr.interview_id AND child.user_id=p.user_id
    WHERE NOT EXISTS(SELECT 1 FROM vacancy_preparation_attempts existing WHERE existing.interview_id=rr.interview_id)
    ORDER BY rr.interview_id,rr.created_at
    ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS affected=ROW_COUNT;
    EXIT WHEN affected=0;
  END LOOP;
END $$;
