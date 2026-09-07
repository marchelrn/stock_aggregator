package main

import (
	"fmt"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/internal/database"
	"github.com/marchelrn/stock_api/repository"
)

func main() {
	config.Load()
	db, _ := database.ConnectDB()
    
    repo := repository.ImplBrokersRepository(db)
    res, err := repo.GetAllBrokers()
    if err != nil {
        fmt.Println(err)
        return
    }
    
    var names []string
    for _, b := range res {
        names = append(names, b.Name)
    }

    details, _ := repo.GetBrokerDetails(names)
    for _, b := range details {
        fmt.Printf("Broker: %v\n", b.Name)
        for _, s := range b.Stocks {
            fmt.Printf("  Stock: %v, Id: %v\n", s.Ticker, s.Id)
        }
    }
}
