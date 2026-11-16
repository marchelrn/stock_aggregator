package repository

import (
	"math"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func cashTestDB(t *testing.T) (*gorm.DB, models.Users) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Users{}, &models.Broker{}, &models.StockHolding{}, &models.StockPrice{}, &models.CashMutation{}); err != nil {
		t.Fatal(err)
	}
	// PostgreSQL's transaction enum is represented as TEXT in this test database.
	if err := db.Exec(`CREATE TABLE transactions (
		id INTEGER PRIMARY KEY, user_id INTEGER, ticker TEXT, price REAL, lot INTEGER,
		stock_id INTEGER, broker_id INTEGER, broker_name TEXT, type TEXT, date DATETIME,
		amount_done REAL, tax REAL, ref_no TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	user := models.Users{Email: "cash-test@example.com", CreatedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return db, user
}

func testBroker(t *testing.T, db *gorm.DB, user models.Users, cash float64) *models.Broker {
	t.Helper()
	broker, err := ImplBrokersRepository(db).AddBrokerData(&models.Broker{UserID: user.Id, Name: "Test Broker", Cash: cash})
	if err != nil {
		t.Fatal(err)
	}
	if broker.CashBase != cash || broker.CashBaseAsOf == nil || !broker.CashBaseAsOf.Equal(user.CreatedAt) {
		t.Fatalf("initial cash baseline must equal entered cash at registration: %+v", broker)
	}
	return broker
}

func assertCash(t *testing.T, db *gorm.DB, userID uint, brokerID int, want float64) {
	t.Helper()
	repo := ImplBrokersRepository(db)
	broker, err := repo.GetBrokerById(userID, brokerID)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(broker.Cash-want) > 0.00001 {
		t.Fatalf("stored cash = %v, want %v", broker.Cash, want)
	}
	breakdown, err := repo.GetCashBreakdown(userID, brokerID)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(breakdown.Cash-want) > 0.00001 || math.Abs(breakdown.CashBase+breakdown.EventsTotal-want) > 0.00001 {
		t.Fatalf("breakdown disagrees with stored cash %v: %+v", want, breakdown)
	}
	if err := RecomputeBrokerCash(db, userID, broker); err != nil {
		t.Fatal(err)
	}
	if math.Abs(broker.Cash-want) > 0.00001 {
		t.Fatalf("recomputed cash = %v, want %v", broker.Cash, want)
	}
}

func testTrade(user models.Users, broker *models.Broker, date time.Time, kind models.Type, ticker string) *models.Transaction {
	return &models.Transaction{UserID: user.Id, BrokerID: broker.ID, BrokerName: broker.Name, Ticker: ticker,
		Date: date, Type: kind, Lot: 1, Price: 10, Tax: 1}
}

func TestCashManualTradesRegistrationAndCutoff(t *testing.T) {
	for _, mode := range []string{"registration", "nil legacy", "before registration", "manual cutoff"} {
		t.Run(mode, func(t *testing.T) {
			db, user := cashTestDB(t)
			broker := testBroker(t, db, user, 0)
			cutoff := user.CreatedAt
			switch mode {
			case "nil legacy":
				if err := db.Model(broker).Updates(map[string]any{"cash_base_as_of": nil, "cash_base": 0}).Error; err != nil {
					t.Fatal(err)
				}
			case "before registration":
				old := cutoff.Add(-24 * time.Hour)
				if err := db.Model(broker).Update("cash_base_as_of", old).Error; err != nil {
					t.Fatal(err)
				}
			case "manual cutoff":
				cutoff = cutoff.Add(48 * time.Hour)
				if err := db.Model(broker).Update("cash_base_as_of", cutoff).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := ImplTransactionRepository(db)
			buy := testTrade(user, broker, cutoff.Add(-time.Hour), models.TypeBuy, "HIST")
			if _, err := repo.CreateTransactionAndSyncHolding(buy); err != nil {
				t.Fatalf("historical BUY with no cash must still create holding: %v", err)
			}
			if _, err := repo.CreateTransactionAndSyncHolding(testTrade(user, broker, cutoff, models.TypeSell, "HIST")); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 0)
			if _, err := repo.CreateTransactionAndSyncHolding(testTrade(user, broker, cutoff.Add(time.Hour), models.TypeBuy, "LIVE")); err == nil {
				t.Fatal("post-cutoff BUY must enforce insufficient cash")
			}
			if _, err := ImplBrokersRepository(db).UpdateBrokerData(user.Id, broker.ID, &models.Broker{Cash: 5000}); err != nil {
				t.Fatal(err)
			}
			live := testTrade(user, broker, cutoff.Add(time.Hour), models.TypeBuy, "LIVE")
			if _, err := repo.CreateTransactionAndSyncHolding(live); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 3999)
			if _, err := repo.CreateTransactionAndSyncHolding(testTrade(user, broker, cutoff.Add(time.Hour), models.TypeBuy, "LIVE")); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 3999)
			if _, err := repo.CreateTransactionAndSyncHolding(testTrade(user, broker, cutoff.Add(2*time.Hour), models.TypeSell, "LIVE")); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 4998)
		})
	}
}

func TestCashBatchHistoryAndIncrementalNoDuplicates(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "baseline", true: "nil cutoff"}[legacy], func(t *testing.T) {
			db, user := cashTestDB(t)
			broker := testBroker(t, db, user, 10000)
			if legacy {
				if err := db.Model(broker).Updates(map[string]any{"cash_base_as_of": nil, "cash_base": 0}).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := ImplTransactionRepository(db)
			trades := []*models.Transaction{
				testTrade(user, broker, user.CreatedAt.Add(-48*time.Hour), models.TypeBuy, "HIST"),
				testTrade(user, broker, user.CreatedAt.Add(-24*time.Hour), models.TypeSell, "HIST"),
				testTrade(user, broker, user.CreatedAt, models.TypeBuy, "EXACT"),
				testTrade(user, broker, user.CreatedAt.Add(time.Hour), models.TypeBuy, "LIVE"),
				testTrade(user, broker, user.CreatedAt.Add(24*time.Hour), models.TypeSell, "LIVE"),
			}
			for run, wantCount := range []int{5, 0} {
				count, err := repo.SyncTradeConfirmationBatch(user.Id, trades)
				if err != nil || count != wantCount {
					t.Fatalf("run %d: count=%d err=%v", run, count, err)
				}
				assertCash(t, db, user.Id, broker.ID, 9998)
			}
			var holding models.StockHolding
			if err := db.Where("ticker = ?", "EXACT").First(&holding).Error; err != nil || holding.Lot != 1 {
				t.Fatalf("historical trade must still contribute to holdings: %+v err=%v", holding, err)
			}
			breakdown, err := ImplBrokersRepository(db).GetCashBreakdown(user.Id, broker.ID)
			if err != nil || len(breakdown.Events) != 2 {
				t.Fatalf("breakdown=%+v err=%v", breakdown, err)
			}
		})
	}
}

func TestCashMutationUsesDateNotEmailReceipt(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "baseline", true: "nil cutoff"}[legacy], func(t *testing.T) {
			db, user := cashTestDB(t)
			broker := testBroker(t, db, user, 1000)
			if legacy {
				if err := db.Model(broker).Updates(map[string]any{"cash_base_as_of": nil, "cash_base": 0}).Error; err != nil {
					t.Fatal(err)
				}
			}
			before, after := user.CreatedAt.Add(-time.Hour), user.CreatedAt.Add(time.Hour)
			mutations := []*models.CashMutation{
				{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 9000, Date: before, EmailReceivedAt: &after, RefNo: "old-deposit"},
				{BrokerID: broker.ID, Type: models.CashMutationWithdraw, Amount: 8000, Date: before, EmailReceivedAt: &after, RefNo: "old-withdraw"},
				{BrokerID: broker.ID, Type: models.CashMutationDividend, Amount: 7000, Date: user.CreatedAt, RefNo: "exact"},
				{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 500, Date: after, EmailReceivedAt: &before, RefNo: "new-deposit"},
				{BrokerID: broker.ID, Type: models.CashMutationWithdraw, Amount: 100, Date: after, RefNo: "new-withdraw"},
				{BrokerID: broker.ID, Type: models.CashMutationDividend, Amount: 50, Date: after, RefNo: "new-dividend"},
			}
			repo := ImplCashMutationRepository(db)
			for run, want := range []int{3, 0} {
				count, err := repo.SyncCashMutationBatch(user.Id, mutations)
				if err != nil || count != want {
					t.Fatalf("run %d count=%d err=%v", run, count, err)
				}
				assertCash(t, db, user.Id, broker.ID, 1450)
			}
		})
	}
}

func TestCashCurrentResetIgnoresClientAsOf(t *testing.T) {
	for _, offset := range []time.Duration{-365 * 24 * time.Hour, 365 * 24 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			db, user := cashTestDB(t)
			broker := testBroker(t, db, user, 10000)
			txRepo := ImplTransactionRepository(db)
			if _, err := txRepo.SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, broker, user.CreatedAt.Add(time.Hour), models.TypeBuy, "OLD")}); err != nil {
				t.Fatal(err)
			}
			repo := ImplBrokersRepository(db)
			start := time.Now().UTC()
			reset, err := repo.SetCashBase(user.Id, broker.ID, 2345, start.Add(offset))
			if err != nil {
				t.Fatal(err)
			}
			if reset.CashBaseAsOf == nil || reset.CashBaseAsOf.Before(start) || reset.CashBaseAsOf.After(time.Now().UTC()) {
				t.Fatalf("reset cutoff not server now: %+v", reset)
			}
			assertCash(t, db, user.Id, broker.ID, 2345)
			// A late import after registration but before the reset is history too.
			if _, err := txRepo.SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, broker, start.Add(-time.Hour), models.TypeBuy, "LATE")}); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 2345)
			received := time.Now().UTC()
			late := &models.CashMutation{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 9999, Date: start.Add(-time.Hour), EmailReceivedAt: &received, RefNo: "late-history"}
			if _, err := ImplCashMutationRepository(db).SyncCashMutationBatch(user.Id, []*models.CashMutation{late}); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 2345)
			date := time.Now().UTC()
			if _, err := txRepo.CreateTransactionAndSyncHolding(testTrade(user, broker, date, models.TypeBuy, "NEW")); err != nil {
				t.Fatal(err)
			}
			mutation := &models.CashMutation{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 100, Date: date, RefNo: "after-reset"}
			if _, err := ImplCashMutationRepository(db).SyncCashMutationBatch(user.Id, []*models.CashMutation{mutation}); err != nil {
				t.Fatal(err)
			}
			assertCash(t, db, user.Id, broker.ID, 1444)
		})
	}
}

func TestCashLegacyPreservesUnverifiedBalance(t *testing.T) {
	db, user := cashTestDB(t)
	broker := testBroker(t, db, user, 4321)
	if err := db.Model(broker).Updates(map[string]any{"cash_base_as_of": nil, "cash_base": 0}).Error; err != nil {
		t.Fatal(err)
	}
	old := testTrade(user, broker, user.CreatedAt.Add(time.Hour), models.TypeBuy, "EXISTING")
	if err := db.Create(old).Error; err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, broker.ID, 4321)
	if _, err := ImplTransactionRepository(db).SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, broker, user.CreatedAt.Add(24*time.Hour), models.TypeBuy, "NEW")}); err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, broker.ID, 3320)
}

func TestCashTransfersAndAdditionsSurviveRecomputation(t *testing.T) {
	db, user := cashTestDB(t)
	from := testBroker(t, db, user, 10000)
	repo := ImplBrokersRepository(db)
	to, err := repo.AddBrokerData(&models.Broker{UserID: user.Id, Name: "Destination", Cash: 500})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImplTransactionRepository(db).SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, from, user.CreatedAt.Add(time.Hour), models.TypeBuy, "LIVE")}); err != nil {
		t.Fatal(err)
	}
	if err := repo.TransferCash(user.Id, from.ID, to.ID, 1000); err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, from.ID, 7999)
	assertCash(t, db, user.Id, to.ID, 1500)
	if _, err := repo.UpdateBrokerData(user.Id, to.ID, &models.Broker{Cash: 200}); err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, to.ID, 1700)
	for _, tc := range []struct {
		from, to int
		amount   float64
	}{
		{from.ID, from.ID, 100}, {from.ID, to.ID, -1}, {from.ID, to.ID, 0}, {from.ID, to.ID, 999999}, {from.ID, 99999, 100},
	} {
		if err := repo.TransferCash(user.Id, tc.from, tc.to, tc.amount); err == nil {
			t.Fatalf("invalid transfer accepted: %+v", tc)
		}
		assertCash(t, db, user.Id, from.ID, 7999)
		assertCash(t, db, user.Id, to.ID, 1700)
	}
	other := models.Users{Email: "other@example.com", CreatedAt: user.CreatedAt}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	foreign, err := repo.AddBrokerData(&models.Broker{UserID: other.Id, Name: "Other", Cash: 50})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.TransferCash(user.Id, from.ID, foreign.ID, 100); err == nil {
		t.Fatal("cross-user transfer accepted")
	}
	assertCash(t, db, user.Id, from.ID, 7999)
	assertCash(t, db, other.Id, foreign.ID, 50)
}

func TestCashUnknownCutoffAndDatesFailClosed(t *testing.T) {
	now := time.Now()
	zero := time.Time{}
	if CountsAfterCutoff(now, nil) || CountsAfterCutoff(now, &zero) || CountsAfterCutoff(zero, &now) || CountsAfterCutoff(now, &now) {
		t.Fatal("missing/zero/equal cutoff must never mean all history")
	}
	db, user := cashTestDB(t)
	broker := testBroker(t, db, user, 10000)
	if _, err := ImplTransactionRepository(db).SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, broker, time.Time{}, models.TypeBuy, "BAD")}); err == nil {
		t.Fatal("undated imported trade accepted")
	}
	if _, err := ImplCashMutationRepository(db).SyncCashMutationBatch(user.Id, []*models.CashMutation{{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 10}}); err == nil {
		t.Fatal("undated mutation accepted")
	}
	assertCash(t, db, user.Id, broker.ID, 10000)
}

func TestCashDeleteTradeOnlyReversesEligibleCash(t *testing.T) {
	db, user := cashTestDB(t)
	broker := testBroker(t, db, user, 5000)
	repo := ImplTransactionRepository(db)
	for _, date := range []time.Time{user.CreatedAt.Add(-time.Hour), user.CreatedAt.Add(time.Hour)} {
		trade := testTrade(user, broker, date, models.TypeBuy, "DELETE")
		if _, err := repo.CreateTransactionAndSyncHolding(trade); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteTransaction(user.Id, trade.Id); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteTransaction(user.Id, trade.Id); err == nil {
			t.Fatal("second delete accepted")
		}
		assertCash(t, db, user.Id, broker.ID, 5000)
	}
}

func TestCashDuplicateTimestampCannotCrossCashBoundary(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual", true: "batch"}[batch], func(t *testing.T) {
			db, user := cashTestDB(t)
			broker := testBroker(t, db, user, 5000)
			repo := ImplTransactionRepository(db)
			midnight := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
			persist := func(date time.Time) {
				trade := testTrade(user, broker, date, models.TypeBuy, "DUP")
				if batch {
					if _, err := repo.SyncTradeConfirmationBatch(user.Id, []*models.Transaction{trade}); err != nil {
						t.Fatal(err)
					}
				} else if _, err := repo.CreateTransactionAndSyncHolding(trade); err != nil {
					t.Fatal(err)
				}
			}
			persist(midnight)
			persist(user.CreatedAt.Add(time.Hour))
			assertCash(t, db, user.Id, broker.ID, 5000)
		})
	}
}
