package parser

import "testing"

func TestTradeMarketByBroker(t *testing.T) {
	for _, tc := range []struct {
		name, broker, text string
		want               TradeMarket
	}{
		{"Stockbit TC", "Stockbit Sekuritas", "Trade Confirmation\nYour RDN Details:\nTotal Value 10000", TradeMarketIndonesia},
		{"Stockbit IPO", "Stockbit Sekuritas", "Allotment Date: 23 July 2026\nTotal Cost IDR 10000", TradeMarketIndonesia},
		{"Mandiri", "Mandiri Sekuritas", "PT Mandiri Sekuritas\nTrade Confirmation\nTotal Charges IDR 250", TradeMarketIndonesia},
		{"Trimegah", "Trimegah Sekuritas", "Trimegah Sekuritas\nTotal Sales Rp 20000", TradeMarketIndonesia},
		{"Ajaib IDX", "Ajaib Sekuritas", "Ajaib Sekuritas\nBukti perdagangan berkala\nJumlah | Harga | Bruto jual | Ticker\n100 | 100 | 10000 | TEST\nTotal Terima: Rp 10000", TradeMarketIndonesia},
		{"Ajaib receipt without table", "Ajaib Sekuritas", "Ajaib Sekuritas\nEquity Trade Confirmation\nTotal Terima: Rp 10000", TradeMarketUnknown},
		{"Ajaib US settled in rupiah", "Ajaib Sekuritas", "Ajaib Futures\nSaham AS\nTotal Bayar Rp 20000", TradeMarketExcluded},
		{"foreign currency beats domestic keyword", "Stockbit Sekuritas", "Trade Confirmation\nYour RDN Details\nTotal 100 USD", TradeMarketExcluded},
		{"Trimegah report", "Trimegah Sekuritas", "Trimegah Sekuritas\nStatement of Account\nRp 20000", TradeMarketExcluded},
		{"unknown market", "Mandiri Sekuritas", "Trade Confirmation\nABCD 10000", TradeMarketUnknown},
		{"broker not enabled for equity", "Unknown Broker", "Trade Confirmation\nIDX Rp 10000", TradeMarketUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyTradeMarket(tc.broker, tc.text); got != tc.want {
				t.Fatalf("market=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestDomesticCashNotice(t *testing.T) {
	for _, tc := range []struct {
		subject, body string
		want          bool
	}{
		{"Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100", true},
		{"Penarikan Dana Berhasil", "Jumlah : IDR 50", true},
		{"Pembagian dividen tunai BBRI", "Dividen Bersih 25 dibayar ke RDN", true},
		{"Pembagian dividen tunai BBRI", "Dividen Bersih 25", false},
		{"Pembagian dividen tunai AAPL", "Dividen Bersih 25 USD", false},
		{"Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100. Produk: kripto", false},
		{"Penarikan Dana Berhasil", "Jumlah : IDR 50. Penarikan dari akun saham AS", false},
	} {
		if got := domesticCashNotice(tc.subject, tc.body); got != tc.want {
			t.Errorf("%q %q: got %t, want %t", tc.subject, tc.body, got, tc.want)
		}
	}
}
