package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func adminGetAllPods(c *gin.Context) {
    kaminoPods, err := GetChildResourcePools(vCenterConfig.TargetResourcePool)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    competitionPods, err := GetChildResourcePools(vCenterConfig.CompetitionResourcePool)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var pods []Pod
    for _, pod := range kaminoPods {
        podName, err := pod.ObjectName(vSphereClient.ctx)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        pods = append(pods, Pod{
            Name: podName,
            ResourceGroup: pod.Reference().Value,
            ServerGUID: pod.Reference().ServerGUID,
        },)
    }

    for _, pod := range competitionPods {
        podName, err := pod.ObjectName(vSphereClient.ctx)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        pods = append(pods, Pod{
            Name: podName,
            ResourceGroup: pod.Reference().Value,
            ServerGUID: pod.Reference().ServerGUID,
        },)
    }

    c.JSON(http.StatusOK, pods)
}


func adminDeletePod(c *gin.Context) {
    podId := c.Param("podId")

    err := DestroyResources(podId)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
        },)
    }

    if err := wg.Wait(); err != nil {
        return errors.Wrap(err, "Error cloning pods")
    }

    return nil
}

func bulkDeletePods(filter []string) ([]string, error) {
    kaminoPods, err := GetChildResourcePools(vCenterConfig.TargetResourcePool)
    if err != nil {
        return []string{}, errors.Wrap(err, "Error getting Kamino pods")
    }

    competitionPods, err := GetChildResourcePools(vCenterConfig.CompetitionResourcePool)
    if err != nil {
        return []string{}, errors.Wrap(err, "Error getting Competition pods")
    }

    pods := append(kaminoPods, competitionPods...)
    failed := []string{}
    wg := errgroup.Group{}
    for _, pod := range pods {
        podName, err := pod.ObjectName(vSphereClient.ctx)
        if err != nil {
            return []string{}, errors.Wrap(err, "Error getting pod name")
        }

        for _, f := range filter {
            if f == "" {
                continue
            }
            if strings.Contains(podName, f) {
                wg.Go(func() error {
                    err := DestroyResources(podName)
                    if err != nil {
                        fmt.Printf("Error destroying resources for pod %s: %v\n", podName, err)
                        failed = append(failed, podName)
                        return err
                    }
                    return nil
                },)
            }
        }
    }

    if err := wg.Wait(); err != nil {
        return failed, errors.Wrap(err, "Error deleting pods:")
    }

    return failed, nil
}

func refreshTemplates(c *gin.Context) {
    err := LoadTemplates()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

