DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'interviews'
          AND column_name = 'interviewer_id'
          AND udt_name = 'uuid'
    ) THEN
        ALTER TABLE interviews
            ALTER COLUMN interviewer_id TYPE BIGINT USING 0;
    END IF;
END $$;
