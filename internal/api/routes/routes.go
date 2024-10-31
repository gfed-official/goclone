package routes

import (
    "goclone/internal/api/handlers"
    "goclone/internal/auth"
    "goclone/internal/providers"

    "github.com/gin-gonic/gin"
)

func AddRoutes(router *gin.Engine, authManager auth.AuthManager, virtProvider providers.Provider) {
    public := router.Group("/api/v2")
    addPublicRoutes(public, authManager)

    private := router.Group("/api/v2")
    private.Use(handlers.AuthRequired)
    addPrivateRoutes(private, virtProvider)

    admin := router.Group("/api/v2/admin")
    admin.Use(handlers.AuthRequired, authManager.IsAdmin)
    addAdminRoutes(admin, virtProvider)
}

func addPublicRoutes(g *gin.RouterGroup, authManager auth.AuthManager) {
    g.POST("/login", authManager.Login)
    g.POST("/register", authManager.RegisterUser)
    g.GET("/health", handlers.HealthCheck)
}

func addPrivateRoutes(g *gin.RouterGroup, virtProvider providers.Provider) {
    g.GET("/logout", handlers.Logout)

    g.GET("/view/pods", virtProvider.GetPodsHandler)

    // system
    g.GET("/view/templates/preset", virtProvider.GetPresetTemplatesHandler)
    g.GET("/view/templates/custom", virtProvider.GetTemplateVMsHandler)

    // clone
    g.POST("/pod/clone/custom", virtProvider.CloneCustomPodHandler)
    g.POST("/pod/clone/template", virtProvider.CloneFromTemplateHandler)
    g.DELETE("/pod/delete/:podId", virtProvider.DeletePodHandler)
}

func addAdminRoutes(g *gin.RouterGroup, virtProvider providers.Provider) {
    g.GET("/view/pods", virtProvider.GetPodsHandler)
}

