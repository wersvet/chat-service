package db

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Connect initializes the database connection and runs migrations.
func Connect() (*sqlx.DB, error) {
	dsn := getEnv("DB_DSN", "postgres://chat_user:password@localhost:5432/chat_service?sslmode=disable")
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sqlx.DB) error {
	migrations := []string{
		`DROP TABLE IF EXISTS group_members;`,
		`DROP TABLE IF EXISTS groups;`,
		`DROP TABLE IF EXISTS chat_members;`,
		`DROP TABLE IF EXISTS messages;`,
		`CREATE TABLE IF NOT EXISTS chats (
            id SERIAL PRIMARY KEY,
            user1_id INT NOT NULL,
            user2_id INT NOT NULL,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE(user1_id, user2_id)
        );`,
		`CREATE TABLE IF NOT EXISTS messages (
            id SERIAL PRIMARY KEY,
            chat_id INT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            sender_id INT NOT NULL,
            content TEXT NOT NULL,
            deleted_by_sender BOOLEAN DEFAULT FALSE,
            deleted_by_receiver BOOLEAN DEFAULT FALSE,
            deleted_for_all BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMPTZ DEFAULT NOW()
        );`,
		`CREATE TABLE IF NOT EXISTS chat_visibility (
            chat_id INT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
            user_id INT NOT NULL,
            hidden BOOLEAN DEFAULT TRUE,
            PRIMARY KEY(chat_id, user_id)
        );`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return err
		}
	}
	log.Println("database migrations applied")
	return nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
