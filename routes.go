package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"goclone/models"
)

func addPublicRoutes(g *gin.RouterGroup) {
	g.POST("/login", login)
	g.POST("/register", register)
	g.GET("/health", health)
}

func addPrivateRoutes(g *gin.RouterGroup) {
	g.GET("/logout", logout)

	// user
	g.GET("/view/pods", getPods)

	// system
	g.GET("/view/templates/preset", getPresetTemplates)
	g.GET("/view/templates/custom", getCustomTemplates)

	// clone
	g.POST("/pod/clone/custom", invokePodCloneCustom)
	g.POST("/pod/clone/template", invokePodCloneFromTemplate)
	g.DELETE("/pod/delete/:podId", deletePod)
}

// func pageData(c *gin.Context, title string, ginMap gin.H) gin.H {
// 	newGinMap := gin.H{}
// 	newGinMap["title"] = title
// 	newGinMap["user"] = getUser(c)
// 	newGinMap["config"] = tomlConf
// 	newGinMap["operation"] = tomlConf.Operation
// 	for key, value := range ginMap {
// 		newGinMap[key] = value
// 	}
// 	return newGinMap
// }

func getPresetTemplates(c *gin.Context) {
	templates, err := vSphereGetPresetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrap(err, "Template presets failed to load").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func getCustomTemplates(c *gin.Context) {
	templates, err := vSphereGetCustomTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrap(err, "Custom templates failed to load").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func getPods(c *gin.Context) {
	username := getUser(c)

	pods, err := vSphereGetPods(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pods": pods})
}

func deletePod(c *gin.Context) {
	podId := c.Param("podId")

	err := DestroyResources(podId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully!"})
}

func invokePodCloneCustom(c *gin.Context) {
	username := getUser(c)

	var form models.CustomCloneForm
	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Missing fields").Error()})
		return
	}

	// change for admin
	if len(form.Vmstoclone) > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many VMs in custom pod"})
		return
	}

	err = vSphereCustomClone(form.Name, form.Vmstoclone, form.Nat, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to deploy custom pod").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func invokePodCloneFromTemplate(c *gin.Context) {
	var jsonData map[string]interface{} // cheaty solution to avoid form struct xd
	err := c.ShouldBindJSON(&jsonData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Missing fields").Error()})
		return
	}

	template := jsonData["template"].(string)
	username := getUser(c)

	err = vSphereTemplateClone(template, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to deploy template pod").Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
