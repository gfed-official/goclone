package vsphere

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (v *VSphereClient) GetPodsHandler(c *gin.Context) {
    username := sessions.Default(c).Get("id")

    pods, err := vSphereGetPods(username.(string))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting pods"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"pods": pods})
}

func (v *VSphereClient) DeletePodHandler(c *gin.Context) {
    podId := c.Param("podId")
    username := sessions.Default(c).Get("id")
    podOwner := strings.Split(podId, "_")
    podOwner = podOwner[:len(podOwner)-1]

    if strings.ToLower(username.(string)) != strings.ToLower(podOwner[0]) {
        c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
        return
    }

    err := DestroyResources(podId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully!"})
}

func (v *VSphereClient) GetPresetTemplatesHandler(c *gin.Context) {
    isAdmin := sessions.Default(c).Get("isAdmin").(bool)
    templates, err := v.vSphereGetPresetTemplates(isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (v *VSphereClient) GetTemplateVMsHandler(c *gin.Context) {
    templates, err := vSphereGetCustomTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (v *VSphereClient) CloneFromTemplateHandler(c *gin.Context) {
    var jsonData map[string]interface{} // cheaty solution to avoid form struct xd
	err := c.ShouldBindJSON(&jsonData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template := jsonData["template"].(string)
    username := sessions.Default(c).Get("id").(string)

	fmt.Printf("User %s is cloning template %s\n", username, template)
	err = v.vSphereTemplateClone(template, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func (v *VSphereClient) CloneCustomPodHandler(c *gin.Context) {
    username := sessions.Default(c).Get("id").(string)

	var form struct {
		Name       string   `json:"name"`
		Nat        bool     `json:"nat"`
		Vmstoclone []string `json:"vmstoclone"`
	}

	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(form.Vmstoclone) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many VMs in custom pod"})
		return
	}

	fmt.Printf("User %s is cloning custom pod %s\n", username, form.Name)
	err = v.vSphereCustomClone(form.Name, form.Vmstoclone, form.Nat, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func (v *VSphereClient) RefreshTemplatesHandler(c *gin.Context) {
    err := LoadTemplates()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Templates refreshed successfully!"})
}
