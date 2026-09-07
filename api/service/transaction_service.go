package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
)

func ImplTransactionService(repo *contract.Repository) contract.TransactionService {
	return &TransactionService{Transaction: repo}
}

type TransactionService struct {
	Transaction *contract.Repository
}

func (s *TransactionService) GetTransaction(id int) (*dto.TransactionResponse, error) {
	transactions, err := s.Transaction.Transactions.GetTransactions(id)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, errs.NotFound("transaction not found")
	}

	transaction := transactions[0]

	response := &dto.TransactionResponse{
		StatusCode: 200,
		Message:    "Transaction retrieved successfully",
		Data: dto.TransactionData{
			Id: transaction.Id,
			Broker: dto.BrokerTransaction{
				BrokerID: transaction.BrokerID,
				Name:     transaction.BrokerName,
			},
			Stock: dto.StockTransaction{
				Id:     transaction.StockID,
				Ticker: transaction.Ticker,
				Lot:    transaction.Lot,
				Price:  transaction.Price,
			},
			Type:       string(transaction.Type),
			AmountDone: transaction.AmountDone,
			Date:       transaction.Date,
		},
	}

	return response, nil
}

func (s *TransactionService) GetAllTransactions() (*dto.AllTransactionsResponse, error) {
	transactions, err := s.Transaction.Transactions.GetAllTransactions()
	if err != nil {
		return nil, err
	}

	var transactionData []dto.TransactionData
	for _, transaction := range transactions {
		transactionData = append(transactionData, dto.TransactionData{
			Id: transaction.Id,
			Broker: dto.BrokerTransaction{
				BrokerID: transaction.BrokerID,
				Name:     transaction.BrokerName,
			},
			Stock: dto.StockTransaction{
				Id:     transaction.StockID,
				Ticker: transaction.Ticker,
				Lot:    transaction.Lot,
				Price:  transaction.Price,
			},
			Type:       string(transaction.Type),
			AmountDone: transaction.AmountDone,
			Date:       transaction.Date,
		})
	}

	response := &dto.AllTransactionsResponse{
		StatusCode: 200,
		Message:    "Transactions retrieved successfully",
		Data:       transactionData,
	}

	return response, nil
}

func (s *TransactionService) CreateTransaction(ticker []string, payload *dto.TransactionRequest) (*dto.TransactionResponse, error) {
	if payload == nil {
		return nil, errs.BadRequest("request payload is required")
	}

	brokerID := payload.BrokerID
	if brokerID == 0 {
		brokerID = payload.BrokerID
	}
	if brokerID == 0 {
		return nil, errs.BadRequest("broker_id is required")
	}

	transactionTicker := strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if transactionTicker == "" {
		for _, t := range ticker {
			t = strings.ToUpper(strings.TrimSpace(t))
			if t != "" {
				transactionTicker = t
				break
			}
		}
	}
	if transactionTicker == "" {
		return nil, errs.BadRequest("stock ticker is required")
	}

	transactionLot := payload.Lot
	if transactionLot <= 0 {
		return nil, errs.BadRequest("stock lot must be greater than 0")
	}

	transactionPrice := payload.Price
	if transactionPrice <= 0 {
		return nil, errs.BadRequest("stock price must be greater than 0")
	}

	transactionType := models.TypeBuy
	requestType := strings.ToUpper(strings.TrimSpace(payload.Type))
	if requestType == "" {
		requestType = strings.ToUpper(strings.TrimSpace(payload.Type))
	}
	switch requestType {
	case "", string(models.TypeBuy):
		transactionType = models.TypeBuy
	case string(models.TypeSell):
		transactionType = models.TypeSell
	default:
		return nil, errs.BadRequest("type must be BUY or SELL")
	}

	transactionDate := payload.Date
	if transactionDate.IsZero() {
		transactionDate = time.Now()
	}

	transactionAmountDone := float64(transactionLot) * transactionPrice * 100
	if transactionAmountDone <= 0 {
		return nil, errs.BadRequest("transaction amount done must be greater than 0")
	}

	brokerName := strings.TrimSpace(payload.BrokerName)
	if brokerName == "" {
		broker, err := s.Transaction.Brokers.GetAllBrokers()
		if err != nil {
			return nil, errs.InternalServerError("failed to resolve broker")
		}
		for _, b := range broker {
			if b.ID == brokerID {
				brokerName = b.Name
				break
			}
		}
	}
	if brokerName == "" {
		return nil, errs.BadRequest("broker_name is required")
	}

	request := &models.Transaction{
		Ticker:     transactionTicker,
		Price:      transactionPrice,
		Lot:        transactionLot,
		BrokerID:   brokerID,
		BrokerName: brokerName,
		Type:       transactionType,
		Date:       transactionDate,
		AmountDone: transactionAmountDone,
	}

	transaction, err := s.Transaction.Transactions.CreateTransactionAndSyncHolding(request)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("broker or stock not found")
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "insufficient lot") || strings.Contains(msg, "cannot sell unknown stock") {
			return nil, errs.BadRequest(err.Error())
		}
		return nil, err
	}

	response := &dto.TransactionResponse{
		StatusCode: http.StatusOK,
		Message:    "Transaction created successfully",
		Data: dto.TransactionData{
			Id: transaction.Id,
			Broker: dto.BrokerTransaction{
				BrokerID: transaction.BrokerID,
				Name:     transaction.BrokerName,
			},
			Stock: dto.StockTransaction{
				Id:     transaction.StockID,
				Ticker: transaction.Ticker,
				Lot:    transaction.Lot,
				Price:  transaction.Price,
			},
			Type:       string(transaction.Type),
			AmountDone: transaction.AmountDone,
			Date:       transaction.Date,
		},
	}

	return response, nil
}

func (s *TransactionService) DeleteTransaction(id int) (*dto.BasicResponse, error) {
	err := s.Transaction.Transactions.DeleteTransaction(id)
	if err != nil {
		return nil, err
	}
	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    "Transaction deleted successfully",
	}

	return response, nil
}
