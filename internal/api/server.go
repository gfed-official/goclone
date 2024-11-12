package api

import (
	"fmt"
	"io"
	"log"
	"os"

	"goclone/internal/api/routes"
	"goclone/internal/auth"
	"goclone/internal/auth/ldap"
	"goclone/internal/config"
	"goclone/internal/providers"
	"goclone/internal/providers/vsphere"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func StartAPI(conf *config.Config) {
    fmt.Println(&conf)

	//setup logging
	gin.SetMode(gin.ReleaseMode)

	f, err := os.OpenFile(conf.Core.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to open log file"))
	}
	defer f.Close()

	log.SetOutput(f)
	gin.DefaultWriter = io.MultiWriter(f)

    // Setup Auth
    authManager := SetupAuthManager(conf)
    if authManager == nil {
        panic("No Auth Provider Enabled")
    }

    // Setup Virtualization Provider
    virtProvider := SetupVirtProvider(conf, &authManager)
    if virtProvider == nil {
        panic("No Virtualization Provider Enabled")
    }

    fmt.Println("API Starting")
	// setup router
	router := gin.Default()
	router.Use(CORSMiddleware(conf.Core.ExternalURL))
	router.MaxMultipartMemory = 8 << 20 // 8Mib
	initCookies(router)

	// add routes
	routes.AddRoutes(router, authManager, virtProvider)

	log.Fatalln(router.Run(conf.Core.ListeningAddress))
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

func SetupAuthManager(conf *config.Config) auth.AuthManager {
    var authManager auth.AuthManager
    if (conf.Auth.Ldap != config.LdapProvider{}) {
        authManager = ldap.NewLdapManager(conf.Auth.Ldap)
        fmt.Println("LDAP Auth Enabled")
    }
    return authManager
}

func SetupVirtProvider(conf *config.Config, authManager *auth.AuthManager) providers.Provider {
    var virtProvider providers.Provider
    if (conf.VirtProvider.VCenter != config.VCenter{}) {
        virtProvider = vsphere.NewVSphereProvider(conf, authManager)
        fmt.Println("vSphere Provider Enabled")
    }
    return virtProvider
}
