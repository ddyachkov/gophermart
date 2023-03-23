package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ddyachkov/gophermart/internal/config"
	"github.com/ddyachkov/gophermart/internal/random"
	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

var cfg config.ServerConfig = config.ServerConfig{DatabaseURI: "postgres://gophermart:gophermart@127.0.0.1:5432/gophermart"}

func Test_handler_RegisterUser(t *testing.T) {
	type args struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()

	storage, err := storage.NewDBStorage(dbpool, dbCtx)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(storage)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(random.ASCIIString(16, 32)), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	a := args{
		Login:    random.ASCIIString(4, 10),
		Password: string(hashedPassword),
	}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	type want struct {
		code          int
		authorization string
	}
	tests := []struct {
		name string
		body string
		want want
	}{
		{
			name: "Positive_NewUser",
			body: string(body),
			want: want{
				code:          http.StatusOK,
				authorization: "Basic",
			},
		},
		{
			name: "Negative_SameUser",
			body: string(body),
			want: want{
				code:          http.StatusConflict,
				authorization: "",
			},
		},
		{
			name: "Negative_WrongFormat",
			body: "",
			want: want{
				code:          http.StatusBadRequest,
				authorization: "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			bodyReader := strings.NewReader(tt.body)
			r := httptest.NewRequest(http.MethodPost, "/api/user/register", bodyReader)
			handler.ServeHTTP(w, r)
			res := w.Result()

			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Contains(t, res.Header.Get("Authorization"), tt.want.authorization)
		})
	}
}
