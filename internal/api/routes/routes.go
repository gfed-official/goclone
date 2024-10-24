package routes

import (
	"goclone/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

func AddRoutes(router *gin.Engine) {
	public := router.Group("/api/v1")
	addPublicRoutes(public)

	private := router.Group("/api/v1")
	private.Use(handlers.AuthRequired)
	addPrivateRoutes(private)

	admin := router.Group("/api/v1/admin")
	admin.Use(handlers.AuthRequired, handlers.AdminRequired)
	addAdminRoutes(admin)
}

func addPublicRoutes(g *gin.RouterGroup) {
	g.GET("/health", handlers.HealthCheck)
}

func addPrivateRoutes(g *gin.RouterGroup) {
	g.GET("/logout", handlers.Logout)
}

func addAdminRoutes(g *gin.RouterGroup) {
}
