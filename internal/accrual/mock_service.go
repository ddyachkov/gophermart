package accrual

import (
	"context"
	"strconv"
	"time"

	"github.com/ddyachkov/gophermart/internal/random"
	"github.com/ddyachkov/gophermart/internal/storage"
)

type MockService struct{}

func NewMockService() (service *MockService) {
	return &MockService{}
}

func (as MockService) OrderAccrual(ctx context.Context, order *storage.Order) (delay time.Duration, err error) {
	sum64, err := strconv.ParseFloat(random.DigitString(1, 3), 32)
	if err != nil {
		return delay, err
	}
	order.Accrual = float32(sum64)
	order.Status = "PROCESSED"

	return delay, nil
}
