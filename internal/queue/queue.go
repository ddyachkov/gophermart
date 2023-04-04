package queue

import (
	"context"
	"log"
	"time"

	"github.com/ddyachkov/gophermart/internal/accrual"
	"github.com/ddyachkov/gophermart/internal/storage"
	"golang.org/x/time/rate"
)

var defaultLimit = rate.Every(time.Second / 10)

type Queue struct {
	service accrual.Accrualler
	orders  chan storage.Order
	storage *storage.DBStorage
	cancel  context.CancelFunc
	limiter *rate.Limiter
}

func NewQueue(a accrual.Accrualler, st *storage.DBStorage) (queue *Queue) {
	queue = &Queue{
		service: a,
		orders:  make(chan storage.Order),
		storage: st,
		limiter: rate.NewLimiter(defaultLimit, 1),
	}

	return queue
}

func (aq *Queue) Start() {
	qCtx, cancel := context.WithCancel(context.Background())
	aq.cancel = cancel

	orders, err := aq.storage.GetNewOrders(qCtx)
	if err != nil {
		log.Fatalln(err.Error())
	}

	go func() {
		for _, order := range orders {
			aq.orders <- order
		}
	}()

	for {
		select {
		case <-qCtx.Done():
			return
		case order := <-aq.orders:
			aq.limiter.Wait(qCtx)
			go func() {
				delay, err := aq.service.OrderAccrual(qCtx, &order)
				if err != nil {
					log.Println("order #"+order.Number+":", err.Error())
					return
				}

				limit := defaultLimit
				if delay > 0 {
					limit = rate.Every(delay)
				}
				if aq.limiter.Limit() != limit {
					aq.limiter.SetLimit(limit)
				}

				if delay > 0 || order.Status == "REGISTERED" || order.Status == "PROCESSING" {
					aq.orders <- order
					return
				}

				if err = aq.storage.UpdateOrderStatus(qCtx, order); err != nil {
					log.Println("order #"+order.Number+":", err.Error())
				}
			}()
		}
	}
}

func (aq *Queue) Push(order storage.Order) {
	aq.orders <- order
}

func (aq *Queue) Stop() {
	aq.cancel()
}
