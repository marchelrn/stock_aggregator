package repository

import (
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
)

func TestCashDiscoveredBrokerHasRegistrationBaseline(t *testing.T) {
	db, user := cashTestDB(t)
	txRepo := ImplTransactionRepository(db)
	trade := &models.Transaction{BrokerName: "Discovered", Ticker: "HIST", Type: models.TypeBuy, Price: 10, Lot: 1, Tax: 1, Date: user.CreatedAt.Add(-time.Hour)}
	if count, err := txRepo.SyncTradeConfirmationBatch(user.Id, []*models.Transaction{trade}); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	broker, err := ImplBrokersRepository(db).GetBrokerByName(user.Id, "Discovered")
	if err != nil {
		t.Fatal(err)
	}
	if broker.CashBaseAsOf == nil || !broker.CashBaseAsOf.Equal(user.CreatedAt) || broker.CashBase != 0 {
		t.Fatalf("wrong discovered broker baseline: %+v", broker)
	}
	assertCash(t, db, user.Id, broker.ID, 0)
	if _, err := txRepo.SyncTradeConfirmationBatch(user.Id, []*models.Transaction{testTrade(user, broker, user.CreatedAt.Add(time.Hour), models.TypeSell, "HIST")}); err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, broker.ID, 999)
}

func TestCashHistoricalMutationDoesNotCreateBrokerOrHistory(t *testing.T) {
	db, user := cashTestDB(t)
	repo := ImplCashMutationRepository(db)
	count, err := repo.SyncCashMutationBatch(user.Id, []*models.CashMutation{{BrokerName: "Historical", Type: models.CashMutationDeposit, Amount: 500, Date: user.CreatedAt.Add(-time.Hour)}})
	if err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	for _, table := range []string{"brokers", "cash_mutations"} {
		var rows int64
		if err := db.Table(table).Count(&rows).Error; err != nil || rows != 0 {
			t.Fatalf("unexpected historical import into %s: rows=%d err=%v", table, rows, err)
		}
	}
	count, err = repo.SyncCashMutationBatch(user.Id, []*models.CashMutation{{BrokerName: "New", Type: models.CashMutationDeposit, Amount: 500, Date: user.CreatedAt.Add(time.Hour)}})
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	broker, err := ImplBrokersRepository(db).GetBrokerByName(user.Id, "New")
	if err != nil {
		t.Fatal(err)
	}
	if broker.CashBase != 0 || broker.CashBaseAsOf == nil || !broker.CashBaseAsOf.Equal(user.CreatedAt) {
		t.Fatalf("mutation-discovered baseline: %+v", broker)
	}
	assertCash(t, db, user.Id, broker.ID, 500)
}

func TestCashMutationBatchFailureRollsBackCashAndRows(t *testing.T) {
	db, user := cashTestDB(t)
	broker := testBroker(t, db, user, 1000)
	count, err := ImplCashMutationRepository(db).SyncCashMutationBatch(user.Id, []*models.CashMutation{
		{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 500, Date: user.CreatedAt.Add(time.Hour)},
		{BrokerID: 99999, Type: models.CashMutationDeposit, Amount: 600, Date: user.CreatedAt.Add(time.Hour)},
	})
	if err == nil || count != 0 {
		t.Fatalf("failed batch reported success: count=%d err=%v", count, err)
	}
	var rows int64
	if err := db.Model(&models.CashMutation{}).Count(&rows).Error; err != nil || rows != 0 {
		t.Fatalf("rolled-back mutations remain: rows=%d err=%v", rows, err)
	}
	assertCash(t, db, user.Id, broker.ID, 1000)
}

func TestCashTransferWriteFailureRollsBackBothBrokers(t *testing.T) {
	db, user := cashTestDB(t)
	from := testBroker(t, db, user, 5000)
	repo := ImplBrokersRepository(db)
	to, err := repo.AddBrokerData(&models.Broker{UserID: user.Id, Name: "Reject", Cash: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_cash_update BEFORE UPDATE ON brokers
		WHEN NEW.name = 'Reject' BEGIN SELECT RAISE(ABORT, 'simulated destination write failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.TransferCash(user.Id, from.ID, to.ID, 1000); err == nil {
		t.Fatal("expected destination write error")
	}
	assertCash(t, db, user.Id, from.ID, 5000)
	assertCash(t, db, user.Id, to.ID, 100)
}

func TestCashMissingRegistrationFailsClosedAndZeroResetWorks(t *testing.T) {
	db, user := cashTestDB(t)
	broker := testBroker(t, db, user, 5000)
	repo := ImplBrokersRepository(db)
	if _, err := repo.SetCashBase(user.Id, broker.ID, 0, user.CreatedAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertCash(t, db, user.Id, broker.ID, 0)
	if err := db.Model(&user).Update("created_at", time.Time{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetCashBreakdown(user.Id, broker.ID); err == nil {
		t.Fatal("missing registration must not enable all-history breakdown")
	}
	if _, err := ImplCashMutationRepository(db).SyncCashMutationBatch(user.Id, []*models.CashMutation{{BrokerID: broker.ID, Type: models.CashMutationDeposit, Amount: 500, Date: time.Now()}}); err == nil {
		t.Fatal("missing registration accepted for cash mutation")
	}
	if _, err := repo.SetCashBase(user.Id, broker.ID, 999, time.Now()); err == nil {
		t.Fatal("missing registration accepted for reset")
	}
	stored, err := repo.GetBrokerById(user.Id, broker.ID)
	if err != nil || stored.Cash != 0 {
		t.Fatalf("cash changed despite missing registration: %+v err=%v", stored, err)
	}
}
