package migration

import (
	"database/sql"
	"io/ioutil"
	"log"
)

func RunMigrations(db *sql.DB) error {
	content, err := ioutil.ReadFile("internal/repository/migration/init.sql")
	if err != nil {
		log.Printf("Warning: Could not read migration file: %v", err)
		return nil
	}

	if _, err := db.Exec(string(content)); err != nil {
		return err
	}

	log.Println("Migrations completed")
	return nil
}