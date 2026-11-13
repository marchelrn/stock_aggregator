package parser

import (
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
)

func TestParseCashEmail(t *testing.T) {
	now := time.Now()

	t.Run("Trimegah Deposit", func(t *testing.T) {
		from := "Trim Operation <trim.operation@trimegah.com>"
		subject := "Top Up RDN Kamu Berhasil!"
		body := `<p>Halo ANY USER,</p><p>Top up RDN kamu sebesar Rp 649,141.00 berhasil dilakukan.</p>`

		mutation, err := ParseCashEmail(from, subject, now, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected mutation, got nil")
		}
		if mutation.BrokerName != "Trimegah Sekuritas" {
			t.Errorf("expected broker Trimegah Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.Type != models.CashMutationDeposit {
			t.Errorf("expected DEPOSIT, got %s", mutation.Type)
		}
		if mutation.Amount != 649141.00 {
			t.Errorf("expected 649141.00, got %f", mutation.Amount)
		}
	})

	t.Run("Trimegah Withdrawal", func(t *testing.T) {
		from := "Trim Operation <trim.operation@trimegah.com>"
		subject := "Tarik Dana Kamu di Trima+ Berhasil!"
		body := `<p>Halo SOMEONE,</p><p>Penarikan dana kamu sebesar Rp 1,000,000.00 telah berhasil diproses.</p>`

		mutation, err := ParseCashEmail(from, subject, now, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected mutation, got nil")
		}
		if mutation.BrokerName != "Trimegah Sekuritas" {
			t.Errorf("expected broker Trimegah Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.Type != models.CashMutationWithdraw {
			t.Errorf("expected WITHDRAW, got %s", mutation.Type)
		}
		if mutation.Amount != 1000000.00 {
			t.Errorf("expected 1000000.00, got %f", mutation.Amount)
		}
	})

	t.Run("Stockbit Deposit", func(t *testing.T) {
		from := "Stockbit <no-reply@stockbit.com>"
		subject := "Deposit Kamu Sudah Masuk"
		body := `<div>deposit yang kamu lakukan sebesar <strong>Rp 353,500</strong> sudah masuk ke RDN</div>`

		mutation, err := ParseCashEmail(from, subject, now, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected mutation, got nil")
		}
		if mutation.BrokerName != "Stockbit Sekuritas" {
			t.Errorf("expected broker Stockbit Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.Type != models.CashMutationDeposit {
			t.Errorf("expected DEPOSIT, got %s", mutation.Type)
		}
		if mutation.Amount != 353500.00 {
			t.Errorf("expected 353500.00, got %f", mutation.Amount)
		}
	})

	t.Run("Stockbit Withdrawal", func(t *testing.T) {
		from := "Stockbit <no-reply@stockbit.com>"
		subject := "Penarikan Dana Berhasil"
		body := `
		<table>
			<tr><td>Jumlah</td><td>:</td><td>IDR 889,654.00</td></tr>
			<tr><td>Nomor Referensi</td><td>:</td><td>43877003</td></tr>
		</table>
		`

		mutation, err := ParseCashEmail(from, subject, now, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected mutation, got nil")
		}
		if mutation.BrokerName != "Stockbit Sekuritas" {
			t.Errorf("expected broker Stockbit Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.Type != models.CashMutationWithdraw {
			t.Errorf("expected WITHDRAW, got %s", mutation.Type)
		}
		if mutation.Amount != 889654.00 {
			t.Errorf("expected 889654.00, got %f", mutation.Amount)
		}
		if mutation.RefNo != "43877003" {
			t.Errorf("expected 43877003, got %s", mutation.RefNo)
		}
	})

	t.Run("Ignore pending withdraw", func(t *testing.T) {
		from := "Trim Operation <trim.operation@trimegah.com>"
		subject := "Tarik Dana Kamu Sedang Diproses"
		body := `<p>Sedang diproses</p>`

		mutation, err := ParseCashEmail(from, subject, now, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation != nil {
			t.Errorf("expected nil for pending withdraw, got %+v", mutation)
		}
	})
}
