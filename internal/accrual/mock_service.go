package accrual

import (
	"context"
	"strconv"
	"time"

	"github.com/ddyachkov/gophermart/internal/random"
	"github.com/ddyachkov/gophermart/internal/storage"
)

type MockService struct {
	storage *storage.DBStorage
}

func NewMockService(st *storage.DBStorage) (service *MockService) {
	return &MockService{
		storage: st,
	}
}

func (as MockService) OrderAccrual(order storage.Order) (ready bool, err error) {
	sum64, err := strconv.ParseFloat(random.DigitString(1, 3), 32)
	if err != nil {
		return true, err
	}
	order.Accrual = float32(sum64)
	order.Status = "PROCESSED"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = as.storage.UpdateOrderStatus(ctx, order)

	return true, err
}
