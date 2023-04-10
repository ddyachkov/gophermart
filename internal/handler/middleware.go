package handler

import (
	"errors"
	"net/http"

	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h handler) Authenticate(c *gin.Context) {
	login, password, ok := c.Request.BasicAuth()
	if !ok {
		message := gin.H{
			"message": storage.ErrIncorrectUserCredentials.Error(),
			"status":  http.StatusUnauthorized,
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, message)
		return
	}

	userID, hashedPassword, err := h.storage.GetUserCredentials(c, login)
	if err != nil {
		httpStatusCode := http.StatusInternalServerError
		if errors.Is(err, storage.ErrIncorrectUserCredentials) {
			httpStatusCode = http.StatusUnauthorized
		}
		message := gin.H{
			"message": err.Error(),
			"status":  httpStatusCode,
		}
		c.AbortWithStatusJSON(httpStatusCode, message)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		message := gin.H{
			"message": storage.ErrIncorrectUserCredentials.Error(),
			"status":  http.StatusUnauthorized,
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, message)
		return
	}
	c.Set("userID", userID)
	c.Next()
}
