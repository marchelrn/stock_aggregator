package repository

import (
	"context"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"gorm.io/gorm"
)

type portfolioPerformance struct{ db *gorm.DB }

const performanceHoldings = `
	FROM brokers b
	JOIN users u ON u.id = b.user_id
		AND (u.deleted_at IS NULL OR u.deleted_at = TIMESTAMP '0001-01-01')
	LEFT JOIN stocks s ON s.broker_id = b.id AND s.user_id = b.user_id AND s.lot > 0
	LEFT JOIN stock_prices p ON p.ticker = upper(trim(s.ticker))
		AND p.price > 0 AND (p.quoted_at IS NULL OR p.quoted_at < ?)
	WHERE b.deleted_at IS NULL`

func (r portfolioPerformance) List(ctx context.Context, userID uint, since string) ([]dto.PortfolioPerformancePoint, error) {
	points := make([]dto.PortfolioPerformancePoint, 0)
	err := r.db.WithContext(ctx).Raw(`
		SELECT to_char(session_date, 'YYYY-MM-DD') AS date, broker_id, broker_name,
		       total_value, unpriced_holdings, captured_at
		FROM portfolio_performance
		WHERE user_id = ? AND session_date >= CAST(? AS date)
		ORDER BY session_date, broker_id`, userID, since).Scan(&points).Error
	return points, err
}

func (r portfolioPerformance) MissingTickers(ctx context.Context, start, end time.Time) ([]string, error) {
	var tickers []string
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT upper(trim(s.ticker))`+performanceHoldings+
		` AND s.id IS NOT NULL AND trim(s.ticker) <> ''
		AND (p.id IS NULL OR p.quoted_at IS NULL OR p.quoted_at < ?
			OR p.observed_at IS NULL OR p.observed_at < ?) ORDER BY 1`,
		end, start, start.Add(17*time.Hour+30*time.Minute)).Scan(&tickers).Error
	return tickers, err
}

func (r portfolioPerformance) Capture(ctx context.Context, start, end, capturedAt time.Time) error {
	date := start.Format("2006-01-02")
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('portfolio_performance'), CAST(? AS date) - DATE '2000-01-01')", date).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM portfolio_performance WHERE session_date = CAST(? AS date)", date).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO portfolio_performance
				(user_id, session_date, broker_id, broker_name, total_value, unpriced_holdings, captured_at)
			SELECT b.user_id, CAST(? AS date), b.id, b.name,
				b.cash + COALESCE(sum(s.lot * 100::numeric * COALESCE(p.price, s.avg_price)), 0),
				count(s.id) FILTER (WHERE p.quoted_at IS NULL OR p.quoted_at < ?
					OR p.observed_at IS NULL OR p.observed_at < ?), ?
			`+performanceHoldings+` GROUP BY b.user_id, b.id, b.name, b.cash`,
			date, start, start.Add(17*time.Hour+30*time.Minute), capturedAt, end).Error
	})
}
