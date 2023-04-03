package queue

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/ddyachkov/gophermart/internal/accrual"
	"github.com/ddyachkov/gophermart/internal/storage"
)

var ErrQueueClosed = errors.New("queue is closed")

type Queue struct {
	service accrual.Accrualler
	orders  chan storage.Order
	sync.Mutex
	closed bool
}

func NewQueue(a accrual.Accrualler, orders []storage.Order) (queue *Queue) {
	queue = &Queue{
		service: a,
		orders:  make(chan storage.Order),
	}
	go func() {
		for _, order := range orders {
			queue.orders <- order
		}
	}()

	return queue
}

func (aq *Queue) Start() {
	for {
		aq.Lock()
		if aq.closed {
			break
		}
		aq.Unlock()

		order, ok := <-aq.orders
		if !ok {
			continue
		}
		go func() {
			ready, err := aq.service.OrderAccrual(order)
			if err != nil {
				log.Println("order #"+order.Number+":", err.Error())
			}
			if !ready {
				aq.orders <- order
				time.Sleep(time.Second)
			}
		}()
	}
}

func (aq *Queue) Push(order storage.Order) error {
	aq.Lock()
	defer aq.Unlock()
	if aq.closed {
		return ErrQueueClosed
	}
	aq.orders <- order
	return nil
}

func (aq *Queue) Stop() error {
	aq.Lock()
	if aq.closed {
		return ErrQueueClosed
	}
	aq.closed = true
	close(aq.orders)
	aq.Unlock()

	return nil
}
