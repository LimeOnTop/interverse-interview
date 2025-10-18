package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func NewConnection(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	createInterviewsTable := `
	CREATE TABLE IF NOT EXISTS interviews (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		candidate_id UUID NOT NULL,
		interviewer_id UUID NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		status VARCHAR(50) DEFAULT 'scheduled',
		scheduled_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		technologies TEXT[],
		level VARCHAR(50),
		specialization VARCHAR(100)
	);`

	createInterviewTechnologiesTable := `
	CREATE TABLE IF NOT EXISTS interview_technologies (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
		technology_id UUID NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	createCandidatesTable := `
	CREATE TABLE IF NOT EXISTS candidates (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		phone VARCHAR(20),
		experience_years INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createInterviewsTable); err != nil {
		return fmt.Errorf("failed to create interviews table: %w", err)
	}

	if _, err := db.Exec(createInterviewTechnologiesTable); err != nil {
		return fmt.Errorf("failed to create interview_technologies table: %w", err)
	}

	if _, err := db.Exec(createCandidatesTable); err != nil {
		return fmt.Errorf("failed to create candidates table: %w", err)
	}

	// Insert test candidate data
	insertTestCandidate := `
	INSERT INTO candidates (id, name, email, phone, experience_years) 
	VALUES ('550e8400-e29b-41d4-a716-446655440001', 'Иван Петров', 'ivan.petrov@example.com', '+7 (999) 123-45-67', 3)
	ON CONFLICT (id) DO NOTHING;`

	if _, err := db.Exec(insertTestCandidate); err != nil {
		return fmt.Errorf("failed to insert test candidate: %w", err)
	}

	return nil
}
