package repository

import (
	"math"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
)

func floatEquals(a, b float64, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestCalculateHoldingAfterBatch_UnrecordedHolding_BuyAndSell(t *testing.T) {
	// Skenario dari gambar: User punya posisi lama (unrecorded di DB, lot=0)
	// Hari ini: Buy 249 lot PART @ 98 (tax 3660.30), Sell 253 lot PART @ 97 (tax 6135.25)
	trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeBuy,
			Lot:        249,
			Price:      98.0,
			Tax:        3660.30,
			AmountDone: 2443860.30,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
			RefNo:      "0033568",
		},
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        253,
			Price:      97.0,
			Tax:        6135.25,
			AmountDone: 2460235.25,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
			RefNo:      "0167201",
		},
	}

	currentLot := 0
	currentAvgPrice := 0.0

	newLot, newAvg, cashDelta := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	// User should retain the 249 lot from the new buy
	if newLot != 249 {
		t.Fatalf("expected newLot 249, got %d", newLot)
	}

	// Average price should reflect the 98 buy with fees: 2,443,860.30 / 24,900 = 98.147
	expectedAvg := 2443860.30 / (249 * 100)
	if !floatEquals(newAvg, expectedAvg, 0.01) {
		t.Fatalf("expected newAvg %.2f, got %.2f", expectedAvg, newAvg)
	}

	// Cash delta should match Stockbit "Payment due to you IDR 4,104.45"
	// Proceeds = 2,454,100 - 6,135.25 = 2,447,964.75
	// Cost = 2,440,200 + 3,660.30 = 2,443,860.30
	// Delta = 2,447,964.75 - 2,443,860.30 = 4,104.45
	if !floatEquals(cashDelta, 4104.45, 0.01) {
		t.Fatalf("expected cashDelta 4104.45, got %.2f", cashDelta)
	}
}

func TestCalculateHoldingAfterBatch_UserSellsAgainNextDay(t *testing.T) {
	// Konteks: User sebelumnya memegang 249 lot @ 98.147, lalu di hari berikutnya menjual habis 249 lot
	trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        249,
			Price:      99.0,
			Tax:        616.27,
			AmountDone: 2465100.0,
			Date:       time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC),
		},
	}

	currentLot := 249
	currentAvgPrice := 98.147

	newLot, newAvg, _ := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	// Posisi ditutup habis: 0 lot, 0 avg
	if newLot != 0 {
		t.Fatalf("expected newLot 0, got %d", newLot)
	}
	if newAvg != 0 {
		t.Fatalf("expected newAvg 0, got %.2f", newAvg)
	}
}

func TestCalculateHoldingAfterBatch_UserSellsPartialNextDay(t *testing.T) {
	// Konteks: User sebelumnya memegang 249 lot @ 98.147, lalu di hari berikutnya menjual 100 lot
	trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        100,
			Price:      100.0,
			Tax:        250.0,
			AmountDone: 1000000.0,
			Date:       time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC),
		},
	}

	currentLot := 249
	currentAvgPrice := 98.147

	newLot, newAvg, _ := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	// Sisa lot: 249 - 100 = 149 lot
	if newLot != 149 {
		t.Fatalf("expected newLot 149, got %d", newLot)
	}
	// Avg price tetap sama pada penjualan parsial
	if !floatEquals(newAvg, 98.147, 0.01) {
		t.Fatalf("expected newAvg 98.15, got %.2f", newAvg)
	}
}

func TestCalculateHoldingAfterBatch_UserHadExactPriorPosition(t *testing.T) {
	// Konteks: User memang tercatat punya 253 lot @ 90.0 di DB
	// Menjual 253 @ 97 dan membeli lagi 249 @ 98
	trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeBuy,
			Lot:        249,
			Price:      98.0,
			Tax:        3660.30,
			AmountDone: 2443860.30,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		},
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        253,
			Price:      97.0,
			Tax:        6135.25,
			AmountDone: 2460235.25,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		},
	}

	currentLot := 253
	currentAvgPrice := 90.0

	newLot, newAvg, cashDelta := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	// Posisi lama 253 habis dijual, posisi baru 249 terbentuk
	if newLot != 249 {
		t.Fatalf("expected newLot 249, got %d", newLot)
	}
	expectedAvg := 2443860.30 / (249 * 100)
	if !floatEquals(newAvg, expectedAvg, 0.01) {
		t.Fatalf("expected newAvg %.2f, got %.2f", expectedAvg, newAvg)
	}
	if !floatEquals(cashDelta, 4104.45, 0.01) {
		t.Fatalf("expected cashDelta 4104.45, got %.2f", cashDelta)
	}
}

func TestCalculateHoldingAfterBatch_UserHadLargerPriorPosition(t *testing.T) {
	// Konteks: User tercatat punya 500 lot @ 90.0 di DB
	// Menjual 253 @ 97 dan membeli 249 @ 98
	trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeBuy,
			Lot:        249,
			Price:      98.0,
			Tax:        3660.30,
			AmountDone: 2443860.30,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		},
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        253,
			Price:      97.0,
			Tax:        6135.25,
			AmountDone: 2460235.25,
			Date:       time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		},
	}

	currentLot := 500
	currentAvgPrice := 90.0

	newLot, newAvg, _ := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	// Sisa lot: (500 - 253) + 249 = 247 + 249 = 496 lot
	if newLot != 496 {
		t.Fatalf("expected newLot 496, got %d", newLot)
	}

	// Blended moving average: (247*100*90 + 2,443,860.30) / (496*100) = 4,666,860.30 / 49,600 = 94.09
	expectedAvg := (247.0*100.0*90.0 + 2443860.30) / (496.0 * 100.0)
	if !floatEquals(newAvg, expectedAvg, 0.01) {
		t.Fatalf("expected newAvg %.2f, got %.2f", expectedAvg, newAvg)
	}
}

func TestCalculateHoldingAfterBatch_IntradayScalpFlat(t *testing.T) {
	// Konteks: User beli 100 lot @ 100 dan jual 100 lot @ 105 (intraday flat scalp)
	trades := []*models.Transaction{
		{
			Ticker: "BBCA",
			Type:   models.TypeBuy,
			Lot:    100,
			Price:  100.0,
		},
		{
			Ticker: "BBCA",
			Type:   models.TypeSell,
			Lot:    100,
			Price:  105.0,
		},
	}

	currentLot := 0
	currentAvgPrice := 0.0

	newLot, newAvg, cashDelta := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, trades)

	if newLot != 0 {
		t.Fatalf("expected newLot 0, got %d", newLot)
	}
	if newAvg != 0 {
		t.Fatalf("expected newAvg 0, got %.2f", newAvg)
	}
	// Profit dari kenaikan harga (105 - 100) * 10,000 = 50,000 minus fees
	if cashDelta <= 0 {
		t.Fatalf("expected positive cashDelta, got %.2f", cashDelta)
	}
}

func TestCalculateHoldingAfterBatch_MultiDayFullCycle(t *testing.T) {
	// Skenario nyata dari database user:
	// Day 1 (05/11): User beli 253 lot PART @ 92
	// Day 2 (06/11): User jual 253 lot PART @ 97, beli 249 lot PART @ 98
	day1Trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeBuy,
			Lot:        253,
			Price:      92.0,
			Tax:        3491.40,
			AmountDone: 2331091.40,
			Date:       time.Date(2026, 11, 5, 10, 31, 12, 0, time.UTC),
		},
	}

	lotDay1, avgDay1, _ := CalculateHoldingAfterBatch(0, 0, day1Trades)
	if lotDay1 != 253 {
		t.Fatalf("expected day 1 lot 253, got %d", lotDay1)
	}

	day2Trades := []*models.Transaction{
		{
			Ticker:     "PART",
			Type:       models.TypeBuy,
			Lot:        249,
			Price:      98.0,
			Tax:        3660.30,
			AmountDone: 2443860.30,
			Date:       time.Date(2026, 11, 6, 10, 22, 15, 0, time.UTC),
			RefNo:      "0033568",
		},
		{
			Ticker:     "PART",
			Type:       models.TypeSell,
			Lot:        253,
			Price:      97.0,
			Tax:        6135.25,
			AmountDone: 2460235.25,
			Date:       time.Date(2026, 11, 6, 10, 22, 15, 0, time.UTC),
			RefNo:      "0167201",
		},
	}

	lotDay2, avgDay2, _ := CalculateHoldingAfterBatch(lotDay1, avgDay1, day2Trades)

	// Sisa lot HARUS 249 (bukan 502!)
	if lotDay2 != 249 {
		t.Fatalf("expected final lot 249, got %d", lotDay2)
	}

	// Avg price HARUS 98.15 (murni harga beli baru 98 + fee)
	expectedAvg := 2443860.30 / (249 * 100)
	if !floatEquals(avgDay2, expectedAvg, 0.01) {
		t.Fatalf("expected final avg %.2f, got %.2f", expectedAvg, avgDay2)
	}
}
