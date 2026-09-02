-- Backfill MCQ options for existing session items from question_options
UPDATE interview_session_items isi
SET options = sub.options_json
FROM (
    SELECT
        isi2.id,
        COALESCE(
            jsonb_agg(
                jsonb_build_object(
                    'text', qo.text,
                    'is_correct', qo.is_correct,
                    'sort_order', qo.sort_order
                )
                ORDER BY qo.sort_order
            ),
            '[]'::jsonb
        ) AS options_json
    FROM interview_session_items isi2
    JOIN question_options qo ON qo.question_id = isi2.question_id
    WHERE isi2.item_type = 'question'
      AND isi2.question_id IS NOT NULL
      AND (isi2.options IS NULL OR isi2.options = '[]'::jsonb)
    GROUP BY isi2.id
) sub
WHERE isi.id = sub.id;
