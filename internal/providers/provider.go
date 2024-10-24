package providers

import "github.com/gin-gonic/gin"

type Provider interface {
    GetPods(c *gin.Context)
    DeletePod(c *gin.Context)
    GetPresetTemplates(c *gin.Context)
    GetTemplateVMs(c *gin.Context)
    CloneFromTemplate(c *gin.Context)
    CloneCustomPod(c *gin.Context)
}
