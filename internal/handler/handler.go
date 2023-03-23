package handler

import (
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/ddyachkov/gophermart/internal/middleware"
	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type handler struct {
	storage *storage.DBStorage
}

type user struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func NewHandler(s *storage.DBStorage) http.Handler {
	router := gin.Default()

	h := handler{
		storage: s,
	}

	router.Use(middleware.Decompress(), gzip.Gzip(gzip.DefaultCompression))
	router.POST("/api/user/register", h.RegisterUser)
	router.POST("/api/user/login", h.LogInUser)

	return router
}

func (h handler) RegisterUser(c *gin.Context) {
	var u user
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "wrong request format"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	if err := h.storage.CreateUser(c, u.Login, string(hashedPassword)); err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrLoginUniqueViolation) {
			httpStatusCode = http.StatusConflict
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error()})
		return
	}

	c.Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password)))
	c.JSON(http.StatusOK, gin.H{"message": "user successfully registered and authenticated"})
}

func (h handler) LogInUser(c *gin.Context) {
	var u user
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "wrong request format"})
		return
	}

	hashedPassword, err := h.storage.GetUserPassword(c, u.Login)
	if err != nil {
		var httpStatusCode int
		if errors.Is(err, storage.ErrIncorrectUserCredentials) {
			httpStatusCode = http.StatusUnauthorized
		} else {
			httpStatusCode = http.StatusInternalServerError
		}
		c.JSON(httpStatusCode, gin.H{"message": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(u.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": storage.ErrIncorrectUserCredentials.Error()})
		return
	}

	c.Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u.Login+":"+u.Password)))
	c.JSON(http.StatusOK, gin.H{"message": "user successfully logged in"})
}
