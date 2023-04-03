package accrual

import (
	"errors"

	"github.com/ddyachkov/gophermart/internal/storage"
)

var ErrNotRegisteredOrder = errors.New("not registered order")

type Accrualler interface {
	OrderAccrual(storage.Order) (bool, error)
}
