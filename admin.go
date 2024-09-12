package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

func adminGetAllPods(c *gin.Context) {
    // Get all pods
    pods, err := vSphereGetPods("*")
    if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
        return
    }

    c.JSON(http.StatusOK, pods)
}

func adminDeletePod(c *gin.Context) {
	podId := c.Param("podId")

	err := DestroyResources(podId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully!"})
}

