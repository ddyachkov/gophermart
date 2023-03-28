package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLoginUniqueViolation     = errors.New("login already in use")
	ErrIncorrectUserCredentials = errors.New("incorrect user credentials")
	ErrHaveOrderBySameUser      = errors.New("order already uploaded by this user")
	ErrHaveOrderByDiffUser      = errors.New("order already uploaded by different user")
	ErrNoOrdersFound            = errors.New("no orders found")
)

type DBStorage struct {
	pool *pgxpool.Pool
}

type Order struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
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

func NewDBStorage(ctx context.Context, p *pgxpool.Pool) (storage *DBStorage, err error) {
	storage = &DBStorage{
		pool: p,
	}

	err = storage.Prepare(ctx)
	if err != nil {
		return storage, err
	}

	return storage, nil
}

func (s DBStorage) Prepare(ctx context.Context) (err error) {
	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.user (id SERIAL PRIMARY KEY, login TEXT UNIQUE NOT NULL, password TEXT NOT NULL)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.order (id SERIAL PRIMARY KEY, number TEXT UNIQUE NOT NULL, uploaded_at timestamp with time zone NOT NULL DEFAULT (current_timestamp), status TEXT NOT NULL, accrual INTEGER NOT NULL DEFAULT 0, user_id INTEGER REFERENCES public.user (id) NOT NULL)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_ord_user_id ON public.order(user_id)")
	if err != nil {
		return err
	}

	return nil
}

func (s DBStorage) CreateUser(ctx context.Context, login string, password string) (err error) {
	_, err = s.pool.Exec(ctx, "INSERT INTO public.user (login, password) VALUES ($1, $2)", login, password)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrLoginUniqueViolation
		}
		return err
	}

	return nil
}

func (s DBStorage) GetUserInfo(ctx context.Context, login string) (id int, password string, err error) {
	err = s.pool.QueryRow(ctx, "SELECT u.id, u.password FROM public.user u WHERE u.login = $1", login).Scan(&id, &password)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, "", ErrIncorrectUserCredentials
		}
		return 0, "", err
	}

	return id, password, nil
}

func (s DBStorage) InsertNewOrder(ctx context.Context, orderNumber string, userID int) (err error) {
	_, err = s.pool.Exec(ctx, "INSERT INTO public.order (number, status, user_id) VALUES ($1, 'NEW', $2)", orderNumber, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			var orderUserID int
			err = s.pool.QueryRow(ctx, "SELECT o.user_id FROM public.order o WHERE o.number = $1", orderNumber).Scan(&orderUserID)
			if err != nil {
				return err
			}
			if userID != orderUserID {
				return ErrHaveOrderByDiffUser
			}
			return ErrHaveOrderBySameUser
		}
		return err
	}
	return nil
}

func (s DBStorage) GetUserOrders(ctx context.Context, userID int) (orders []Order, err error) {
	orders = make([]Order, 0)
	err = pgxscan.Select(ctx, s.pool, &orders, "SELECT o.number, o.status, o.accrual, o.uploaded_at FROM public.order o WHERE o.user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrNoOrdersFound

	}

	return orders, nil
}
