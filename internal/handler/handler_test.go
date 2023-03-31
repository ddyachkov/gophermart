package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/ddyachkov/gophermart/internal/accrual"
	"github.com/ddyachkov/gophermart/internal/config"
	"github.com/ddyachkov/gophermart/internal/queue"
	"github.com/ddyachkov/gophermart/internal/random"
	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cfg config.ServerConfig = config.ServerConfig{DatabaseURI: "postgres://gophermart:gophermart@127.0.0.1:5432/gophermart"}

func sendRequest(handler http.Handler, body string, method string, path string, user user) *http.Response {
	w := httptest.NewRecorder()
	var r *http.Request
	if body != "" {
		bodyReader := strings.NewReader(body)
		r = httptest.NewRequest(method, path, bodyReader)
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if user.Login != "" {
		r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user.Login+":"+user.Password)))
	}
	handler.ServeHTTP(w, r)
	return w.Result()
}

func Test_handler_RegisterUser(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(dbStorage, nil)

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
			res := sendRequest(handler, tt.body, http.MethodPost, "/api/user/register", user{})
			defer res.Body.Close()
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

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(dbStorage, nil)

	registeredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	ruBody, err := json.Marshal(registeredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(ruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
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
			res := sendRequest(handler, tt.body, http.MethodPost, "/api/user/login", user{})
			defer res.Body.Close()
			assert.Equal(t, tt.want.code, res.StatusCode)
			assert.Contains(t, res.Header.Get("Authorization"), tt.want.authorization)
		})
	}
}

func Test_handler_PostUserOrder(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(dbStorage, nil)

	firstRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	fruBody, err := json.Marshal(firstRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(fruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	secondRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	sruBody, err := json.Marshal(secondRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res = sendRequest(handler, string(sruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}

	orderNumber := goluhn.Generate(8)

	tests := []struct {
		name string
		user user
		body string
		code int
	}{
		{
			name: "Positive_NewOrder",
			user: firstRegisteredUser,
			body: orderNumber,
			code: http.StatusAccepted,
		},
		{
			name: "Positive_SameOrder_SameUser",
			user: firstRegisteredUser,
			body: orderNumber,
			code: http.StatusOK,
		},
		{
			name: "Negative_SameOrder_DiffUser",
			user: secondRegisteredUser,
			body: orderNumber,
			code: http.StatusConflict,
		},
		{
			name: "Negative_Unauthorized",
			user: unregisteredUser,
			body: orderNumber,
			code: http.StatusUnauthorized,
		},
		{
			name: "Negative_WrongOrderNumber",
			user: firstRegisteredUser,
			body: orderNumber + "f",
			code: http.StatusUnprocessableEntity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sendRequest(handler, tt.body, http.MethodPost, "/api/user/orders", tt.user)
			defer res.Body.Close()
			assert.Equal(t, tt.code, res.StatusCode)
		})
	}
}

func Test_handler_GetUserOrders(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(dbStorage, nil)

	firstRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	fruBody, err := json.Marshal(firstRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(fruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	orderNumber := goluhn.Generate(8)
	res = sendRequest(handler, orderNumber, http.MethodPost, "/api/user/orders", firstRegisteredUser)
	defer res.Body.Close()
	require.Equal(t, http.StatusAccepted, res.StatusCode)

	secondRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	sruBody, err := json.Marshal(secondRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res = sendRequest(handler, string(sruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}

	tests := []struct {
		name string
		user user
		code int
	}{
		{
			name: "Positive_FoundOrder",
			user: firstRegisteredUser,
			code: http.StatusOK,
		},
		{
			name: "Negative_NoOrders",
			user: secondRegisteredUser,
			code: http.StatusNoContent,
		},
		{
			name: "Negative_Unauthorized",
			user: unregisteredUser,
			code: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sendRequest(handler, "", http.MethodGet, "/api/user/orders", tt.user)
			defer res.Body.Close()
			assert.Equal(t, tt.code, res.StatusCode)
		})
	}
}

func Test_handler_GetUserBalance(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(dbStorage, nil)

	registeredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	ruBody, err := json.Marshal(registeredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(ruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}

	tests := []struct {
		name string
		user user
		code int
	}{
		{
			name: "Positive_FoundOrder",
			user: registeredUser,
			code: http.StatusOK,
		},
		{
			name: "Negative_Unauthorized",
			user: unregisteredUser,
			code: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sendRequest(handler, "", http.MethodGet, "/api/user/balance", tt.user)
			defer res.Body.Close()
			assert.Equal(t, tt.code, res.StatusCode)
		})
	}
}

func Test_handler_WithdrawFromUserBalance(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	accrualler := accrual.NewMockService(dbStorage)
	queue := queue.NewQueue(accrualler, nil)
	handler := NewHandler(dbStorage, queue)
	go queue.Start()
	defer queue.Stop()

	registeredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	ruBody, err := json.Marshal(registeredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(ruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	orderNumber := goluhn.Generate(8)
	res = sendRequest(handler, orderNumber, http.MethodPost, "/api/user/orders", registeredUser)
	defer res.Body.Close()
	require.Equal(t, http.StatusAccepted, res.StatusCode)

	res = sendRequest(handler, orderNumber, http.MethodGet, "/api/user/balance", registeredUser)
	require.Equal(t, http.StatusOK, res.StatusCode)
	balance := struct {
		Current   float32
		Withdrawn float32
	}{}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(resBody, &balance)

	correctWithdrawal := storage.Withdrawal{
		OrderNumber: orderNumber,
		Sum:         balance.Current,
	}
	cwBody, err := json.Marshal(correctWithdrawal)
	if err != nil {
		t.Fatal(err)
	}

	incorrectWithdrawal := storage.Withdrawal{
		OrderNumber: orderNumber + "a",
		Sum:         balance.Current,
	}
	iwBody, err := json.Marshal(incorrectWithdrawal)
	if err != nil {
		t.Fatal(err)
	}

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}

	tests := []struct {
		name       string
		user       user
		withdrawal string
		code       int
	}{
		{
			name:       "Positive_SuccessfulWithdrawal",
			user:       registeredUser,
			withdrawal: string(cwBody),
			code:       http.StatusOK,
		},
		{
			name:       "Negative_InsufficientFunds",
			user:       registeredUser,
			withdrawal: string(cwBody),
			code:       http.StatusPaymentRequired,
		},
		{
			name:       "Negative_WrongOrderNumber",
			user:       registeredUser,
			withdrawal: string(iwBody),
			code:       http.StatusUnprocessableEntity,
		},
		{
			name:       "Negative_Unauthorized",
			user:       unregisteredUser,
			withdrawal: string(cwBody),
			code:       http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sendRequest(handler, tt.withdrawal, http.MethodPost, "/api/user/balance/withdraw", tt.user)
			defer res.Body.Close()
			assert.Equal(t, tt.code, res.StatusCode)
		})
	}
}

func Test_handler_GetUserWithdrawals(t *testing.T) {
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		t.Fatal(err)
	}
	defer dbPool.Close()

	dbStorage, err := storage.NewDBStorage(dbCtx, dbPool)
	if err != nil {
		t.Fatal(err)
	}

	accrualler := accrual.NewMockService(dbStorage)
	queue := queue.NewQueue(accrualler, nil)
	handler := NewHandler(dbStorage, queue)
	go queue.Start()
	defer queue.Stop()

	firstRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	fruBody, err := json.Marshal(firstRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res := sendRequest(handler, string(fruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	secondRegisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}
	sruBody, err := json.Marshal(secondRegisteredUser)
	if err != nil {
		t.Fatal(err)
	}
	res = sendRequest(handler, string(sruBody), http.MethodPost, "/api/user/register", user{})
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	unregisteredUser := user{
		Login:    random.ASCIIString(4, 10),
		Password: random.ASCIIString(16, 32),
	}

	orderNumber := goluhn.Generate(8)
	res = sendRequest(handler, orderNumber, http.MethodPost, "/api/user/orders", firstRegisteredUser)
	defer res.Body.Close()
	require.Equal(t, http.StatusAccepted, res.StatusCode)

	res = sendRequest(handler, orderNumber, http.MethodGet, "/api/user/balance", firstRegisteredUser)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	balance := struct {
		Current   float32
		Withdrawn float32
	}{}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(resBody, &balance)

	withdrawal := storage.Withdrawal{
		OrderNumber: orderNumber,
		Sum:         balance.Current,
	}
	wBody, err := json.Marshal(withdrawal)
	if err != nil {
		t.Fatal(err)
	}
	res = sendRequest(handler, string(wBody), http.MethodPost, "/api/user/balance/withdraw", firstRegisteredUser)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	tests := []struct {
		name string
		user user
		code int
	}{
		{
			name: "Positive_FoundWithdrawals",
			user: firstRegisteredUser,
			code: http.StatusOK,
		},
		{
			name: "Negative_NoWithdrawals",
			user: secondRegisteredUser,
			code: http.StatusNoContent,
		},
		{
			name: "Negative_Unauthorized",
			user: unregisteredUser,
			code: http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sendRequest(handler, "", http.MethodGet, "/api/user/withdrawals", tt.user)
			defer res.Body.Close()
			assert.Equal(t, tt.code, res.StatusCode)
		})
	}
}
