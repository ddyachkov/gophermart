package storage

import (
	"encoding/json"
	"time"
)

type Order struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float32   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"-" db:"uploaded_at"`
}

func (o Order) MarshalJSON() ([]byte, error) {
	type OrderAlias Order

	aliasValue := struct {
		OrderAlias
		UploadedAtRFC3339 string `json:"uploaded_at"`
	}{
		OrderAlias:        OrderAlias(o),
		UploadedAtRFC3339: o.UploadedAt.Format(time.RFC3339),
	}

	return json.Marshal(aliasValue)
}

type Withdrawal struct {
	OrderNumber string    `json:"order" db:"order_number"`
	Sum         float32   `json:"sum"`
	ProcessedAt time.Time `json:"-" db:"processed_at"`
}

func (w Withdrawal) MarshalJSON() ([]byte, error) {
	type WithdrawalAlias Withdrawal

	aliasValue := struct {
		WithdrawalAlias
		ProcessedAtRFC3339 string `json:"processed_at"`
	}{
		WithdrawalAlias:    WithdrawalAlias(w),
		ProcessedAtRFC3339: w.ProcessedAt.Format(time.RFC3339),
	}

	return json.Marshal(aliasValue)
}
