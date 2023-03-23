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
	"github.com/stretchr/testify/require"
)

var cfg config.ServerConfig = config.ServerConfig{DatabaseURI: "postgres://gophermart:gophermart@127.0.0.1:5432/gophermart"}

func Test_handler_RegisterUser(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	storage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(storage)

	u := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	body, err := json.Marshal(u)
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

func Test_handler_LogInUser(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	storage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(storage)

	registeredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	ruBody, err := json.Marshal(registeredUser)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	bodyReader := strings.NewReader(string(ruBody))
	r := httptest.NewRequest(http.MethodPost, "/api/user/register", bodyReader)
	handler.ServeHTTP(w, r)
	res := w.Result()
	require.Equal(t, http.StatusOK, res.StatusCode)

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	uuBody, err := json.Marshal(unregisteredUser)
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
			name: "Positive_UserLoggedIn",
			body: string(ruBody),
			want: want{
				code:          http.StatusOK,
				authorization: "Basic",
			},
		},
		{
			name: "Negative_IncorrectUserCredentials",
			body: string(uuBody),
			want: want{
				code:          http.StatusUnauthorized,
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
			r := httptest.NewRequest(http.MethodPost, "/api/user/login", bodyReader)
			handler.ServeHTTP(w, r)
			res := w.Result()

			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Contains(t, res.Header.Get("Authorization"), tt.want.authorization)
		})
	}
}
