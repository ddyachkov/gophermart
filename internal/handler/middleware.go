package handler

import (
	"errors"
	"net/http"

	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h handler) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		login, password, ok := c.Request.BasicAuth()
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": storage.ErrIncorrectUserCredentials.Error()})
			return
		}

		userID, hashedPassword, err := h.storage.GetUserInfo(c, login)
		if err != nil {
			var httpStatusCode int
			if errors.Is(err, storage.ErrIncorrectUserCredentials) {
				httpStatusCode = http.StatusUnauthorized
			} else {
				httpStatusCode = http.StatusInternalServerError
			}
			c.AbortWithStatusJSON(httpStatusCode, gin.H{"message": err.Error()})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": storage.ErrIncorrectUserCredentials.Error()})
			return
		}
		c.Set("userID", userID)
		c.Next()
	}
}
