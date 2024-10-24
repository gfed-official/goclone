package api

import (
	"fmt"
	"io"
	"log"
	"os"

	"goclone/internal/api/routes"
	"goclone/internal/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func StartAPI(config *config.Config) {
	//setup logging
	gin.SetMode(gin.ReleaseMode)

	f, err := os.OpenFile(config.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to open log file"))
	}
	defer f.Close()

	log.SetOutput(f)
	gin.DefaultWriter = io.MultiWriter(f)

	// setup router
	router := gin.Default()
	router.Use(CORSMiddleware(config.Fqdn))
	router.MaxMultipartMemory = 8 << 20 // 8Mib
	initCookies(router)

	// add routes
	routes.AddRoutes(router)

	log.Fatalln(router.Run(":" + fmt.Sprint(config.Port)))
}

func CORSMiddleware(fqdn string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", fqdn)
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		}

		c.Next()
	}
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
