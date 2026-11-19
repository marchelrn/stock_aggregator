package service

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
)

// fakeAuthRepo hanya mengisi bagian yang dipakai penjadwal.
type fakeAuthRepo struct {
	contract.AuthRepository
	users []models.Users
	err   error
}

func (f *fakeAuthRepo) ListGmailSyncCandidates(context.Context) ([]models.Users, error) {
	return f.users, f.err
}

// fakeGmailService mencatat user mana yang benar-benar disinkronkan.
type fakeGmailService struct {
	contract.GmailService
	tradeCalls []uint
	cashCalls  []uint
	tradeErr   error
}

func (f *fakeGmailService) SyncTradeConfirmations(userID uint) (*dto.GmailSyncResponse, error) {
	f.tradeCalls = append(f.tradeCalls, userID)
	return &dto.GmailSyncResponse{}, f.tradeErr
}

func (f *fakeGmailService) SyncCashMutations(userID uint) (*dto.GmailCashSyncResponse, error) {
	f.cashCalls = append(f.cashCalls, userID)
	return &dto.GmailCashSyncResponse{}, nil
}

func newTestScheduler(auth contract.AuthRepository, gmail contract.GmailService) *GmailScheduler {
	return &GmailScheduler{
		gmail:     gmail,
		auth:      auth,
		enabled:   true,
		startHour: 17,
		endHour:   23,
		loc:       time.FixedZone("WITA", 8*60*60),
	}
}

func TestGmailSchedulerSkipsOutsideWindow(t *testing.T) {
	gmail := &fakeGmailService{}
	auth := &fakeAuthRepo{users: []models.Users{{Id: 1}}}
	scheduler := newTestScheduler(auth, gmail)

	at := time.Date(2026, time.November, 16, 16, 0, 0, 0, scheduler.loc)
	scheduler.runOnce(at)

	if len(gmail.tradeCalls) != 0 || len(gmail.cashCalls) != 0 {
		t.Fatalf("sync seharusnya dilewati di luar jendela, trade=%v cash=%v", gmail.tradeCalls, gmail.cashCalls)
	}
}

func TestGmailSchedulerSyncsEligibleUsersInsideWindow(t *testing.T) {
	oldSync := time.Date(2026, time.November, 16, 15, 0, 0, 0, time.FixedZone("WITA", 8*60*60))
	gmail := &fakeGmailService{}
	auth := &fakeAuthRepo{users: []models.Users{
		{Id: 1},
		{Id: 2, LastGmailSyncAt: &oldSync},
	}}
	scheduler := newTestScheduler(auth, gmail)

	at := time.Date(2026, time.November, 16, 17, 0, 0, 0, scheduler.loc)
	scheduler.runOnce(at)

	if len(gmail.tradeCalls) != 2 || len(gmail.cashCalls) != 2 {
		t.Fatalf("kedua user harus disinkronkan, trade=%v cash=%v", gmail.tradeCalls, gmail.cashCalls)
	}
	if gmail.tradeCalls[0] != 1 || gmail.tradeCalls[1] != 2 {
		t.Fatalf("urutan user tidak sesuai: %v", gmail.tradeCalls)
	}
}

func TestGmailSchedulerSkipsRecentlySyncedUser(t *testing.T) {
	gmail := &fakeGmailService{}
	at := time.Date(2026, time.November, 16, 17, 0, 0, 0, time.FixedZone("WITA", 8*60*60))
	recent := at.Add(-10 * time.Minute)
	auth := &fakeAuthRepo{users: []models.Users{
		{Id: 1, LastGmailSyncAt: &recent},
		{Id: 2},
	}}
	scheduler := newTestScheduler(auth, gmail)

	scheduler.runOnce(at)

	if len(gmail.tradeCalls) != 2 || gmail.tradeCalls[0] != 1 {
		t.Fatalf("kedua user harus mendapat tick TC, dapat %v", gmail.tradeCalls)
	}
	if len(gmail.cashCalls) != 1 || gmail.cashCalls[0] != 2 {
		t.Fatalf("hanya user 2 yang boleh disinkronkan kas, dapat %v", gmail.cashCalls)
	}
}

func TestHourlyGmailSchedule(t *testing.T) {
	t.Setenv("GMAIL_CRON_START_HOUR", "")
	t.Setenv("GMAIL_CRON_END_HOUR", "")
	synctest.Test(t, func(t *testing.T) {
		gmail := &fakeGmailService{}
		scheduler := NewGmailScheduler(gmail, &fakeAuthRepo{users: []models.Users{{Id: 1}}})
		start := time.Date(2026, 11, 18, 16, 37, 0, 0, scheduler.loc)
		time.Sleep(time.Until(start))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go scheduler.loop(ctx)
		synctest.Wait()
		time.Sleep(23*time.Minute - time.Second)
		synctest.Wait()
		if len(gmail.tradeCalls) != 0 {
			t.Fatal("early TC sync")
		}
		for hour := 17; hour <= 24; hour++ {
			time.Sleep(time.Until(time.Date(2026, 11, 18, hour, 0, 0, 0, scheduler.loc)))
			synctest.Wait()
			want := min(hour-16, 7)
			if len(gmail.tradeCalls) != want {
				t.Fatalf("%02d:00 WITA got %d TC syncs, want %d", hour, len(gmail.tradeCalls), want)
			}
		}
	})
}

func TestGmailTradeCalendar(t *testing.T) {
	loc := time.FixedZone("WITA", 8*60*60)
	for _, day := range []string{"2026-11-21", "2026-11-22", "2026-12-24", "2026-12-31", "2027-01-05"} {
		at, err := time.ParseInLocation("2006-01-02 15:04", day+" 21:00", loc)
		if err != nil || tradeConfirmationDue(at) {
			t.Fatalf("expected closed: %s", day)
		}
	}
}
