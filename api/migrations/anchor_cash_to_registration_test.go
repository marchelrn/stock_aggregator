package migrations

import (
	"math"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAnchorCashToRegistrationPreservesBalances(t *testing.T) {
	orm, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := orm.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, created_at TIMESTAMP);
		CREATE TABLE brokers (id INTEGER PRIMARY KEY, user_id INTEGER, cash REAL, cash_base REAL, cash_base_as_of TIMESTAMP);
		CREATE TABLE transactions (user_id INTEGER, broker_id INTEGER, type TEXT, lot INTEGER, price REAL, tax REAL, date TIMESTAMP);
		CREATE TABLE cash_mutations (user_id INTEGER, broker_id INTEGER, type TEXT, amount REAL, date TIMESTAMP, email_received_at TIMESTAMP);
		INSERT INTO users VALUES (1, '2026-01-10 12:00:00');
		INSERT INTO brokers VALUES
			(1,1,4321,0,NULL), (2,1,9000,100,'2026-01-01 00:00:00'),
			(3,1,7000,100,'2026-01-20 00:00:00'), (4,99,123,0,NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, brokerID := range []int{1, 2, 3} {
		_, err := db.Exec(`INSERT INTO transactions VALUES
			(1,?,'BUY',1,10,1,'2026-01-09 00:00:00'),
			(1,?,'BUY',1,10,1,'2026-01-11 00:00:00'),
			(1,?,'SELL',1,20,0,'2026-01-12 00:00:00'),
			(1,?,'BUY',1,10,NULL,'2026-01-13 00:00:00'),
			(999,?,'BUY',1,1000,1,'2026-01-21 00:00:00')`, brokerID, brokerID, brokerID, brokerID, brokerID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO cash_mutations VALUES
			(1,?,'DEPOSIT',9999,'2026-01-09 00:00:00','2026-01-21 00:00:00'),
			(1,?,'DEPOSIT',500,'2026-01-11 00:00:00','2026-01-09 00:00:00'),
			(1,?,'WITHDRAW',100,'2026-01-12 00:00:00',NULL),
			(1,?,'DIVIDEND',50,'2026-01-13 00:00:00',NULL),
			(1,?,'DEPOSIT',10,'2026-01-21 00:00:00',NULL)`, brokerID, brokerID, brokerID, brokerID, brokerID)
		if err != nil {
			t.Fatal(err)
		}
	}
	m := getAnchorCashToRegistration()
	for run := 0; run < 2; run++ {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Up(tx); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			id         int
			cash, base float64
			cutoff     string
		}{
			{1, 4321, 3868.5, "2026-01-10 12:00:00"},
			{2, 9000, 8547.5, "2026-01-10 12:00:00"},
			{3, 7000, 6990, "2026-01-20 00:00:00"},
		} {
			var cash, base float64
			var cutoff string
			if err := db.QueryRow("SELECT cash, cash_base, CAST(cash_base_as_of AS TEXT) FROM brokers WHERE id = ?", tc.id).Scan(&cash, &base, &cutoff); err != nil {
				t.Fatal(err)
			}
			if cash != tc.cash || math.Abs(base-tc.base) > 0.00001 || cutoff != tc.cutoff {
				t.Fatalf("run %d broker %d: cash=%v base=%v cutoff=%s", run, tc.id, cash, base, cutoff)
			}
		}
	}
	var cash float64
	if err := db.QueryRow("SELECT cash FROM brokers WHERE id = 4").Scan(&cash); err != nil || cash != 123 {
		t.Fatalf("orphan cash changed: %v err=%v", cash, err)
	}
}

func TestAnchorCashToRegistrationRegisteredForwardOnly(t *testing.T) {
	all := getMigrations()
	checkDuplicateMigrationNames(all)
	if all[len(all)-1].Name() != "anchor_cash_to_registration" || all[len(all)-1].SkipProd() {
		t.Fatal("cash cutover must follow existing migrations and run in production")
	}
	if err := getAnchorCashToRegistration().Down(nil); err != nil {
		t.Fatal(err)
	}
}
