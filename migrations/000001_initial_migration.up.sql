CREATE TABLE IF NOT EXISTS interviews (
    id UUID PRIMARY KEY,
    candidate_id UUID NOT NULL,
    interviewer_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',
    scheduled_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    level VARCHAR(50),
    specialization VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS interview_technologies (
    interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
    technology_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (interview_id, technology_id)
);
