package accrual

import (
	"errors"

	"github.com/ddyachkov/gophermart/internal/storage"
)

var ErrNotRegisteredOrder = errors.New("not registered order")
var ErrPageNotFound = errors.New("page not found")

type Accrualler interface {
	OrderAccrual(storage.Order) (bool, error)
}
