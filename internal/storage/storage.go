package storage

import (
	"context"
	"errors"
	"log"

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
	ErrInsufficientFunds        = errors.New("insufficient funds on the user balance")
	ErrNoWithdrawalsFound       = errors.New("no withdrawals found")
)

type DBStorage struct {
	pool *pgxpool.Pool
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
	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.user (id SERIAL PRIMARY KEY, login TEXT UNIQUE NOT NULL, password TEXT NOT NULL, current REAL NOT NULL DEFAULT 0 CHECK (current >= 0), withdrawn REAL NOT NULL DEFAULT 0)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.order (id SERIAL PRIMARY KEY, number TEXT UNIQUE NOT NULL, uploaded_at timestamp with time zone NOT NULL DEFAULT (current_timestamp), status TEXT NOT NULL, accrual REAL NOT NULL DEFAULT 0, user_id INTEGER REFERENCES public.user (id) NOT NULL)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_ord_user_id ON public.order(user_id)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_ord_status_new ON public.order(status) where status = 'NEW'")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.withdrawal (id SERIAL PRIMARY KEY, order_number TEXT NOT NULL, sum REAL NOT NULL DEFAULT 0, processed_at timestamp with time zone NOT NULL DEFAULT (current_timestamp), user_id INTEGER REFERENCES public.user (id) NOT NULL)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_wd_user_id ON public.withdrawal(user_id)")
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

func (s DBStorage) GetUserCredentials(ctx context.Context, login string) (id int, password string, err error) {
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
	err = pgxscan.Select(ctx, s.pool, &orders, "SELECT o.number, o.status, o.accrual, o.uploaded_at FROM public.order o WHERE o.user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, ErrNoOrdersFound
	}

	return orders, nil
}

func (s DBStorage) GetUserBalance(ctx context.Context, userID int) (current float32, withdrawn float32, err error) {
	err = s.pool.QueryRow(ctx, "SELECT u.current, u.withdrawn FROM public.user u WHERE u.id = $1", userID).Scan(&current, &withdrawn)
	if err != nil {
		return 0, 0, err
	}

	return current, withdrawn, nil
}

func (s DBStorage) WithdrawFromUserBalance(ctx context.Context, orderNumber string, sum float32, userID int) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE public.user SET current = current - $1, withdrawn = withdrawn + $1 WHERE id = $2", sum, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.CheckViolation {
			return ErrInsufficientFunds
		}
		return err
	}

	_, err = tx.Exec(ctx, "INSERT INTO public.withdrawal (order_number, sum, user_id) VALUES ($1, $2, $3)", orderNumber, sum, userID)
	if err != nil {
		return err
	}

	tx.Commit(ctx)
	return nil
}

func (s DBStorage) GetUserWithdrawals(ctx context.Context, userID int) (withdrawals []Withdrawal, err error) {
	err = pgxscan.Select(ctx, s.pool, &withdrawals, "SELECT wd.order_number, wd.sum, wd.processed_at FROM public.withdrawal wd WHERE wd.user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	if len(withdrawals) == 0 {
		return nil, ErrNoWithdrawalsFound
	}

	return withdrawals, nil
}

func (s DBStorage) UpdateOrderStatus(ctx context.Context, order Order) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE public.order SET status = $1, accrual = $2 WHERE number = $3", order.Status, order.Accrual, order.Number)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, "UPDATE public.user SET current = current + $1 WHERE id = $2", order.Accrual, order.UserID)
	if err != nil {
		return err
	}

	tx.Commit(ctx)
	return nil
}

func (s DBStorage) GetNewOrders(ctx context.Context) (orders []Order, err error) {
	err = pgxscan.Select(ctx, s.pool, &orders, "SELECT o.number, o.user_id FROM public.order o WHERE o.status = 'NEW'")
	if err != nil {
		return nil, err
	}
	log.Println("got orders")
	return orders, nil
}
