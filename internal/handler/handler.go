package handler

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"

	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/ddyachkov/gophermart/internal/middleware"
	"github.com/ddyachkov/gophermart/internal/queue"
	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type handler struct {
	storage *storage.DBStorage
	queue   *queue.Queue
}

type user struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func NewHandler(s *storage.DBStorage, q *queue.Queue) http.Handler {
	router := gin.Default()

	h := handler{
		storage: s,
		queue:   q,
	}

	router.Use(middleware.Decompress(), gzip.Gzip(gzip.DefaultCompression))
	router.POST("/api/user/register", h.RegisterUser)
	router.POST("/api/user/login", h.LogInUser)

	authorized := router.Group("/")
	authorized.Use(h.Authenticate)
	{
		authorized.POST("/api/user/orders", h.PostUserOrder)
		authorized.GET("/api/user/orders", h.GetUserOrders)
		authorized.GET("/api/user/balance", h.GetUserBalance)
		authorized.POST("/api/user/balance/withdraw", h.WithdrawFromUserBalance)
		authorized.GET("/api/user/withdrawals", h.GetUserWithdrawals)
	}

	return router
}

func (h handler) RegisterUser(c *gin.Context) {
	var u user
	if err := c.ShouldBindJSON(&u); err != nil {
		message := gin.H{
			"message": "wrong request format",
			"status":  http.StatusBadRequest,
		}
		c.JSON(http.StatusBadRequest, message)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		message := gin.H{
			"message": err.Error(),
			"status":  http.StatusInternalServerError,
		}
		c.JSON(http.StatusInternalServerError, message)
	}

	if err := h.storage.CreateUser(c, u.Login, string(hashedPassword)); err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrLoginUniqueViolation) {
			httpStatusCode = http.StatusConflict
		}

		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password))
	c.Header("Authorization", authorization)
	message := gin.H{
		"message": "user successfully registered and authenticated",
		"status":  http.StatusOK,
	}
	c.JSON(http.StatusOK, message)
}

func (h handler) LogInUser(c *gin.Context) {
	var u user
	if err := c.ShouldBindJSON(&u); err != nil {
		message := gin.H{
			"message": "user successfully registered and authenticated",
			"status":  http.StatusBadRequest,
		}
		c.JSON(http.StatusBadRequest, message)
		return
	}

	_, hashedPassword, err := h.storage.GetUserCredentials(c, u.Login)
	if err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrIncorrectUserCredentials) {
			httpStatusCode = http.StatusUnauthorized
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(u.Password)); err != nil {
		message := gin.H{
			"message": storage.ErrIncorrectUserCredentials.Error(),
			"status":  http.StatusUnauthorized,
		}
		c.JSON(http.StatusUnauthorized, message)
		return
	}

	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password))
	c.Header("Authorization", authorization)
	message := gin.H{
		"message": "user successfully logged in",
		"status":  http.StatusOK,
	}
	c.JSON(http.StatusOK, message)
}

func (h handler) PostUserOrder(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		message := gin.H{
			"message": err.Error(),
			"status":  http.StatusBadRequest,
		}
		c.JSON(http.StatusBadRequest, message)
		return
	}

	orderNumber := string(body)
	if err = goluhn.Validate(orderNumber); err != nil {
		message := gin.H{
			"message": "wrong order number format",
			"status":  http.StatusUnprocessableEntity,
		}
		c.JSON(http.StatusUnprocessableEntity, message)
		return
	}

	userID := c.MustGet("userID").(int)
	err = h.storage.InsertNewOrder(c, orderNumber, userID)
	if err != nil {
		httpStatusCode := http.StatusInternalServerError
		switch err {
		case storage.ErrHaveOrderBySameUser:
			httpStatusCode = http.StatusOK
		case storage.ErrHaveOrderByDiffUser:
			httpStatusCode = http.StatusConflict
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	go func() {
		order := storage.Order{
			Number: orderNumber,
			Status: "NEW",
			UserID: userID,
		}
		h.queue.Push(order)
	}()

	message := gin.H{
		"message": "new order accepted",
		"status":  http.StatusAccepted,
	}
	c.JSON(http.StatusAccepted, message)
}

func (h handler) GetUserOrders(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	orders, err := h.storage.GetUserOrders(c, userID)
	if err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNoOrdersFound) {
			httpStatusCode = http.StatusNoContent
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h handler) GetUserBalance(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	current, withdrawn, err := h.storage.GetUserBalance(c, userID)
	if err != nil {
		message := gin.H{
			"message": err.Error(),
			"status":  http.StatusInternalServerError,
		}
		c.JSON(http.StatusInternalServerError, message)
		return
	}
	message := gin.H{
		"current":   current,
		"withdrawn": withdrawn,
	}
	c.JSON(http.StatusOK, message)
}

func (h handler) WithdrawFromUserBalance(c *gin.Context) {
	w := storage.Withdrawal{}
	if err := c.ShouldBindJSON(&w); err != nil {
		message := gin.H{
			"message": "wrong request format",
			"status":  http.StatusBadRequest,
		}
		c.JSON(http.StatusBadRequest, message)
		return
	}

	if err := goluhn.Validate(w.OrderNumber); err != nil {
		message := gin.H{
			"message": "wrong order number format",
			"status":  http.StatusUnprocessableEntity,
		}
		c.JSON(http.StatusUnprocessableEntity, message)
		return
	}

	userID := c.MustGet("userID").(int)
	if err := h.storage.WithdrawFromUserBalance(c, w.OrderNumber, w.Sum, userID); err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrInsufficientFunds) {
			httpStatusCode = http.StatusPaymentRequired
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	message := gin.H{
		"message": "successful withdrawal",
		"status":  http.StatusOK,
	}
	c.JSON(http.StatusOK, message)
}

func (h handler) GetUserWithdrawals(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	withdrawals, err := h.storage.GetUserWithdrawals(c, userID)
	if err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrNoWithdrawalsFound) {
			httpStatusCode = http.StatusNoContent
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.JSON(httpStatusCode, message)
		return
	}

	c.JSON(http.StatusOK, withdrawals)
}
