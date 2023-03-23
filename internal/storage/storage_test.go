package storage

import (
	"context"
	"testing"
	"time"

	"github.com/ddyachkov/gophermart/internal/config"
	"github.com/ddyachkov/gophermart/internal/random"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

var cfg config.ServerConfig = config.ServerConfig{DatabaseURI: "postgres://gophermart:gophermart@127.0.0.1:5432/gophermart"}

func TestDBStorage_CreateUser(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	storage, err := NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	login := random.ASCIIString(4, 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(random.ASCIIString(16, 32)), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		login    string
		password string
		wantErr  bool
		errType  error
	}{
		{
			name:     "Positive_NewUser",
			login:    login,
			password: string(hashedPassword),
			wantErr:  false,
			errType:  nil,
		},
		{
			name:     "Negative_SameUser",
			login:    login,
			password: string(hashedPassword),
			wantErr:  true,
			errType:  ErrLoginUniqueViolation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			err := storage.CreateUser(ctx, tt.login, tt.password)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}

func TestDBStorage_GetUserPassword(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	storage, err := NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	login := random.ASCIIString(4, 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(random.ASCIIString(16, 32)), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	err = storage.CreateUser(dbCtx, login, string(hashedPassword))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		login   string
		wantErr bool
		errType error
	}{
		{
			name:    "Positive_GotUserPassword",
			login:   login,
			wantErr: false,
			errType: nil,
		},
		{
			name:    "Negative_IncorrectUserCredentials",
			login:   login + "1",
			wantErr: true,
			errType: ErrIncorrectUserCredentials,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			_, err := storage.GetUserPassword(ctx, tt.login)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}
