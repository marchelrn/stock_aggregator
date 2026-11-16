package migrations

import (
	"database/sql"
	"log"
)

// AddCashBaseToBrokers introduces the manual cash-base model:
//
//	cash            = cash_base + SUM(event deltas after cash_base_as_of)
//
// cash_base        : saldo dasar yang di-set manual oleh user
// cash_base_as_of  : titik waktu saldo dasar tersebut berlaku (cutoff).
//                    NULL = mode lama (cash dihitung langsung dari replay).
// email_received_at: waktu email diterima dari broker, dipakai sebagai
//                    waktu efektif sebuah mutasi kas saat membandingkan
//                    dengan cutoff (lebih akurat daripada tanggal yang
//                    tercantum di dalam email).
type AddCashBaseToBrokers struct{}

func getAddCashBaseToBrokers() migration {
	return &AddCashBaseToBrokers{}
}

func (m *AddCashBaseToBrokers) Name() string {
	return "add_cash_base_to_brokers"
}

func (m *AddCashBaseToBrokers) Up(conn *sql.Tx) error {
	log.Println("Running migration: add_cash_base_to_brokers")
	_, err := conn.Exec(`
	ALTER TABLE brokers ADD COLUMN IF NOT EXISTS cash_base NUMERIC(14,2) NOT NULL DEFAULT 0;
	ALTER TABLE brokers ADD COLUMN IF NOT EXISTS cash_base_as_of TIMESTAMP NULL;
	ALTER TABLE cash_mutations ADD COLUMN IF NOT EXISTS email_received_at TIMESTAMP NULL;
	`)
	if err != nil {
		log.Printf("Error adding cash_base columns: %v", err)
		return err
	}
	return nil
}

func (m *AddCashBaseToBrokers) Down(conn *sql.Tx) error {
	log.Println("Rolling back migration: add_cash_base_to_brokers")
	_, err := conn.Exec(`
	ALTER TABLE brokers DROP COLUMN IF EXISTS cash_base_as_of;
	ALTER TABLE brokers DROP COLUMN IF EXISTS cash_base;
	ALTER TABLE cash_mutations DROP COLUMN IF EXISTS email_received_at;
	`)
	return err
}

func (m *AddCashBaseToBrokers) SkipProd() bool {
	return false
}
