package service

import (
	"context"
	"log"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/marchelrn/stock_api/contract"
)

// GmailScheduler menjalankan sync Gmail incremental secara berkala untuk user
// yang token Gmail-nya masih tersimpan. Jendela jam opsional mengikuti WITA (UTC+8).
type GmailScheduler struct {
	gmail contract.GmailService
	auth  contract.AuthRepository

	enabled   bool
	startHour int
	endHour   int
	loc       *time.Location

	// mu mencegah dua proses sync berjalan bersamaan, baik dari cron maupun
	// dari tombol manual di UI.
	mu sync.Mutex
}

// NewGmailScheduler membaca konfigurasi jam dari environment:
//
//	GMAIL_CRON_ENABLED     (default true)
//	GMAIL_CRON_START_HOUR  (default 0, WITA)
//	GMAIL_CRON_END_HOUR    (default 23, WITA)
func NewGmailScheduler(gmail contract.GmailService, auth contract.AuthRepository) *GmailScheduler {
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		loc = time.FixedZone("WITA", 8*60*60)
	}

	enabled := true
	if raw := os.Getenv("GMAIL_CRON_ENABLED"); raw != "" {
		enabled = raw == "1" || raw == "true" || raw == "TRUE"
	}

	return &GmailScheduler{
		gmail:     gmail,
		auth:      auth,
		enabled:   enabled,
		startHour: envHour("GMAIL_CRON_START_HOUR", 0),
		endHour:   envHour("GMAIL_CRON_END_HOUR", 23),
		loc:       loc,
	}
}

func envHour(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	hour, err := strconv.Atoi(raw)
	if err != nil || hour < 0 || hour > 23 {
		log.Printf("Nilai %s tidak valid (%q), memakai %d", key, raw, fallback)
		return fallback
	}
	return hour
}

// Start menjalankan loop penjadwal di goroutine terpisah.
func (s *GmailScheduler) Start() {
	if !s.enabled {
		log.Println("Penjadwal sync Gmail dimatikan (GMAIL_CRON_ENABLED=false)")
		return
	}
	log.Printf("Penjadwal Gmail: tick berikut %s; trade confirmation tiap jam 17:00–23:00 WITA pada hari bursa", nextGmailHour(time.Now().In(s.loc)).Format(time.RFC3339))
	go s.loop(context.Background())
}

func nextGmailHour(now time.Time) time.Time {
	return now.Truncate(time.Hour).Add(time.Hour)
}

func (s *GmailScheduler) loop(ctx context.Context) {
	for {
		next := nextGmailHour(time.Now().In(s.loc))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.runOnce(next)
		}
	}
}

// Weekday equity-market closures from KSEI PENG-0002/DIR/KSEI/0126 and PENG-0004/DIR/KSEI/0926.
var exchangeHolidays = map[int][]string{
	2026: {"01-01", "01-16", "02-16", "02-17", "03-18", "03-19", "03-20", "03-23", "03-24", "04-03", "05-01", "05-14", "05-15", "05-27", "05-28", "06-01", "06-16", "08-17", "08-25", "12-24", "12-25", "12-31"},
	2027: {"01-01", "01-05", "02-05", "03-08", "03-09", "03-10", "03-11", "03-12", "03-15", "03-25", "03-26", "05-06", "05-17", "05-18", "05-19", "05-20", "06-01", "08-17", "12-24", "12-31"},
}

func tradeConfirmationDue(at time.Time) bool {
	if at.Hour() < 17 || at.Weekday() == time.Saturday || at.Weekday() == time.Sunday {
		return false
	}
	holidays, ok := exchangeHolidays[at.Year()]
	if !ok {
		log.Printf("Kalender bursa %d belum tersedia; sinkronisasi TC dilewati", at.Year())
		return false
	}
	return !slices.Contains(holidays, at.Format("01-02"))
}

// runOnce menjalankan satu putaran sync. Dipisah dari loop agar bisa diuji.
func (s *GmailScheduler) runOnce(at time.Time) {
	hour := at.Hour()
	if hour < s.startHour || hour > s.endHour {
		log.Printf("Penjadwal sync Gmail: lewati %02d:00 WITA (di luar jendela %02d:00-%02d:59)",
			hour, s.startHour, s.endHour)
		return
	}

	if !s.mu.TryLock() {
		log.Println("Penjadwal sync Gmail: putaran sebelumnya masih berjalan, lewati jam ini")
		return
	}
	defer s.mu.Unlock()

	users, err := s.auth.ListGmailSyncCandidates(context.Background())
	if err != nil {
		log.Printf("Penjadwal sync Gmail: gagal memuat daftar user: %v", err)
		return
	}
	if len(users) == 0 {
		log.Println("Penjadwal sync Gmail: tidak ada user dengan token Gmail aktif")
		return
	}

	log.Printf("Penjadwal sync Gmail: mulai putaran %s WITA untuk %d user", at.Format("15:04"), len(users))
	tradeDue := tradeConfirmationDue(at.In(s.loc))
	for _, user := range users {
		if tradeDue {
			if _, err := s.gmail.SyncTradeConfirmations(user.Id); err != nil {
				log.Printf("Penjadwal sync Gmail: transaksi user %d gagal: %v", user.Id, err)
			}
		}
		if user.LastGmailSyncAt != nil && at.Sub(*user.LastGmailSyncAt) < 30*time.Minute {
			continue
		}
		if _, err := s.gmail.SyncCashMutations(user.Id); err != nil {
			log.Printf("Penjadwal sync Gmail: mutasi kas user %d gagal: %v", user.Id, err)
		}
	}
	log.Println("Penjadwal sync Gmail: putaran selesai")
}
