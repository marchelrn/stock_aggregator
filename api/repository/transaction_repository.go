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

		var existing models.Transaction
		err := tx.Where("user_id = ? AND ticker = ? AND type = ? AND lot = ? AND price = ? AND DATE(date) = DATE(?) AND broker_id = ?",
			payload.UserID, payload.Ticker, payload.Type, payload.Lot, payload.Price, payload.Date, payload.BrokerID).
			First(&existing).Error
		if err == nil {
			if (existing.Date.Hour() == 0 && existing.Date.Minute() == 0 && existing.Date.Second() == 0) &&
				!(payload.Date.Hour() == 0 && payload.Date.Minute() == 0 && payload.Date.Second() == 0) {
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
			if broker.Cash < amountDone {
				return fmt.Errorf("Insufficient broker cash: have Rp.%.1f, need Rp.%.1f", broker.Cash, amountDone)
			}

			// avg_price includes tax: amountDone / shares
			shares := float64(payload.Lot) * 100
			avgPriceWithTax := amountDone / shares

			if err == gorm.ErrRecordNotFound {
				stock = models.StockHolding{
					UserID:     payload.UserID,
					Ticker:     payload.Ticker,
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
			broker.Cash -= amountDone

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
			broker.Cash += (grossAmount - tax)

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

func (r *TransactionRepository) GetTransactions(userID uint, id int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) GetAllTransactions(userID uint) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Where("user_id = ?", userID).Order("date ASC").Find(&transactions).Error; err != nil {
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
	result := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Transaction{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *TransactionRepository) SyncTradeConfirmationBatch(userID uint, trades []*models.Transaction) (int, error) {
	if len(trades) == 0 {
		return 0, nil
	}

	type brokerDateKey struct {
		brokerID int
		dateStr  string
	}

	// 1. Resolve brokers and normalize fields
	for _, trade := range trades {
		trade.UserID = userID
		if trade.Date.IsZero() {
			trade.Date = time.Now()
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
				if createErr := r.db.Create(&newBroker).Error; createErr == nil {
					trade.BrokerID = newBroker.ID
				}
			}
		}
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

	// 3. Process each group in an isolated transaction
	for _, key := range orderedKeys {
		groupTrades := grouped[key]

		err := r.db.Transaction(func(tx *gorm.DB) error {
			var broker models.Broker
			if key.brokerID != 0 {
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, key.brokerID).First(&broker).Error; err != nil {
					return fmt.Errorf("lock broker %d: %w", key.brokerID, err)
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
				var tickerBuyLot, tickerSellLot int
				var tickerBuyCost, tickerSellProceeds float64

				for _, trade := range tradesForTicker {
					var existing models.Transaction
					err := tx.Where("user_id = ? AND ticker = ? AND type = ? AND lot = ? AND price = ? AND DATE(date) = DATE(?) AND broker_id = ?",
						trade.UserID, trade.Ticker, trade.Type, trade.Lot, trade.Price, trade.Date, trade.BrokerID).First(&existing).Error
					if err == nil {
						if (existing.Date.Hour() == 0 && existing.Date.Minute() == 0 && existing.Date.Second() == 0) &&
							!(trade.Date.Hour() == 0 && trade.Date.Minute() == 0 && trade.Date.Second() == 0) {
							_ = tx.Model(&existing).Update("date", trade.Date).Error
						}
						continue
					} else if err != gorm.ErrRecordNotFound {
						return err
					}

					gross := float64(trade.Lot) * 100 * trade.Price
					tax := trade.Tax
					if tax == 0 {
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
				newLot, newAvg, tickerCashDelta := CalculateHoldingAfterBatch(currentLot, currentAvgPrice, newTrades)

				if hasStock {
					stock.Lot = newLot
					stock.AvgPrice = newAvg
					stock.BrokerID = key.brokerID
					if err := tx.Save(&stock).Error; err != nil {
						return err
					}
				} else {
					stock = models.StockHolding{
						UserID:     userID,
						Ticker:     ticker,
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
					syncedCount++
				}

				// Reconcile holding across the full transaction history to guarantee
				// accuracy regardless of message ingestion order
				if err := ReconcileStockHolding(tx, userID, ticker, key.brokerID); err != nil {
					log.Printf("Warning: failed to reconcile stock holding for %s: %v", ticker, err)
				}

				brokerCashDelta += tickerCashDelta
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
			log.Printf("Failed syncing trade confirmation batch for broker %d on %s: %v", key.brokerID, key.dateStr, err)
			return syncedCount, err
		}
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
		if tax == 0 {
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
		// Both buy and sell on the same stock in this batch
		if currentLot >= tickerSellLot {
			// User had enough recorded holding to cover the sell
			remainingOldLot := currentLot - tickerSellLot
			newLot = remainingOldLot + tickerBuyLot
			if remainingOldLot == 0 {
				newAvg = tickerBuyCost / (float64(newLot) * 100)
			} else {
				newAvg = (float64(remainingOldLot)*100*currentAvgPrice + tickerBuyCost) / (float64(newLot) * 100)
			}
		} else {
			// currentLot < tickerSellLot: user sold more lot than recorded in DB.
			// This occurs when user had an unrecorded prior holding at their broker.
			if currentLot == 0 && tickerBuyLot == tickerSellLot {
				// Pure intraday scalp round-trip with no prior position: closed flat
				newLot = 0
				newAvg = 0
			} else if currentLot == 0 && tickerBuyLot > tickerSellLot {
				// User bought more than sold today without prior position
				newLot = tickerBuyLot - tickerSellLot
				newAvg = tickerBuyCost / (float64(tickerBuyLot) * 100)
			} else {
				// User sold an unrecorded prior position (e.g. PART: sold 253 old shares, bought 249 new shares).
				// The old unrecorded position was liquidated by the sell, and the new buy forms the active holding.
				newLot = tickerBuyLot
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
		return nil
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
