package repository

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ImplTransactionRepository(db *gorm.DB) contract.TransactionRepository {
	return &TransactionRepository{db: db}
}

type TransactionRepository struct {
	db *gorm.DB
}

func (r *TransactionRepository) CreateTransactionAndSyncHolding(payload *models.Transaction) (*models.Transaction, error) {
	if payload.Date.IsZero() {
		payload.Date = time.Now()
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var broker models.Broker
		if payload.BrokerID == 0 && payload.BrokerName != "" {
			var b models.Broker
			if err := tx.Where("user_id = ? AND LOWER(name) = LOWER(?)", payload.UserID, strings.TrimSpace(payload.BrokerName)).First(&b).Error; err == nil {
				payload.BrokerID = b.ID
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", payload.UserID, payload.BrokerID).First(&broker).Error; err != nil {
			return err
		}

		if err := prepareCashBaseline(tx, payload.UserID, &broker); err != nil {
			return err
		}

		// NOTE: duplicate detection is identity-based (ticker/type/lot/price/
		// date below). RefNo must not be used as a uniqueness key: a single
		// trade confirmation legitimately contains several fills that share
		// the same Ref#, and blocking the second fill would corrupt holdings.

		var existing models.Transaction
		// Compare the date as a plain string: DATE(?) with a zoned
		// time.Time would shift the day for late-evening trades.
		err := tx.Where("user_id = ? AND ticker = ? AND type = ? AND lot = ? AND price = ? AND DATE(date) = ? AND broker_id = ?",
			payload.UserID, payload.Ticker, payload.Type, payload.Lot, payload.Price, payload.Date.Format("2006-01-02"), payload.BrokerID).
			First(&existing).Error
		if err == nil {
			// Timestamp enrichment must not turn a history-only duplicate into a cash event.
			if (existing.Date.Hour() == 0 && existing.Date.Minute() == 0 && existing.Date.Second() == 0) &&
				!(payload.Date.Hour() == 0 && payload.Date.Minute() == 0 && payload.Date.Second() == 0) &&
				CountsAfterCutoff(existing.Date, broker.CashBaseAsOf) == CountsAfterCutoff(payload.Date, broker.CashBaseAsOf) {
				if updateErr := tx.Model(&existing).Update("date", payload.Date).Error; updateErr != nil {
					return updateErr
				}
				log.Printf("Updated transaction %d (%s %s) date from %s to %s",
					existing.Id, existing.Type, existing.Ticker, existing.Date.Format(time.RFC3339), payload.Date.Format(time.RFC3339))
			} else {
				log.Printf("Transaction %s %s %d lot @ %.2f on %s already exists, skipping duplicate",
					payload.Type, payload.Ticker, payload.Lot, payload.Price, payload.Date.Format("2006-01-02"))
			}
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		var stock models.StockHolding
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND ticker = ? AND broker_id = ?", payload.UserID, payload.Ticker, payload.BrokerID).First(&stock).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		grossAmount := float64(payload.Lot) * 100 * payload.Price

		// Mode cash-base: transaksi yang terjadi sebelum cutoff sudah
		// termasuk di saldo dasar, jadi tidak boleh mengubah cash lagi.
		applyCash := CountsAfterCutoff(payload.Date, broker.CashBaseAsOf)

		// Use the actual tax from the parsed trade confirmation if available,
		// otherwise fallback to default broker fee rates
		tax := payload.Tax
		if tax == 0 {
			switch payload.Type {
			case models.TypeBuy:
				tax = grossAmount * 0.0015
			case models.TypeSell:
				tax = grossAmount * 0.0025
			}
		}

		// amountDone = gross + tax (total cost including fees)
		amountDone := grossAmount + tax

		switch payload.Type {
		case models.TypeBuy:
			if applyCash && broker.Cash < amountDone {
				return fmt.Errorf("Insufficient broker cash: have Rp.%.1f, need Rp.%.1f", broker.Cash, amountDone)
			}

			// avg_price includes tax: amountDone / shares
			shares := float64(payload.Lot) * 100
			avgPriceWithTax := amountDone / shares

			if err == gorm.ErrRecordNotFound {
				var defaultSector string
				_ = tx.Table("stocks").Select("sector").Where("ticker = ? AND sector IS NOT NULL AND sector != ''", payload.Ticker).Limit(1).Scan(&defaultSector).Error
				if defaultSector == "" {
					_ = tx.Table("stock_prices").Select("sector").Where("ticker = ? AND sector IS NOT NULL AND sector != ''", payload.Ticker).Limit(1).Scan(&defaultSector).Error
				}
				stock = models.StockHolding{
					UserID:     payload.UserID,
					Ticker:     payload.Ticker,
					Sector:     defaultSector,
					Lot:        payload.Lot,
					AvgPrice:   avgPriceWithTax,
					BrokerID:   payload.BrokerID,
					BrokerName: payload.BrokerName,
				}
				if err := tx.Create(&stock).Error; err != nil {
					return err
				}
			} else {
				if stock.Lot <= 0 {
					stock.Lot = payload.Lot
					stock.AvgPrice = avgPriceWithTax
				} else {
					oldTotalValue := float64(stock.Lot) * 100 * stock.AvgPrice
					newTotalValue := amountDone
					newLot := stock.Lot + payload.Lot
					newAvg := (oldTotalValue + newTotalValue) / (float64(newLot) * 100)

					stock.Lot = newLot
					stock.AvgPrice = newAvg
				}
				stock.BrokerID = payload.BrokerID
				stock.BrokerName = payload.BrokerName
				if err := tx.Save(&stock).Error; err != nil {
					return err
				}
			}

			// BUY: cash keluar = gross + tax
			if applyCash {
				broker.Cash -= amountDone
			}

		case models.TypeSell:
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("cannot sell unknown stock %s", payload.Ticker)
			}
			if stock.Lot < payload.Lot {
				return fmt.Errorf("insufficient lot: have %d, want to sell %d", stock.Lot, payload.Lot)
			}

			stock.Lot -= payload.Lot
			if stock.Lot == 0 {
				stock.AvgPrice = 0
			}
			stock.BrokerID = payload.BrokerID
			stock.BrokerName = payload.BrokerName

			if err := tx.Save(&stock).Error; err != nil {
				return err
			}

			// SELL: cash masuk = gross - tax
			if applyCash {
				broker.Cash += (grossAmount - tax)
			}

		default:
			return fmt.Errorf("invalid transaction type: %s", payload.Type)
		}

		if err := tx.Save(&broker).Error; err != nil {
			return err
		}

		payload.StockID = stock.Id
		if err := tx.Create(payload).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return payload, nil
}

// History dates are WITA wall times in PostgreSQL's timestamp-without-zone column.
const transactionHistoryColumns = "id, user_id, ticker, price, lot, stock_id, broker_id, broker_name, type, date AT TIME ZONE 'Asia/Makassar' AS date, amount_done, tax, ref_no"

func (r *TransactionRepository) GetTransactions(userID uint, id int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Select(transactionHistoryColumns).Where("user_id = ? AND id = ?", userID, id).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) GetAllTransactions(userID uint) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Select(transactionHistoryColumns).Where("user_id = ?", userID).Order("date DESC").Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) CountTransactionsByTicker(userID uint, ticker string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Transaction{}).Where("user_id = ? AND ticker = ?", userID, ticker).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *TransactionRepository) DeleteTransaction(userID uint, id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var trade models.Transaction
		if err := tx.Where("user_id = ? AND id = ?", userID, id).First(&trade).Error; err != nil {
			return err
		}
		var broker models.Broker
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, trade.BrokerID).First(&broker).Error; err != nil {
			return err
		}
		if err := prepareCashBaseline(tx, userID, &broker); err != nil {
			return err
		}
		result := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Transaction{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if CountsAfterCutoff(trade.Date, broker.CashBaseAsOf) {
			broker.Cash -= TradeCashDelta(&trade)
		}
		if err := ReconcileStockHolding(tx, userID, trade.Ticker, trade.BrokerID); err != nil {
			return err
		}
		return tx.Save(&broker).Error
	})
}

func (r *TransactionRepository) SyncTradeConfirmationBatch(userID uint, trades []*models.Transaction) (int, error) {
	if len(trades) == 0 {
		return 0, nil
	}

	type brokerDateKey struct {
		brokerID int
		dateStr  string
	}

	// 1. Resolve brokers, normalize fields, and drop duplicates within the batch.
	// Stockbit delivers duplicate TC emails for the same confirmation, so the
	// same row can appear twice in one batch. Two keys are used:
	//   - (ref, ticker, type, lot, price): one confirmation can carry several
	//     fills sharing a Ref# (partial executions), but the same fill may also
	//     arrive in documents with different timestamps/dates. RefNo with the
	//     fill details identifies it either way.
	//   - (ticker, type, lot, price, date, broker): catches copies with empty
	//     or differing RefNos (e.g. daily TC vs per-trade email).
	seenRefFills := make(map[string]struct{}, len(trades))
	seenIdentities := make(map[string]struct{}, len(trades))
	deduped := make([]*models.Transaction, 0, len(trades))
	for _, trade := range trades {
		trade.UserID = userID
		if trade.Date.IsZero() {
			return 0, fmt.Errorf("trade date is required for cash eligibility")
		}
		trade.Ticker = strings.ToUpper(strings.TrimSpace(trade.Ticker))
		if trade.BrokerID == 0 && trade.BrokerName != "" {
			var b models.Broker
			if err := r.db.Where("user_id = ? AND LOWER(name) = LOWER(?)", userID, strings.TrimSpace(trade.BrokerName)).First(&b).Error; err == nil {
				trade.BrokerID = b.ID
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				newBroker := models.Broker{
					UserID: userID,
					Name:   strings.TrimSpace(trade.BrokerName),
					Cash:   0,
				}
				if _, createErr := ImplBrokersRepository(r.db).AddBrokerData(&newBroker); createErr != nil {
					return 0, createErr
				}
				trade.BrokerID = newBroker.ID
			}
		}

		if trade.RefNo != "" {
			// Numeric RefNos appear with and without leading zeros depending on
			// the document (daily TC vs per-trade email); canonicalize so the
			// same confirmation is stored consistently.
			ref := strings.TrimSpace(trade.RefNo)
			canonical := strings.TrimLeft(ref, "0")
			if canonical == "" {
				canonical = ref
			}
			trade.RefNo = canonical

			refFill := fmt.Sprintf("%s|%s|%s|%d|%s",
				trade.RefNo, trade.Ticker, trade.Type, trade.Lot,
				strconv.FormatFloat(trade.Price, 'f', -1, 64))
			if _, dup := seenRefFills[refFill]; dup {
				continue
			}
			seenRefFills[refFill] = struct{}{}
		}

		identity := fmt.Sprintf("%s|%s|%d|%s|%s|%d",
			trade.Ticker, trade.Type, trade.Lot,
			strconv.FormatFloat(trade.Price, 'f', -1, 64),
			trade.Date.Format("2006-01-02"), trade.BrokerID)
		if trade.ReviewedFee && trade.RefNo != "" {
			identity += "|" + trade.RefNo
		}
		if _, dup := seenIdentities[identity]; dup {
			continue
		}
		seenIdentities[identity] = struct{}{}
		deduped = append(deduped, trade)
	}
	trades = deduped
	if len(trades) == 0 {
		return 0, nil
	}

	// 2. Group by (BrokerID, Date)
	grouped := make(map[brokerDateKey][]*models.Transaction)
	var orderedKeys []brokerDateKey
	for _, trade := range trades {
		key := brokerDateKey{
			brokerID: trade.BrokerID,
			dateStr:  trade.Date.Format("2006-01-02"),
		}
		if _, exists := grouped[key]; !exists {
			orderedKeys = append(orderedKeys, key)
		}
		grouped[key] = append(grouped[key], trade)
	}

	// Sort groups chronologically (oldest date first) so history is built sequentially
	sort.SliceStable(orderedKeys, func(i, j int) bool {
		if orderedKeys[i].dateStr != orderedKeys[j].dateStr {
			return orderedKeys[i].dateStr < orderedKeys[j].dateStr
		}
		return orderedKeys[i].brokerID < orderedKeys[j].brokerID
	})

	syncedCount := 0
	var groupErrs []error

	// 3. Process each group in an isolated transaction
	for _, key := range orderedKeys {
		groupTrades := grouped[key]
		groupSynced := 0

		err := r.db.Transaction(func(tx *gorm.DB) error {
			var broker models.Broker
			if key.brokerID != 0 {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, key.brokerID).First(&broker).Error; err != nil {
					return fmt.Errorf("lock broker %d: %w", key.brokerID, err)
				}
				if err := prepareCashBaseline(tx, userID, &broker); err != nil {
					return err
				}
			}

			// Group by Ticker within this broker & date
			tickerMap := make(map[string][]*models.Transaction)
			var orderedTickers []string
			for _, trade := range groupTrades {
				if _, exists := tickerMap[trade.Ticker]; !exists {
					orderedTickers = append(orderedTickers, trade.Ticker)
				}
				tickerMap[trade.Ticker] = append(tickerMap[trade.Ticker], trade)
			}

			var brokerCashDelta float64

			for _, ticker := range orderedTickers {
				tradesForTicker := tickerMap[ticker]

				var stock models.StockHolding
				stockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND ticker = ? AND broker_id = ?", userID, ticker, key.brokerID).First(&stock).Error
				hasStock := (stockErr == nil)
				currentLot := 0
				currentAvgPrice := 0.0
				if hasStock && stock.Lot > 0 {
					currentLot = stock.Lot
					currentAvgPrice = stock.AvgPrice
				}

				var newTrades []*models.Transaction

				for _, trade := range tradesForTicker {
					// Duplicate check: a fill is already stored when the same
					// RefNo carries the same ticker/type/lot/price (a
					// confirmation may repeat in documents with different
					// dates), or when the identical row exists for the same
					// date. RefNo alone must NOT be a uniqueness key because
					// one confirmation can contain several distinct fills.
					// Dates are compared as plain strings: passing a zoned
					// time.Time would shift the day for late-evening trades.
					query := tx.Where("user_id = ? AND ticker = ? AND type = ? AND lot = ? AND price = ? AND broker_id = ?",
						trade.UserID, trade.Ticker, trade.Type, trade.Lot, trade.Price, trade.BrokerID)
					if trade.ReviewedFee && trade.RefNo != "" {
						query = query.Where("(ref_no = ? OR (COALESCE(ref_no,'')='' AND DATE(date)=?))", trade.RefNo, trade.Date.Format("2006-01-02"))
					} else if trade.RefNo != "" {
						query = query.Where("(ref_no = ? OR DATE(date) = ?)",
							trade.RefNo, trade.Date.Format("2006-01-02"))
					} else {
						query = query.Where("DATE(date) = ?", trade.Date.Format("2006-01-02"))
					}
					var existing models.Transaction
					err := query.First(&existing).Error
					if err == nil {
						// Keep a duplicate on its original side of the cash boundary.
						if (existing.Date.Hour() == 0 && existing.Date.Minute() == 0 && existing.Date.Second() == 0) &&
							!(trade.Date.Hour() == 0 && trade.Date.Minute() == 0 && trade.Date.Second() == 0) &&
							CountsAfterCutoff(existing.Date, broker.CashBaseAsOf) == CountsAfterCutoff(trade.Date, broker.CashBaseAsOf) {
							if err := tx.Model(&existing).Update("date", trade.Date).Error; err != nil {
								return err
							}
						}
						continue
					} else if err != gorm.ErrRecordNotFound {
						return err
					}

					newTrades = append(newTrades, trade)
				}

				if len(newTrades) == 0 {
					continue
				}

				// Sort newTrades chronologically by RefNo if present
				sort.SliceStable(newTrades, func(i, j int) bool {
					refA := strings.TrimSpace(newTrades[i].RefNo)
					refB := strings.TrimSpace(newTrades[j].RefNo)
					if refA != "" && refB != "" {
						numA, errA := strconv.ParseInt(refA, 10, 64)
						numB, errB := strconv.ParseInt(refB, 10, 64)
						if errA == nil && errB == nil {
							return numA < numB
						}
						return refA < refB
					}
					return false
				})

				// Calculate new holding lot, average price, and cash delta
				newLot, newAvg, _ := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, newTrades)

				if hasStock {
					stock.Lot = newLot
					stock.AvgPrice = newAvg
					stock.BrokerID = key.brokerID
					if err := tx.Save(&stock).Error; err != nil {
						return err
					}
				} else {
					var defaultSector string
					_ = tx.Table("stocks").Select("sector").Where("ticker = ? AND sector IS NOT NULL AND sector != ''", ticker).Limit(1).Scan(&defaultSector).Error
					if defaultSector == "" {
						_ = tx.Table("stock_prices").Select("sector").Where("ticker = ? AND sector IS NOT NULL AND sector != ''", ticker).Limit(1).Scan(&defaultSector).Error
					}
					stock = models.StockHolding{
						UserID:     userID,
						Ticker:     ticker,
						Sector:     defaultSector,
						Lot:        newLot,
						AvgPrice:   newAvg,
						BrokerID:   key.brokerID,
						BrokerName: broker.Name,
					}
					if err := tx.Create(&stock).Error; err != nil {
						return err
					}
					hasStock = true
				}

				for _, trade := range newTrades {
					trade.StockID = stock.Id
					if err := tx.Create(trade).Error; err != nil {
						return err
					}
					groupSynced++
				}

				// Reconcile holding across the full transaction history to guarantee
				// accuracy regardless of message ingestion order
				if err := ReconcileStockHolding(tx, userID, ticker, key.brokerID); err != nil {
					log.Printf("Warning: failed to reconcile stock holding for %s: %v", ticker, err)
				}

				// Mode cash-base: hanya transaksi setelah cutoff yang
				// mengubah cash. Sebelum cutoff sudah termasuk saldo dasar.
				for _, trade := range newTrades {
					if CountsAfterCutoff(trade.Date, broker.CashBaseAsOf) {
						brokerCashDelta += TradeCashDelta(trade)
					}
				}
			}

			if key.brokerID != 0 && brokerCashDelta != 0 {
				broker.Cash += brokerCashDelta
				if err := tx.Save(&broker).Error; err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			// Keep processing the remaining days so one bad group does not
			// discard the rest of the history; the aggregated error still
			// propagates so the caller does not advance its checkpoint.
			log.Printf("Failed syncing trade confirmation batch for broker %d on %s: %v", key.brokerID, key.dateStr, err)
			groupErrs = append(groupErrs, fmt.Errorf("broker %d tanggal %s: %w", key.brokerID, key.dateStr, err))
			continue
		}
		syncedCount += groupSynced
	}

	if len(groupErrs) > 0 {
		return syncedCount, fmt.Errorf("gagal menyimpan %d kelompok transaksi (contoh: %w)", len(groupErrs), groupErrs[0])
	}

	return syncedCount, nil
}

// CalculateHoldingAfterBatch calculates the updated holding lot, moving average buy price,
// and net cash delta from a batch of trades on a single ticker.
// It gracefully handles intraday trading, buy-then-sell, sell-then-buyback, and unrecorded prior positions.
func CalculateHoldingAfterBatch(currentLot int, currentAvgPrice float64, trades []*models.Transaction) (int, float64, float64) {
	if len(trades) == 0 {
		return currentLot, currentAvgPrice, 0
	}

	var tickerBuyLot, tickerSellLot int
	var tickerBuyCost, tickerSellProceeds float64

	for _, trade := range trades {
		gross := float64(trade.Lot) * 100 * trade.Price
		tax := trade.Tax
		if tax == 0 && !trade.ReviewedFee {
			switch trade.Type {
			case models.TypeBuy:
				tax = gross * 0.0015
			case models.TypeSell:
				tax = gross * 0.0025
			}
			trade.Tax = tax
		}
		if trade.AmountDone == 0 {
			trade.AmountDone = gross + tax
		}

		if trade.Type == models.TypeBuy {
			tickerBuyLot += trade.Lot
			tickerBuyCost += gross + tax
		} else if trade.Type == models.TypeSell {
			tickerSellLot += trade.Lot
			tickerSellProceeds += (gross - tax)
		}
	}

	netCashDelta := tickerSellProceeds - tickerBuyCost
	var newLot int
	var newAvg float64

	if tickerBuyLot == 0 && tickerSellLot > 0 {
		// Pure sell
		if currentLot > tickerSellLot {
			newLot = currentLot - tickerSellLot
			newAvg = currentAvgPrice
		} else {
			// Position fully closed or unrecorded prior holding liquidated
			newLot = 0
			newAvg = 0
		}
	} else if tickerSellLot == 0 && tickerBuyLot > 0 {
		// Pure buy
		newLot = currentLot + tickerBuyLot
		if currentLot == 0 {
			newAvg = tickerBuyCost / (float64(newLot) * 100)
		} else {
			newAvg = (float64(currentLot)*100*currentAvgPrice + tickerBuyCost) / (float64(newLot) * 100)
		}
	} else if tickerBuyLot > 0 && tickerSellLot > 0 {
		// Both buy and sell on the same stock in this batch/day.
		naiveLot := currentLot + tickerBuyLot - tickerSellLot
		if naiveLot < 0 {
			// Sold more than the recorded holding plus today's buys: the user
			// held an unrecorded prior position that was liquidated. Keep the
			// legacy result (today's buys remain as the active holding).
			newLot = tickerBuyLot
			newAvg = tickerBuyCost / (float64(tickerBuyLot) * 100)
		} else {
			// Sells are covered by the recorded holding and today's buys.
			newLot = naiveLot
			switch {
			case newLot == 0:
				newAvg = 0
			case tickerSellLot <= currentLot:
				// Sells only consumed old shares; blend remaining old + new buys.
				remainingOldLot := currentLot - tickerSellLot
				if remainingOldLot == 0 {
					newAvg = tickerBuyCost / (float64(newLot) * 100)
				} else {
					newAvg = (float64(remainingOldLot)*100*currentAvgPrice + tickerBuyCost) / (float64(newLot) * 100)
				}
			default:
				// Sells consumed the whole old position plus part of today's
				// buys; the remainder consists of today's buys only.
				newAvg = tickerBuyCost / (float64(tickerBuyLot) * 100)
			}
		}
	} else {
		newLot = currentLot
		newAvg = currentAvgPrice
	}

	return newLot, newAvg, netCashDelta
}

// ReconcileStockHolding recalculates a user's stock holding lot and average price
// by replaying all historical transactions in chronological order (Date ASC, Id ASC).
func ReconcileStockHolding(tx *gorm.DB, userID uint, ticker string, brokerID int) error {
	var allTrades []models.Transaction
	if err := tx.Where("user_id = ? AND ticker = ? AND broker_id = ?", userID, ticker, brokerID).
		Order("date ASC, id ASC").
		Find(&allTrades).Error; err != nil {
		return err
	}

	if len(allTrades) == 0 {
		return tx.Model(&models.StockHolding{}).Where("user_id = ? AND ticker = ? AND broker_id = ?", userID, ticker, brokerID).
			Updates(map[string]interface{}{"lot": 0, "avg_price": 0}).Error
	}

	// Group transactions by date
	type dayKey string
	grouped := make(map[dayKey][]*models.Transaction)
	var orderedDays []dayKey

	for i := range allTrades {
		t := &allTrades[i]
		dKey := dayKey(t.Date.Format("2006-01-02"))
		if _, exists := grouped[dKey]; !exists {
			orderedDays = append(orderedDays, dKey)
		}
		grouped[dKey] = append(grouped[dKey], t)
	}

	sort.SliceStable(orderedDays, func(i, j int) bool {
		return orderedDays[i] < orderedDays[j]
	})

	runningLot := 0
	runningAvg := 0.0

	for _, dKey := range orderedDays {
		dayTrades := grouped[dKey]
		// Sort intraday trades by RefNo if present, otherwise by ID
		sort.SliceStable(dayTrades, func(i, j int) bool {
			refA := strings.TrimSpace(dayTrades[i].RefNo)
			refB := strings.TrimSpace(dayTrades[j].RefNo)
			if refA != "" && refB != "" {
				numA, errA := strconv.ParseInt(refA, 10, 64)
				numB, errB := strconv.ParseInt(refB, 10, 64)
				if errA == nil && errB == nil {
					return numA < numB
				}
				return refA < refB
			}
			return dayTrades[i].Id < dayTrades[j].Id
		})

		newLot, newAvg, _ := CalculateHoldingAfterBatch(runningLot, runningAvg, dayTrades)
		runningLot = newLot
		runningAvg = newAvg
	}

	return tx.Model(&models.StockHolding{}).
		Where("user_id = ? AND ticker = ? AND broker_id = ?", userID, ticker, brokerID).
		Updates(map[string]interface{}{
			"lot":       runningLot,
			"avg_price": runningAvg,
		}).Error
}
