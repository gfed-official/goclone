package providers

import "github.com/gin-gonic/gin"

type Provider interface {
    GetPodsHandler(c *gin.Context)
    DeletePodHandler(c *gin.Context)

    GetPresetTemplatesHandler(c *gin.Context)
    GetTemplateVMsHandler(c *gin.Context)

    CloneFromTemplateHandler(c *gin.Context)
    CloneCustomPodHandler(c *gin.Context)

    RefreshTemplatesHandler(c *gin.Context)
    BulkClonePodsHandler(c *gin.Context)
    BulkDeletePodsHandler(c *gin.Context)
    BulkRevertPodHandler(c *gin.Context)
    BulkPowerPodHandler(c *gin.Context)
}
