package main

import (
	"fmt"
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

func bulkClonePods(template string, users []string) error {
    ldapClient := Client{}
    err := ldapClient.Connect()
    if err != nil {
        return errors.Wrap(err, "Error connecting to LDAP")
    }
    for _, user := range users {
        if user == "" {
            continue
        }
        exists, err := ldapClient.UserExists(user)
        if err != nil {
            return errors.Wrap(err, "Error checking if user exists")
        }
        if !exists {
            fmt.Printf("User %s does not exist, skipping\n", user)
            continue
        }
        err = vSphereTemplateClone(template, user)
        if err != nil {
            return errors.Wrap(err, "Error cloning pod")
        }
    }
    return nil
}
