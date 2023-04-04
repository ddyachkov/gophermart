package accrual

import (
	"context"
	"errors"
	"time"

	"github.com/ddyachkov/gophermart/internal/storage"
)

var ErrNotRegisteredOrder = errors.New("not registered order")

type Accrualler interface {
	OrderAccrual(context.Context, *storage.Order) (time.Duration, error)
}
