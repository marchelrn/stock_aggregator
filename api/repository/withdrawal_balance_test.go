package repository

import (
	"os"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestImportedWithdrawalFloor(t *testing.T) {
	dsn := os.Getenv("WITHDRAWAL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires an empty disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.AutoMigrate(&models.Users{}, &models.Broker{}, &models.CashMutation{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("CREATE TEMP TABLE transactions (user_id bigint, broker_id bigint, date timestamptz)").Error; err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-time.Hour)
	user := models.Users{CreatedAt: start}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	b := models.Broker{UserID: user.Id, Name: "test", Cash: 100, CashBase: 100, CashBaseAsOf: &start}
	if err := tx.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	repo := ImplCashMutationRepository(tx)
	for _, step := range []struct {
		kind            models.CashMutationType
		amount, balance float64
		count           int
	}{
		{models.CashMutationWithdraw, 30, 70, 1},
		{models.CashMutationWithdraw, 120, 0, 1},
		{models.CashMutationWithdraw, 120, 0, 0},
		{models.CashMutationWithdraw, 10, 0, 1},
		{models.CashMutationDeposit, 25, 25, 1},
	} {
		item := &models.CashMutation{BrokerID: b.ID, Type: step.kind, Amount: step.amount, Date: start.Add(time.Minute)}
		n, err := repo.SyncCashMutationBatch(user.Id, []*models.CashMutation{item})
		if err != nil || n != step.count {
			t.Fatalf("sync %v: count=%d err=%v", step, n, err)
		}
		if err := tx.First(&b, b.ID).Error; err != nil || b.Cash != step.balance {
			t.Fatalf("stored cash %v: %v", b.Cash, err)
		}
		breakdown, err := ImplBrokersRepository(tx).GetCashBreakdown(user.Id, b.ID)
		if err != nil || breakdown.Cash != step.balance {
			t.Fatalf("breakdown for %v: %+v %v", step, breakdown, err)
		}
	}
	var recorded models.CashMutation
	if err := tx.Where("amount = ?", 120).First(&recorded).Error; err != nil || MutationCashDelta(&recorded) != -70 {
		t.Fatalf("withdrawal history: %+v %v", recorded, err)
	}
}
