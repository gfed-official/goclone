package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

// authRequired provides authentication middleware for ensuring that a user is logged in.
func authRequired(c *gin.Context) {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
		return
	}
	c.Next()
}

// getUUID returns a randomly generated UUID
func getUUID() string {
	return uuid.NewString()
}

// initCookies use gin-contrib/sessions{/cookie} to initalize a cookie store.
// It generates a random secret for the cookie store -- not ideal for continuity or invalidating previous cookies, but it's secure and it works
func initCookies(router *gin.Engine) {
	router.Use(sessions.Sessions("kamino", cookie.NewStore([]byte("kamino")))) // change to secret
}

// login is a handler that parses a form and checks for specific data
func login(c *gin.Context) {
	session := sessions.Default(c)
	var jsonData map[string]interface{}
	if err := c.ShouldBindJSON(&jsonData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing fields"})
		return
	}

	username := jsonData["username"].(string)
	password := jsonData["password"].(string)
	// var user models.UserData

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username or password can't be empty."})
		return
	}

	// Log into vSphere to test credentials
	ctx := context.Background()
	u, err := soap.ParseURL(tomlConf.VCenterURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to reach %s.", tomlConf.VCenterURL)})
		return
	}

	u.User = url.UserPassword(username, password)

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect credentials."})
		return
	}
	defer client.Logout(ctx)

	// Save the username in the session
	session.Set("id", username)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to save session."})
		return
	}
	// c.Redirect(http.StatusSeeOther, "/dashboard")
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged in!"})
}

func getUser(c *gin.Context) string {
	userID := sessions.Default(c).Get("id")
	if userID != nil {
		return userID.(string)
	}
	return ""
}

func logout(c *gin.Context) {
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

func register(c *gin.Context) {
	var jsonData map[string]interface{}
	if err := c.ShouldBindJSON(&jsonData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing fields"})
		return
	}

	username := jsonData["username"].(string)
	password := jsonData["password"].(string)

	matched, _ := regexp.MatchString(`^\w{1,16}$`, username)

	if !matched {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Username must not exceed 16 characters and may only contain letters, numbers, or an underscore (_)!"})
		return
	}

	message, err := registerUser(username, password, tomlConf.LdapAdminPassword)

	if err != 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": message})

}
