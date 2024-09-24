package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
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
        return failed, errors.Wrap(err, "Error deleting pods")
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
    startPG := vCenterConfig.StartingPortGroup
    endPG := vCenterConfig.EndingPortGroup

    if templateMap[templateId].CompetitionPod {
        startPG = vCenterConfig.CompetitionStartPortGroup
        endPG = vCenterConfig.CompetitionEndPortGroup
    }

	var nextAvailablePortGroup int
	availablePortGroups.Mu.Lock()
    for i := startPG; i < endPG; i++ {
        if _, exists := availablePortGroups.Data[i]; !exists {
            nextAvailablePortGroup = i
            availablePortGroups.Data[i] = fmt.Sprintf("%v_%s", nextAvailablePortGroup, vCenterConfig.PortGroupSuffix)
            break
        }
	}
	availablePortGroups.Mu.Unlock()

    err := TemplateClone(templateId, username, nextAvailablePortGroup)
    if err != nil {
        return err
    }

    return nil
}

func competitionSingleTemplateClone(templateId string) (map[string]string, error) {
    startPG := vCenterConfig.StartingPortGroup
    endPG := vCenterConfig.EndingPortGroup

    if templateMap[templateId].CompetitionPod {
        startPG = vCenterConfig.CompetitionStartPortGroup
        endPG = vCenterConfig.CompetitionEndPortGroup
    }

	var nextAvailablePortGroup int
	availablePortGroups.Mu.Lock()
    for i := startPG; i < endPG; i++ {
        if _, exists := availablePortGroups.Data[i]; !exists {
            nextAvailablePortGroup = i
            availablePortGroups.Data[i] = fmt.Sprintf("%v_%s", nextAvailablePortGroup, vCenterConfig.PortGroupSuffix)
            break
        }
	}
	availablePortGroups.Mu.Unlock()

    ldapClient := Client{}
    err := ldapClient.Connect()
    username := fmt.Sprintf("Team%02d", nextAvailablePortGroup-startPG+1)
    password, err := createUser(ldapClient, username)
    if err != nil {
        return nil, err
    }

    err = TemplateClone(templateId, username, nextAvailablePortGroup)
    if err != nil {
        return nil, err
    }

    userMap := map[string]string{
        "username": username,
        "password": password,
    }
    return userMap, nil
}

func bulkCreateUsers(c *gin.Context) {
    var users struct {
        Users []string `json:"users"`
    }
    if err := c.ShouldBindJSON(&users); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    ldapClient := Client{}
    err := ldapClient.Connect()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    badUsers := []string{}
    goodUsers := []map[string]string{}
    for _, user := range users.Users {
        if user == "" {
            continue
        }
        password, err := createUser(ldapClient, user)
        if err != nil {
            badUsers = append(badUsers, user)
        }
        goodUsers = append(goodUsers, map[string]string{"username": user, "password": string(password)})
    }

    if len(badUsers) > 0 {
        userJson, _ := json.Marshal(badUsers)
        c.JSON(http.StatusBadRequest, gin.H{"error": userJson})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Users created successfully!", "users": goodUsers})
}

func competitionClone(template string, count int) (map[string]string, error) {
    wg := errgroup.Group{}
    userMap := map[string]string{}
    failed := []string{}
    for i := 1; i <= count; i++ {
        wg.Go(func() error {
            res, err := competitionSingleTemplateClone(template)
            if err != nil {
                failed = append(failed, fmt.Sprintf("Team%02d", i))
                return err
            }
            userMap[res["username"]] = res["password"]
            return nil
        },)
    }

    if err := wg.Wait(); err != nil {
        return nil, errors.Wrap(err, "Error cloning pods")
    }

    if len(failed) > 0 {
        return userMap, errors.New(fmt.Sprintf("Failed to clone pods for users: %v", failed))
    }

    return userMap, nil
}


func createNumUsers(count int) (map[string]string, error) {
    ldapClient := Client{}
    err := ldapClient.Connect()
    if err != nil {
        return map[string]string{}, errors.Wrap(err, "Error connecting to LDAP")
    }
    defer ldapClient.Disconnect()

    users := map[string]string{}
    for i := 1; i <= count; i++ {
        username := fmt.Sprintf("Team%02d", i)
        password, err := createUser(ldapClient, username)
        if err != nil {
            return map[string]string{}, errors.Wrap(err, "Error creating user")
        }
        users[username] = password
    }

    return users, nil
}

func createUser(ldapClient Client, username string) (string, error) {
    exists, err := ldapClient.UserExists(username)
    if err != nil {
        return "", errors.Wrap(err, "Error checking if user exists")
    }
    if exists {
        return "", errors.Wrap(err, "User already exists")
    }

    var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
    password := make([]rune, 8)
    for i := range password {
        password[i] = letters[rand.Intn(len(letters))]
    }
    err = ldapClient.registerUser(username, string(password))
    if err != nil {
        return "", errors.Wrap(err, "Error registering user")
    }

    return string(password), nil
}
