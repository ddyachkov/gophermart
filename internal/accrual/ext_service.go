package accrual

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/go-resty/resty/v2"
)

type AccrualService struct {
	address string
	client  *resty.Client
}

func NewAccrualService(a string) (service *AccrualService) {
	return &AccrualService{
		address: a,
		client:  resty.New(),
	}
}

func (as AccrualService) OrderAccrual(ctx context.Context, order *storage.Order) (delay time.Duration, err error) {
	serviceURL, err := url.JoinPath(as.address, "/api/orders/")
	if err != nil {
		return delay, err
	}
	responce, err := as.client.R().SetContext(ctx).SetResult(order).Get(serviceURL + order.Number)
	if err != nil {
		return delay, err
	}

	switch responce.StatusCode() {
	case http.StatusNoContent:
		return delay, ErrNotRegisteredOrder
	case http.StatusTooManyRequests:
		retry, err := strconv.Atoi(responce.Header().Get("Retry-After"))
		if err != nil {
			return delay, err
		}
		delay = time.Second * time.Duration(retry)
	}

	return delay, nil
}
