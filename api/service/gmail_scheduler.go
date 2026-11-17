package service

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/marchelrn/stock_api/contract"
)

// GmailScheduler menjalankan sync Gmail incremental secara berkala untuk user
// yang token Gmail-nya masih tersimpan. Jadwal mengikuti WITA (UTC+8) dan
// dimulai saat pasar tutup agar transaksi hari itu sudah lengkap.
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
//	GMAIL_CRON_START_HOUR  (default 17, WITA)
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
		startHour: envHour("GMAIL_CRON_START_HOUR", 17),
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
	log.Printf("Penjadwal sync Gmail aktif setiap jam %02d:00-%02d:59 WITA", s.startHour, s.endHour)
	go s.loop()
}

func (s *GmailScheduler) loop() {
	for {
		now := time.Now().In(s.loc)
		next := now.Truncate(time.Hour).Add(time.Hour)
		time.Sleep(time.Until(next))
		s.runOnce(next.In(s.loc))
	}
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

	log.Printf("Penjadwal sync Gmail: mulai putaran %02d:00 WITA untuk %d user", hour, len(users))
	for _, user := range users {
		// Lewati user yang baru saja sync lewat tombol manual agar email yang
		// sama tidak diproses dua kali.
		if user.LastGmailSyncAt != nil && at.Sub(*user.LastGmailSyncAt) < 30*time.Minute {
			log.Printf("Penjadwal sync Gmail: user %d baru sync, lewati", user.Id)
			continue
		}

		if _, err := s.gmail.SyncTradeConfirmations(user.Id); err != nil {
			log.Printf("Penjadwal sync Gmail: transaksi user %d gagal: %v", user.Id, err)
		}
		if _, err := s.gmail.SyncCashMutations(user.Id); err != nil {
			log.Printf("Penjadwal sync Gmail: mutasi kas user %d gagal: %v", user.Id, err)
		}
	}
	log.Println("Penjadwal sync Gmail: putaran selesai")
}
