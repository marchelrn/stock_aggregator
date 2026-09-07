package migrations

import (
	"database/sql"

	"github.com/marchelrn/stock_api/config"
)

type migration interface {
	Name() string
	Up(conn *sql.Tx) error
	Down(conn *sql.Tx) error
	SkipProd() bool
}

func getMigrations() []migration {
	return []migration{
		getCreateStocksTable(),
		getCreateTableBroker(),
		getCreateTableTransaction(),
		getCreateStockPricesTable(),
		getAlterStocksUniqueTickerBroker(),
		getCreateTableUsers(),
		getAlterUsersAddGmailTokens(),
	}
}

func checkDuplicateMigrationNames(migrations []migration) {
	nameSet := make(map[string]bool)
	for _, m := range migrations {
		if nameSet[m.Name()] {
			panic("Duplicate Migrations Name " + m.Name())
		}
		nameSet[m.Name()] = true
	}
}

func Up(db *sql.DB) {
	migrations := getMigrations()

	cfg := config.GetConfig()
	checkDuplicateMigrationNames(migrations)

	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS migrations (
		name VARCHAR(255) PRIMARY KEY,	
		applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		panic(err)
	}

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		var count int
		err := tx.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = $1", m.Name()).Scan(&count)
		if err != nil {
			panic(err)
		}

		if count == 0 {
			println("Executing migration:", m.Name())
			if cfg.IsProd && m.SkipProd() {
				continue
			}
			if err := m.Up(tx); err != nil {
				panic(err)
			}
			_, err := tx.Exec("INSERT INTO migrations (name) VALUES ($1)", m.Name())
			if err != nil {
				panic(err)
			}

			println("Applied migration:", m.Name())
		} else {
			println("Skipping migration (already applied):", m.Name())
		}
	}

	if err := tx.Commit(); err != nil {
		panic(err)
	}
}

func Down(db *sql.DB) {
	migrations := getMigrations()
	checkDuplicateMigrationNames(migrations)

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	var lastMigration string
	err = tx.QueryRow("SELECT name FROM migrations ORDER BY applied_at DESC LIMIT 1").Scan(&lastMigration)
	if err != nil {
		if err == sql.ErrNoRows {
			println("No Migrations to revert")
			return
		}
		panic(err)
	}

	var migrationToRevert migration
	for i := len(migrations) - 1; i >= 0; i-- {
		if migrations[i].Name() == lastMigration {
			migrationToRevert = migrations[i]
			break
		}
	}

	if migrationToRevert == nil {
		panic("Last applied migrations not found in migration list")
	}

	if err := migrationToRevert.Down(tx); err != nil {
		panic(err)
	}

	_, err = tx.Exec("DELETE FROM migrations WHERE name = $1", lastMigration)
	if err != nil {
		panic(err)
	}

	println("Reverted migrations:", lastMigration)

	if err := tx.Commit(); err != nil {
		panic(err)
	}
}

func DownAll(db *sql.DB) {
	migrations := getMigrations()
	checkDuplicateMigrationNames(migrations)

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		var count int
		err := tx.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = $1", m.Name()).Scan(&count)
		if err != nil {
			panic(err)
		}

		if count > 0 {

			if err := m.Down(tx); err != nil {
				panic(err)
			}

			_, err = tx.Exec("DELETE FROM migrations WHERE name = $1", m.Name())
			if err != nil {
				panic(err)
			}

			println("Reverted migration:", m.Name())
		}
	}

	if err := tx.Commit(); err != nil {
		panic(err)
	}
}
