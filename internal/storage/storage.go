package storage

import (
	"context"
	"errors"

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
	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.user (id SERIAL PRIMARY KEY, login TEXT UNIQUE NOT NULL, password TEXT NOT NULL)")
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.order (id SERIAL PRIMARY KEY, number BIGINT UNIQUE NOT NULL, uploaded_at timestamp without time zone NOT NULL DEFAULT (current_timestamp AT TIME ZONE 'EETDST'), status TEXT NOT NULL, accrual INTEGER, user_id INTEGER REFERENCES public.user (id) NOT NULL)")
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

func (s DBStorage) InsertNewOrder(ctx context.Context, orderNumber int, userID int) (err error) {
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
