package handlers

import (
	"net/http"
	"unicode"

	"goclone/internal/auth/ldap"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// authRequired provides authentication middleware for ensuring that a user is logged in.
func AuthRequired(c *gin.Context) {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
		return
	}
	c.Next()
}

func GetUser(c *gin.Context) string {
	userID := sessions.Default(c).Get("id")
	if userID != nil {
		return userID.(string)
	}
	return ""
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.JSON(http.StatusOK, gin.H{"message": "No session."})
		return
	}
	session.Delete("id")
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out!"})
}

func validatePassword(password string) bool {
	var number, letter bool
	if len(password) < 8 {
		return false
	}
	for _, c := range password {
		switch {
		case unicode.IsNumber(c):
			number = true
		case unicode.IsLetter(c):
			letter = true
		}
	}

	return number && letter
}

func AdminRequired(c *gin.Context) {
	user := GetUser(c)

	ldapClient := ldap.LdapClient{}
	err := ldapClient.Connect()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}
	defer ldapClient.Disconnect()

	isAdmin, err := ldapClient.IsAdmin(user)
	if !isAdmin {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized."})
		c.Abort()
		return
	}

	c.Next()
}
