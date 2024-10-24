package auth

import "github.com/gin-gonic/gin"

type AuthManager interface {
    Login(c *gin.Context)
    RegisterUser(c *gin.Context)
    IsAdmin(c *gin.Context)
}
