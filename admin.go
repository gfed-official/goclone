package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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
	defer ldapClient.Disconnect()

	wg := errgroup.Group{}
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

		wg.Go(func() error {
			err := singleTemplateClone(template, user)
			if err != nil {
				return errors.Wrap(err, "Error cloning pod")
			}
			return nil
		},
		)
	}

	if err := wg.Wait(); err != nil {
		return errors.Wrap(err, "Error cloning pods")
	}

	return nil
}

func refreshTemplates(c *gin.Context) {
	err := LoadTemplates()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Error").Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Templates refreshed successfully!"})
}

func singleTemplateClone(templateId string, username string) error {
	err := vSpherePodLimit(username)
	if err != nil {
		return err
	}

	var nextAvailablePortGroup int
	availablePortGroups.Mu.Lock()
	for i := vCenterConfig.StartingPortGroup; i < vCenterConfig.EndingPortGroup; i++ {
		if _, exists := availablePortGroups.Data[i]; !exists {
			nextAvailablePortGroup = i
			availablePortGroups.Data[i] = fmt.Sprintf("%v_%s", nextAvailablePortGroup, vCenterConfig.PortGroupSuffix)
			break
		}
	}
	availablePortGroups.Mu.Unlock()

	err = TemplateClone(templateId, username, nextAvailablePortGroup)
	if err != nil {
		return err
	}

	return nil
}
