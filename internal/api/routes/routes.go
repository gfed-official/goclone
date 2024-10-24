package routes

import (
	"goclone/internal/api/handlers"
	"goclone/internal/auth"

	"github.com/gin-gonic/gin"
)

func AddRoutes(router *gin.Engine, authManager auth.AuthManager) {
	public := router.Group("/api/v2")
	addPublicRoutes(public, authManager)

	private := router.Group("/api/v2")
	private.Use(handlers.AuthRequired)
	addPrivateRoutes(private)

	admin := router.Group("/api/v2/admin")
	admin.Use(handlers.AuthRequired, authManager.IsAdmin)
	addAdminRoutes(admin)
}

func addPublicRoutes(g *gin.RouterGroup, authManager auth.AuthManager, virtManager ) {
    g.POST("/login", authManager.Login)
	g.POST("/register", authManager.RegisterUser)
	g.GET("/health", handlers.HealthCheck)
}

func addPrivateRoutes(g *gin.RouterGroup) {
	g.GET("/logout", handlers.Logout)

    g.GET("/pods", handlers.GetPods)
}

func addAdminRoutes(g *gin.RouterGroup) {
}

