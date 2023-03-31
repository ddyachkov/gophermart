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
	authorized.Use(h.Authenticate())
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
		c.JSON(http.StatusBadRequest, gin.H{"message": "wrong request format", "status": http.StatusBadRequest})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "status": http.StatusInternalServerError})
	}

	if err := h.storage.CreateUser(c, u.Login, string(hashedPassword)); err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrLoginUniqueViolation) {
			httpStatusCode = http.StatusConflict
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
		return
	}

	c.Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password)))
	c.JSON(http.StatusOK, gin.H{"message": "user successfully registered and authenticated", "status": http.StatusOK})
}

func (h handler) LogInUser(c *gin.Context) {
	var u user
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "wrong request format", "status": http.StatusBadRequest})
		return
	}

	_, hashedPassword, err := h.storage.GetUserCredentials(c, u.Login)
	if err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrIncorrectUserCredentials) {
			httpStatusCode = http.StatusUnauthorized
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(u.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": storage.ErrIncorrectUserCredentials.Error(), "status": http.StatusUnauthorized})
		return
	}

	c.Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password)))
	c.JSON(http.StatusOK, gin.H{"message": "user successfully logged in", "status": http.StatusOK})
}

func (h handler) PostUserOrder(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error(), "status": http.StatusBadRequest})
		return
	}

	orderNumber := string(body)
	if err = goluhn.Validate(orderNumber); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "wrong order number format", "status": http.StatusUnprocessableEntity})
		return
	}

	userID := c.MustGet("userID").(int)
	err = h.storage.InsertNewOrder(c, orderNumber, userID)
	if err != nil {
		var httpStatusCode int
		switch err {
		case storage.ErrHaveOrderBySameUser:
			httpStatusCode = http.StatusOK
		case storage.ErrHaveOrderByDiffUser:
			httpStatusCode = http.StatusConflict
		default:
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
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
	c.JSON(http.StatusAccepted, gin.H{"message": "new order accepted", "status": http.StatusAccepted})
}

func (h handler) GetUserOrders(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	orders, err := h.storage.GetUserOrders(c, userID)
	if err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrNoOrdersFound) {
			httpStatusCode = http.StatusNoContent
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h handler) GetUserBalance(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	current, withdrawn, err := h.storage.GetUserBalance(c, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error(), "status": http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"current": current, "withdrawn": withdrawn})
}

func (h handler) WithdrawFromUserBalance(c *gin.Context) {
	w := storage.Withdrawal{}
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "wrong request format", "status": http.StatusBadRequest})
		return
	}

	if err := goluhn.Validate(w.OrderNumber); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "wrong order number format", "status": http.StatusUnprocessableEntity})
		return
	}

	userID := c.MustGet("userID").(int)
	if err := h.storage.WithdrawFromUserBalance(c, w.OrderNumber, w.Sum, userID); err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrInsufficientFunds) {
			httpStatusCode = http.StatusPaymentRequired
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "successful withdrawal", "status": http.StatusOK})
}

func (h handler) GetUserWithdrawals(c *gin.Context) {
	userID := c.MustGet("userID").(int)
	withdrawals, err := h.storage.GetUserWithdrawals(c, userID)
	if err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrNoWithdrawalsFound) {
			httpStatusCode = http.StatusNoContent
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error(), "status": httpStatusCode})
		return
	}

	c.JSON(http.StatusOK, withdrawals)
}
