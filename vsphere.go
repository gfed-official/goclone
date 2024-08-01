package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"goclone/models"
)

type RWPortGroupMap struct {
	Mu   sync.Mutex
	Data map[int]string
}

var (
	availablePortGroups = &RWPortGroupMap{
		Data: make(map[int]string),
	}
)

func refreshSession() {
	for {
		time.Sleep(time.Minute * 5)

		err := vSphereLoadTakenPortGroups()
		if err != nil {
			log.Println(errors.Wrap(err, "Error finding taken port groups"))
		} else {
			log.Println("Session refreshed successfully")
		}
	}
}

func vSphereLoadTakenPortGroups() error {
	podNetworks, err := finder.NetworkList(vSphereClient.ctx, "*_"+tomlConf.PortGroupSuffix)
	if err != nil {
		return errors.Wrap(err, "Failed to list networks")
	}

	// Collect found DistributedVirtualPortgroup refs
	var refs []types.ManagedObjectReference
	for _, pgRef := range podNetworks {
		refs = append(refs, pgRef.Reference())
	}

	pc := property.DefaultCollector(vSphereClient.client)

	// Collect property from references list
	var pgs []mo.DistributedVirtualPortgroup
	err = pc.Retrieve(vSphereClient.ctx, refs, []string{"name"}, &pgs)
	if err != nil {
		errors.Wrap(err, "Failed to get references for Virtual Port Groups")
	}

	availablePortGroups.Mu.Lock()
	for _, pg := range pgs {
		r, _ := regexp.Compile("^\\d+")
		match := r.FindString(pg.Name)
		pgNumber, _ := strconv.Atoi(match)
		if pgNumber >= tomlConf.StartingPortGroup && pgNumber < tomlConf.EndingPortGroup {
			availablePortGroups.Data[pgNumber] = pg.Name
		}
	}
	availablePortGroups.Mu.Unlock()
	log.Printf("Found %d port groups within on-demand DistributedPortGroup range: %d - %d", len(availablePortGroups.Data), tomlConf.StartingPortGroup, tomlConf.EndingPortGroup)
	return nil
}

func vSpherePodLimit(username string) error {
	existingPods, err := vSphereGetPods(username)

	if err != nil {
		return err
	}

	if len(existingPods) >= tomlConf.MaxPodLimit {
		return errors.New("Max pod limit reached")
	}
	return nil
}

func vSphereGetPresetTemplates() ([]string, error) {
	var templates []string

	templateResourcePool, err := finder.ResourcePool(vSphereClient.ctx, tomlConf.PresetTemplateResourcePool)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to find preset template resource pool")
	}

	var trp mo.ResourcePool
	err = templateResourcePool.Properties(vSphereClient.ctx, templateResourcePool.Reference(), []string{"resourcePool"}, &trp)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to find preset templates")
	}

	pc := property.DefaultCollector(vSphereClient.client)

	var rps []mo.ResourcePool
	err = pc.Retrieve(vSphereClient.ctx, trp.ResourcePool, []string{"name"}, &rps)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to collect references for preset templates")
	}

	for _, rp := range rps {
		templates = append(templates, rp.Name)
	}

	return templates, nil
}

func vSphereGetCustomTemplates() ([]gin.H, error) {
	var templates []gin.H

	templateFolder, err := finder.Folder(vSphereClient.ctx, tomlConf.TemplateFolder)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to find templates folder")
	}

	templateSubfolderRefs, err := templateFolder.Children(vSphereClient.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to find template sub-folders")
	}

	pc := property.DefaultCollector(vSphereClient.client)

	for _, subfolderRef := range templateSubfolderRefs {
		var subfolder []mo.Folder
		err := pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{subfolderRef.Reference()}, []string{"name", "childEntity"}, &subfolder)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to retrieve templates from sub-folders")
		}
		var vms []mo.VirtualMachine
		for _, vmRef := range subfolder[0].ChildEntity {
			err := pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{vmRef.Reference()}, []string{"name"}, &vms)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to retrieve VM template")
			}
		}
		var vmNames []string
		for _, vm := range vms {
			vmNames = append(vmNames, vm.Name)
		}
		subfolderData := gin.H{"name": subfolder[0].Name, "vms": vmNames}
		templates = append(templates, subfolderData)
	}

	return templates, nil
}

func vSphereGetPods(owner string) ([]models.Pod, error) {
	var pods []models.Pod

	ownerPods, err := finder.VirtualAppList(vSphereClient.ctx, fmt.Sprintf("*_%s", owner)) // hard coded based on our naming scheme

	// No pods found
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			return pods, nil
		}
		return nil, errors.Wrap(err, "Failed to get vApp list")
	}

	// Collect found vApp refs
	var refs []types.ManagedObjectReference
	for _, podRef := range ownerPods {
		refs = append(refs, podRef.Reference())
	}

	pc := property.DefaultCollector(vSphereClient.client)

	var rps []mo.ResourcePool
	err = pc.Retrieve(vSphereClient.ctx, refs, []string{"name", "config"}, &rps)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to collect references for your pods")
	}

	// serviceInstance := mo.ServiceInstance{}
	// err = vSphereClient.RetrieveOne(mainCtx, , nil, &serviceInstance)

	for _, rp := range rps {
		pods = append(pods, models.Pod{Name: rp.Name, ResourceGroup: rp.Config.Entity.Value, ServerGUID: vSphereClient.client.ServiceContent.About.InstanceUuid})
	}

	return pods, nil
}

func vSphereDeletePod(podId string, username string) error {
	cmd := exec.Command("pwsh", ".\\pwsh\\deletepod.ps1", tomlConf.VCenterURL, username, podId)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}

	availablePortGroups.Mu.Lock()
	deleted_pg, _ := strconv.Atoi(strings.Split(podId, "_")[0])
	delete(availablePortGroups.Data, deleted_pg)
	availablePortGroups.Mu.Unlock()

	return nil
}

func vSphereTemplateClone(templateId string, username string) error {
	err := vSpherePodLimit(username)
	if err != nil {
		return err
	}

	var nextAvailablePortGroup string

	availablePortGroups.Mu.Lock()
	for i := tomlConf.StartingPortGroup; i < tomlConf.EndingPortGroup; i++ {
		if _, exists := availablePortGroups.Data[i]; !exists {
			nextAvailablePortGroup = strconv.Itoa(i)
			availablePortGroups.Data[i] = fmt.Sprintf("%s_%s", nextAvailablePortGroup, tomlConf.PortGroupSuffix)
			break
		}
	}
	availablePortGroups.Mu.Unlock()
	cmd := exec.Command("pwsh", ".\\pwsh\\cloneondemand.ps1", tomlConf.VCenterURL, templateId, username, nextAvailablePortGroup, tomlConf.TargetResourcePool, tomlConf.Domain, tomlConf.WanPortGroup)

	var out bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		log.Println(stderr.String())
		return err
	}

	log.Println(stderr.String())

	return nil
}

func vSphereCustomClone(podName string, vmsToClone []string, nat bool, username string) error {
	err := vSpherePodLimit(username)
	if err != nil {
		return err
	}

	var nextAvailablePortGroup string

	availablePortGroups.Mu.Lock()
	for i := tomlConf.StartingPortGroup; i < tomlConf.EndingPortGroup; i++ {
		if _, exists := availablePortGroups.Data[i]; !exists {
			nextAvailablePortGroup = strconv.Itoa(i)
			availablePortGroups.Data[i] = fmt.Sprintf("%s_%s", nextAvailablePortGroup, tomlConf.PortGroupSuffix)
			break
		}
	}
	availablePortGroups.Mu.Unlock()

	var natString string
	if nat {
		natString = "$true"
	} else {
		natString = "$false"
	}

	var formattedSlice []string
	for _, s := range vmsToClone {
		formattedSlice = append(formattedSlice, `"`+s+`"`)
	}

	vms := strings.Join(formattedSlice, ",")
	cmd := exec.Command("pwsh", "-Command", ".\\pwsh\\customclone.ps1", tomlConf.VCenterURL, podName, username, vms, natString, nextAvailablePortGroup, tomlConf.TargetResourcePool, tomlConf.Domain, tomlConf.WanPortGroup)

	var out bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		log.Println(stderr.String())
		return err
	}

	log.Println(stderr.String())

	return nil
}

func WebClone(sourceRP, targetRP, wanPG, username string, portGroup int32) {
	/**
	err := vSpherePodLimit(username)
	if err != nil {
		log.Println(err)
		return
	}
	*/

	strPortGroup := strconv.Itoa(int(portGroup))
	pgName := strings.Join([]string{strPortGroup, tomlConf.PortGroupSuffix}, "_")
	tagName := strings.Join([]string{strPortGroup, sourceRP, username}, "_")

	tag, err := CreateTag(tagName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating tag"))
	}

	pgRef, err := CreatePortGroup(pgName, portGroup)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating portgroup"))
	}

	err = AssignTagToObject(tag, pgRef)
	if err != nil {
		log.Println(errors.Wrap(err, "Error assigning tag"))
	}

	newFolder, err := CreateVMFolder(tagName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating VM folder"))
	}

	err = AssignTagToObject(tag, newFolder)
	if err != nil {
		log.Println(errors.Wrap(err, "Error assigning tag"))
	}

	srcRpRef, err := GetResourcePool(sourceRP)
    if err != nil {
        log.Println(errors.Wrap(err, "Error getting resource pool"))
    }

    dstRpRef, err := GetResourcePool(targetRP)
    if err != nil {
        log.Println(errors.Wrap(err, "Error getting resource pool"))
    }

	tagsOnTmpl, err := GetTagsFromObject(srcRpRef)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tags"))
	}

	natted := false
	for _, tag := range tagsOnTmpl {
		if tag.Name == "natted" {
			natted = true
			fmt.Println(natted)
		}
	}

	vms, err := GetVMsInResourcePool(srcRpRef)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VMs in resource pool"))
	}

	if vmsWithoutSnapshot := GetSnapshot(vms, "SnapshotForCloning"); vmsWithoutSnapshot != nil {
		err = CreateSnapshot(vmsWithoutSnapshot, "SnapshotForCloning")
		if err != nil {
			log.Println(errors.Wrap(err, "Error creating snapshot"))
		}
		fmt.Println("Snapshot created", vmsWithoutSnapshot)
	}

	CloneVMs(vms, newFolder, dstRpRef, datastore.Reference())

    clonedVMs, err := GetVMsInResourcePool(dstRpRef)
    if err != nil {
        log.Println(errors.Wrap(err, "Error getting VMs in resource pool"))
    }

    for _, vm := range clonedVMs {
        ConfigureVMNetwork(vm, pgRef.Reference(), pgName)
    }
}
