package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/vim25/soap"
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

func addAdminRoutes(g *gin.RouterGroup) {
	g.GET("/view/pods", adminGetAllPods)
	g.POST("/pod/clone/bulk", adminBulkClonePods)
	g.DELETE("/pod/delete/:podId", adminDeletePod)
	g.POST("/templates/refresh", refreshTemplates)
    g.POST("/pod/delete/bulk", adminBulkDeletePods)
}

func getPresetTemplates(c *gin.Context) {
	user := getUser(c)
	templates, err := vSphereGetPresetTemplates(user)
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

	username := getUser(c)
	podOwner := strings.Split(podId, "_")
	podOwner = podOwner[len(podOwner)-1:]
	if strings.ToLower(podOwner[0]) != strings.ToLower(username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can only delete your own pods"})
		return
	}

	err := DestroyResources(podId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully!"})
}

func invokePodCloneCustom(c *gin.Context) {
	username := getUser(c)

	var form struct {
		Name       string   `json:"name"`
		Nat        bool     `json:"nat"`
		Vmstoclone []string `json:"vmstoclone"`
	}

	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Missing fields").Error()})
		return
	}

	if len(form.Vmstoclone) > 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many VMs in custom pod"})
		return
	}

    fmt.Printf("User %s is cloning custom pod %s\n", username, form.Name)

	err = vSphereCustomClone(form.Name, form.Vmstoclone, form.Nat, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to deploy custom pod").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func adminBulkClonePods(c *gin.Context) {
	var form struct {
		Template string   `json:"template"`
		Users    []string `json:"users"`
	}

	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Missing fields").Error()})
		return
	}

	err = bulkClonePods(form.Template, form.Users)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to deploy pods").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pods deployed successfully!"})
}

func adminBulkDeletePods(c *gin.Context) {
    var form struct {
        Filters []string `json:"filters"`
    }

    err := c.ShouldBindJSON(&form)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Missing fields").Error()})
        return
    }

    failed, err := bulkDeletePods(form.Filters)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to delete pods").Error()})
        return
    }

    if len(failed) > 0 {
        failedStr := strings.Join(failed, ", ")
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to delete pods: " + failedStr})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Pods deleted successfully!"})
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

    fmt.Printf("User %s is cloning template %s\n", username, template)
	err = vSphereTemplateClone(template, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Failed to deploy template pod").Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func health(c *gin.Context) {
	rc, err := vSphereClient.restClient.Session(vSphereClient.ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrap(err, "Failed to get session").Error()})
		return
	}

	if rc == nil {
		u, _ := soap.ParseURL(vCenterConfig.VCenterURL)
		u.User = url.UserPassword(vCenterConfig.VCenterUsername, vCenterConfig.VCenterPassword)
		err = vSphereClient.restClient.Login(vSphereClient.ctx, u.User)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrap(err, "Failed to refresh Rest Client").Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
