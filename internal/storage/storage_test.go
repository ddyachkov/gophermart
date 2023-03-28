package storage

import (
	"context"
	"testing"
	"time"

	"github.com/ShiraazMoollatjie/goluhn"
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
		errType  error
	}{
		{
			name:     "Positive_NewUser",
			login:    login,
			password: string(hashedPassword),
			errType:  nil,
		},
		{
			name:     "Negative_SameUser",
			login:    login,
			password: string(hashedPassword),
			errType:  ErrLoginUniqueViolation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			err := storage.CreateUser(ctx, tt.login, tt.password)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}

func TestDBStorage_GetUserCredentials(t *testing.T) {
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
		errType error
	}{
		{
			name:    "Positive_GotUserPassword",
			login:   login,
			errType: nil,
		},
		{
			name:    "Negative_IncorrectUserCredentials",
			login:   login + "1",
			errType: ErrIncorrectUserCredentials,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			_, _, err := storage.GetUserCredentials(ctx, tt.login)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}

func TestDBStorage_InsertNewOrder(t *testing.T) {
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

	userID, _, err := storage.GetUserCredentials(dbCtx, login)
	if err != nil {
		t.Fatal(err)
	}

	orderNumber := goluhn.Generate(8)

	tests := []struct {
		name        string
		orderNumber string
		userID      int
		errType     error
	}{
		{
			name:        "Positive_NewOrder",
			orderNumber: orderNumber,
			userID:      userID,
			errType:     nil,
		},
		{
			name:        "Negative_SameOrder_SameUser",
			orderNumber: orderNumber,
			userID:      userID,
			errType:     ErrHaveOrderBySameUser,
		},
		{
			name:        "Negative_SameOrder_DiffUser",
			orderNumber: orderNumber,
			userID:      userID - 1,
			errType:     ErrHaveOrderByDiffUser,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			err := storage.InsertNewOrder(ctx, tt.orderNumber, tt.userID)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}

func TestDBStorage_GetUserOrders(t *testing.T) {
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

	userID, _, err := storage.GetUserCredentials(dbCtx, login)
	if err != nil {
		t.Fatal(err)
	}

	orderNumber := goluhn.Generate(8)
	err = storage.InsertNewOrder(dbCtx, orderNumber, userID)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		orderNumber string
		userID      int
		errType     error
	}{
		{
			name:        "Positive_FoundOrder",
			orderNumber: orderNumber,
			userID:      userID,
			errType:     nil,
		},
		{
			name:        "Negative_NoOrdersFound",
			orderNumber: "",
			userID:      userID + 1,
			errType:     ErrNoOrdersFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			gotOrders, err := storage.GetUserOrders(ctx, tt.userID)
			var gotOrderNumber string
			if len(gotOrders) > 0 {
				gotOrderNumber = gotOrders[0].Number
			}
			assert.Equal(t, tt.orderNumber, gotOrderNumber)
			assert.Equal(t, tt.errType, err)
		})
	}
}

func TestDBStorage_GetUserBalance(t *testing.T) {
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

	userID, _, err := storage.GetUserCredentials(dbCtx, login)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		userID  int
		wantErr bool
	}{
		{
			name:    "Positive_FoundBalance",
			userID:  userID,
			wantErr: false,
		},
		{
			name:    "Negative_NotFoundBalance",
			userID:  userID + 1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			_, _, err := storage.GetUserBalance(ctx, tt.userID)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
