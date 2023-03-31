package accrual

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/go-resty/resty/v2"
)

type AccrualService struct {
	address string
	client  *resty.Client
	storage *storage.DBStorage
}

func NewAccrualService(a string, st *storage.DBStorage) (service *AccrualService) {
	return &AccrualService{
		address: a,
		client:  resty.New(),
		storage: st,
	}
}

func (as AccrualService) OrderAccrual(order storage.Order) (ready bool, err error) {
	serviceURL, err := url.JoinPath(as.address, "/api/orders/")
	if err != nil {
		return true, err
	}
	responce, err := as.client.R().SetResult(&order).Get(serviceURL + order.Number)
	if err != nil {
		return true, err
	}

	switch responce.StatusCode() {
	case http.StatusNoContent:
		return true, ErrNotRegisteredOrder
	case http.StatusNotFound:
		return true, ErrPageNotFound
	case http.StatusTooManyRequests:
		return false, nil
	}

	if order.Status == "REGISTERED" || order.Status == "PROCESSING" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = as.storage.UpdateOrderStatus(ctx, order)

	return true, err
}
