package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLoginUniqueViolation = errors.New("login already in use")

type DBStorage struct {
	pool *pgxpool.Pool
}

func NewDBStorage(p *pgxpool.Pool, ctx context.Context) (storage *DBStorage, err error) {
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

	return nil
}

func (s DBStorage) CreateUser(ctx context.Context, login string, password string) (err error) {
	_, err = s.pool.Exec(ctx, "INSERT INTO public.user (login, password) VALUES ($1, $2) RETURNING id", login, password)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrLoginUniqueViolation
		}
		return err
	}

	return nil
}
