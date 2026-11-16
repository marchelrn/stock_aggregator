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

func TestParseDividendEmail(t *testing.T) {
	emailDate := time.Date(2026, 10, 22, 9, 15, 0, 0, time.Local)

	t.Run("Stockbit Dividend Plain Text", func(t *testing.T) {
		from := "Stockbit <no-reply@stockbit.com>"
		subject := "Pembagian dividen tunai CDIA"
		body := `Cash Dividend
Hi
MARCHEL MATTHEW HOTASI MANULLANG,
Kamu mendapatkan
dividen tunai
dengan detail sebagai berikut:
Tanggal Pembayaran
2026-10-22
Tipe
Dividen Tunai
Jumlah Kepemilikan Saham
3,900
Dividen per Saham (Rp)
1.42
Nilai Dividen (Rp)
5,546.51
Pajak Dividen (%)
0%
Pajak Dividen (Rp)
0
Dividen Bersih
5,546.51`

		mutation, err := ParseCashEmail(from, subject, emailDate, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected dividend mutation, got nil")
		}
		if mutation.Type != models.CashMutationDividend {
			t.Errorf("expected DIVIDEND, got %s", mutation.Type)
		}
		if mutation.Amount != 5546.51 {
			t.Errorf("expected 5546.51, got %f", mutation.Amount)
		}
		if got := mutation.Date.Format("2006-01-02"); got != "2026-10-22" {
			t.Errorf("expected pay date 2026-10-22, got %s", got)
		}
		if mutation.BrokerName != "Stockbit Sekuritas" {
			t.Errorf("expected Stockbit Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.RefNo != "DIV-CDIA-20261022" {
			t.Errorf("expected ref DIV-CDIA-20261022, got %s", mutation.RefNo)
		}
		if mutation.Description != "Dividen Tunai CDIA" {
			t.Errorf("expected description Dividen Tunai CDIA, got %s", mutation.Description)
		}
	})

	t.Run("Stockbit Dividend HTML", func(t *testing.T) {
		from := "Stockbit <no-reply@stockbit.com>"
		subject := "Pembagian dividen tunai ACES"
		body := `<div>Tanggal Pembayaran</div><div>2026-07-10</div>
<table><tr><td>Nilai Dividen (Rp)</td><td>233,673</td></tr>
<tr><td style="font-size:12px">Dividen Bersih</td><td>233,673</td></tr></table>`

		mutation, err := ParseCashEmail(from, subject, emailDate, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected dividend mutation, got nil")
		}
		if mutation.Amount != 233673 {
			t.Errorf("expected 233673, got %f", mutation.Amount)
		}
		if got := mutation.Date.Format("2006-01-02"); got != "2026-07-10" {
			t.Errorf("expected pay date 2026-07-10, got %s", got)
		}
		if mutation.RefNo != "DIV-ACES-20260710" {
			t.Errorf("expected ref DIV-ACES-20260710, got %s", mutation.RefNo)
		}
	})

	t.Run("Mandiri Dividend", func(t *testing.T) {
		from := "cc_admin <cc_admin@mandirisekuritas.co.id>"
		subject := "Pembayaran Dividen Tunai' BANK RAKYAT INDONESIA (PERSERO) TBK (BBRI)'"
		body := `Jakarta, 08 May 2026

Sesuai dengan jadwal "Dividen Tunai" atas efek BANK RAKYAT INDONESIA
(PERSERO) TBK (BBRI), dividen tunai telah di distribusikan ke RDN/MTBI
dengan perincian, sebagai berikut :

Dividen Tunai
Jumlah Kepemilikan Saham (lembar)	300
Dividen per Saham (Rp)	209
Perhitungan Dividen
Nilai Dividen (Rp)	62,700
Pajak Dividen (Rp) 	0
Dividen Bersih (Rp)	62,700

Dividen dibayarkan ke RDN/ MTBI pada tanggal 	8 Mei 2026`

		mutation, err := ParseCashEmail(from, subject, emailDate, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation == nil {
			t.Fatalf("expected dividend mutation, got nil")
		}
		if mutation.Type != models.CashMutationDividend {
			t.Errorf("expected DIVIDEND, got %s", mutation.Type)
		}
		if mutation.Amount != 62700 {
			t.Errorf("expected 62700, got %f", mutation.Amount)
		}
		if got := mutation.Date.Format("2006-01-02"); got != "2026-05-08" {
			t.Errorf("expected pay date 2026-05-08, got %s", got)
		}
		if mutation.BrokerName != "Mandiri Sekuritas" {
			t.Errorf("expected Mandiri Sekuritas, got %s", mutation.BrokerName)
		}
		if mutation.RefNo != "DIV-BBRI-20260508" {
			t.Errorf("expected ref DIV-BBRI-20260508, got %s", mutation.RefNo)
		}
	})

	t.Run("Stockbit Newsletter With Dividend Word Is Ignored", func(t *testing.T) {
		from := "Stockbit Snips <no-reply@stockbit.com>"
		subject := "PTBA dan ANTM Tebar Dividen Jumbo, Musim Dividen Berlanjut"
		body := `Dividen Bersih 1.000.000 saham akan dibagikan minggu ini.`

		mutation, err := ParseCashEmail(from, subject, emailDate, body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mutation != nil {
			t.Errorf("expected nil for newsletter, got %+v", mutation)
		}
	})
}
