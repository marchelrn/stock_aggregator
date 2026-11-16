package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type legacyTrimegahMigrationConn struct {
	query string
	args  []driver.NamedValue
	err   error
	calls int
}

func (c *legacyTrimegahMigrationConn) Connect(context.Context) (driver.Conn, error) {
	return c, nil
}
func (c *legacyTrimegahMigrationConn) Driver() driver.Driver { return c }
func (c *legacyTrimegahMigrationConn) Open(string) (driver.Conn, error) {
	return c, nil
}
func (c *legacyTrimegahMigrationConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (c *legacyTrimegahMigrationConn) Close() error              { return nil }
func (c *legacyTrimegahMigrationConn) Begin() (driver.Tx, error) { return c, nil }
func (c *legacyTrimegahMigrationConn) Commit() error             { return nil }
func (c *legacyTrimegahMigrationConn) Rollback() error           { return nil }
func (c *legacyTrimegahMigrationConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.query, c.args = query, args
	c.calls++
	return driver.RowsAffected(1), c.err
}

func TestDisableLegacyTrimegahDiscoveryTemplate(t *testing.T) {
	conn := &legacyTrimegahMigrationConn{}
	db := sql.OpenDB(conn)
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	m := getDisableLegacyTrimegahDiscoveryTemplate()
	if err := m.Up(tx); err != nil {
		t.Fatal(err)
	}
	query := strings.Join(strings.Fields(conn.query), " ")
	wantQuery := "UPDATE pdf_parsing_templates SET active = FALSE, updated_at = NOW() WHERE key = $1 AND version = $2 AND active = TRUE AND config = $3::jsonb"
	if conn.calls != 1 || query != wantQuery {
		t.Fatalf("unexpected migration SQL: calls=%d query=%q", conn.calls, query)
	}
	if len(conn.args) != 3 || conn.args[0].Value != "trimegah" || conn.args[1].Value != int64(1) || conn.args[2].Value != legacyTrimegahDiscoveryConfig {
		t.Fatalf("unexpected migration parameters: %+v", conn.args)
	}
	var config struct {
		SchemaVersion   int      `json:"schema_version"`
		RequiredMarkers []string `json:"required_markers"`
		Note            string   `json:"note"`
	}
	decoder := json.NewDecoder(strings.NewReader(legacyTrimegahDiscoveryConfig))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		t.Fatal(err)
	}
	if config.SchemaVersion != 1 || len(config.RequiredMarkers) != 1 || config.RequiredMarkers[0] != "TRIMEGAH" || config.Note != "Trimegah TC parsed by hardcoded TrimegahParser; this template exists for Gmail discovery only" {
		t.Fatalf("legacy seed fingerprint changed: %+v", config)
	}

	conn.err = errors.New("update failed")
	if err := m.Up(tx); !errors.Is(err, conn.err) {
		t.Fatalf("expected original database error, got %v", err)
	}
	calls := conn.calls
	if err := m.Down(tx); err != nil || conn.calls != calls {
		t.Fatalf("rollback must not reactivate the invalid seed: calls=%d err=%v", conn.calls, err)
	}
}

func TestDisableLegacyTrimegahDiscoveryTemplateRegistered(t *testing.T) {
	all := getMigrations()
	checkDuplicateMigrationNames(all)
	seedIndex, cacheIndex, repairIndex := -1, -1, -1
	for i, m := range all {
		switch m.Name() {
		case "add_trimegah_template":
			seedIndex = i
		case "create_processed_gmail_messages_table":
			cacheIndex = i
		case "disable_legacy_trimegah_discovery_template":
			repairIndex = i
			if m.SkipProd() {
				t.Fatal("legacy seed repair must run in production")
			}
		}
	}
	if seedIndex < 0 || cacheIndex < 0 || repairIndex <= seedIndex || repairIndex <= cacheIndex {
		t.Fatalf("expected repair after existing seed/cache migrations: seed=%d cache=%d repair=%d", seedIndex, cacheIndex, repairIndex)
	}
}
