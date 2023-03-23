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
	type args struct {
		login    string
		password string
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()

	storage, err := NewDBStorage(dbpool, dbCtx)
	if err != nil {
		t.Fatal(err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(random.ASCIIString(16, 32)), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	a := args{
		login:    random.ASCIIString(4, 10),
		password: string(hashedPassword),
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		errType error
	}{
		{
			name:    "Positive_NewUser",
			args:    a,
			wantErr: false,
			errType: nil,
		},
		{
			name:    "Negative_SameUser",
			args:    a,
			wantErr: true,
			errType: ErrLoginUniqueViolation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			err := storage.CreateUser(ctx, tt.args.login, tt.args.password)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.ErrorIs(t, tt.errType, err)
		})
	}
}
