package pgmemory

import (
	"database/sql"
	"embed"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*/*.sql
var migrationFiles embed.FS

func runModuleMigration(db *sql.DB, sourceDir string, migrationTable string) error {
	d, err := iofs.New(migrationFiles, "migrations/"+sourceDir)
	if err != nil {
		return err
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: migrationTable,
	})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
