package service

import (
	"context"
	"log"
	"time"

	"github.com/marchelrn/stock_api/contract"
)

// StartPortfolioPerformance captures after the evening broker imports and retries until midnight WITA.
func StartPortfolioPerformance(ctx context.Context, repo contract.PortfolioPerformanceRepository, quotes contract.YahooService) {
	loc := time.FixedZone("WITA", 8*60*60)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			now := time.Now().In(loc)
			if now.Hour() == 23 && now.Minute() >= 30 && tradeConfirmationDue(now) {
				if err := capturePortfolioPerformance(ctx, now, repo, quotes); err != nil {
					log.Printf("Gagal mencatat kinerja portofolio: %v", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func capturePortfolioPerformance(ctx context.Context, day time.Time, repo contract.PortfolioPerformanceRepository, quotes contract.YahooService) error {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.AddDate(0, 0, 1)
	tickers, err := repo.MissingTickers(ctx, start, end)
	if err != nil {
		return err
	}
	deadline := day.Add(3 * time.Minute)
	if end.Before(deadline) {
		deadline = end
	}
	for len(tickers) > 0 && ctx.Err() == nil && time.Now().Before(deadline) {
		n := min(len(tickers), 12)
		_, _, _ = quotes.GetPrices(tickers[:n], true)
		tickers = tickers[n:]
	}
	now := time.Now()
	if !now.Before(end) {
		return nil
	}
	return repo.Capture(ctx, start, end, now)
}
