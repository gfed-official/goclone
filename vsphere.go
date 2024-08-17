package main

import (
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
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
	podNetworks, err := finder.NetworkList(vSphereClient.ctx, "*_"+vCenterConfig.PortGroupSuffix)
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
		if pgNumber >= vCenterConfig.StartingPortGroup && pgNumber < vCenterConfig.EndingPortGroup {
			availablePortGroups.Data[pgNumber] = pg.Name
		}
	}
	availablePortGroups.Mu.Unlock()
	log.Printf("Found %d port groups within on-demand DistributedPortGroup range: %d - %d", len(availablePortGroups.Data), vCenterConfig.StartingPortGroup, vCenterConfig.EndingPortGroup)
	return nil
}

func vSpherePodLimit(username string) error {
	existingPods, err := vSphereGetPods(username)

	if err != nil {
		return err
	}

	if len(existingPods) >= vCenterConfig.MaxPodLimit {
		return errors.New("Max pod limit reached")
	}
	return nil
}

func vSphereGetPresetTemplates() ([]string, error) {
	var templates []string

	templateResourcePool, err := finder.ResourcePool(vSphereClient.ctx, vCenterConfig.PresetTemplateResourcePool)

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

	templateFolder, err := finder.Folder(vSphereClient.ctx, vCenterConfig.TemplateFolder)

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

func vSphereTemplateClone(templateId string, username string) error {
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

func vSphereCustomClone(podName string, vmsToClone []string, nat bool, username string) error {
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

	err = CustomClone(podName, vmsToClone, nat, username, nextAvailablePortGroup)
	if err != nil {
		return err
	}

	return nil
}

func TemplateClone(sourceRP, username string, portGroup int) error {
	targetRP, pg, newFolder, err := InitializeClone(sourceRP, username, portGroup)

	srcRp, err := GetResourcePool(sourceRP)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool"))
		return err
	}

	tagsOnTmpl, err := GetTagsFromObject(srcRp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tags"))
		return err
	}

	natted := false
	noRouter := false
	for _, tag := range tagsOnTmpl {
		if tag.Name == "natted" {
			natted = true
		}
		if tag.Name == "NoRouter" {
			noRouter = true
		}
	}

	vms, err := GetVMsInResourcePool(srcRp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VMs in resource pool"))
		return err
	}

	var router *mo.VirtualMachine
	if !noRouter {
		if !slices.ContainsFunc(vms, func(vm mo.VirtualMachine) bool {
			if strings.Contains(vm.Name, "PodRouter") {
				router = &vm
				return true
			} else {
				return false
			}
		}) {
			router, err = CreateRouter(srcRp.Reference(), datastore.Reference(), templateFolder, natted)
			vms = append(vms, *router)
		}
	}

	err = CreateSnapshot(vms, "SnapshotForCloning")
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating snapshot"))
		return err
	}

	pgStr := strconv.Itoa(portGroup)
	CloneVMs(vms, newFolder, targetRP.Reference(), datastore.Reference(), pg.Reference(), pgStr)

	vmClones, err := newFolder.Children(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting children"))
		return err
	}

	var vmClonesMo []mo.VirtualMachine
	for _, vm := range vmClones {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		var vm mo.VirtualMachine

		fmt.Println(vmObj.Reference())
		fmt.Println(vmObj.ObjectName(vSphereClient.ctx))

		err = vmObj.Properties(vSphereClient.ctx, vmObj.Reference(), []string{"name"}, &vm)
		if err != nil {
			log.Println(errors.Wrap(err, "Error getting VM properties"))
			return err
		}
		vmClonesMo = append(vmClonesMo, vm)
		if strings.Contains(vm.Name, "PodRouter") {
			router = &vm
		}
	}

	if !noRouter {
		err = ConfigRouter(pg.Reference(), wanPG.Reference(), router, pgStr)
		if err != nil {
			log.Println(errors.Wrap(err, "Error cloning router"))
			return err
		}

		if natted {
			pgOctet, err := GetNatOctet(strconv.Itoa(portGroup))
			if err != nil {
				return err
			}
			program := types.GuestProgramSpec{
				ProgramPath: vCenterConfig.RouterProgram,
				Arguments:   fmt.Sprintf(vCenterConfig.RouterProgramArgs, pgOctet),
			}

			auth := types.NamePasswordAuthentication{
				Username: vCenterConfig.RouterUsername,
				Password: vCenterConfig.RouterPassword,
			}
			err = RunProgramOnVM(router, program, auth)
			if err != nil {
				log.Println(errors.Wrap(err, "Error running program on router"))
				return err
			}
		}
	}
	SnapshotVMs(vmClonesMo, "Base")

    permission := types.Permission{
        Principal: strings.Join([]string{mainConfig.Domain, username}, "\\"),
        RoleId:    cloneRole.RoleId,
        Propagate: true,
    }
	AssignPermissionToObjects(&permission, []types.ManagedObjectReference{newFolder.Reference()})

	return nil
}

func CustomClone(podName string, vmsToClone []string, natted bool, username string, portGroup int) error {
	targetRP, pg, newFolder, err := InitializeClone(podName, username, portGroup)
	if err != nil {
		log.Println(errors.Wrap(err, "Error initializing clone"))
		return err
	}
	var vms []mo.VirtualMachine
	for _, vm := range vmsToClone {
		vmObj, err := finder.VirtualMachine(vSphereClient.ctx, vm)
		if err != nil {
			log.Println(errors.Wrap(err, "Error finding VM"))
			return err
		}
		var vmMo mo.VirtualMachine
		err = vmObj.Properties(vSphereClient.ctx, vmObj.Reference(), []string{"name"}, &vmMo)
		if err != nil {
			log.Println(errors.Wrap(err, "Error getting VM properties"))
			return err
		}
		vms = append(vms, vmMo)
	}

	pgStr := strconv.Itoa(portGroup)
	CloneVMsFromTemplates(vms, newFolder, targetRP.Reference(), datastore.Reference(), pg.Reference(), pgStr)

	router, err := CreateRouter(targetRP.Reference(), datastore.Reference(), newFolder, natted)
	vms = append(vms, *router)

	vmClones, err := newFolder.Children(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting children"))
		return err
	}

	var vmClonesMo []mo.VirtualMachine
	for _, vm := range vmClones {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		var vm mo.VirtualMachine
		err = vmObj.Properties(vSphereClient.ctx, vmObj.Reference(), []string{"name"}, &vm)
		vmClonesMo = append(vmClonesMo, vm)
	}

	if natted {
		pgOctet, err := GetNatOctet(strconv.Itoa(portGroup))
		if err != nil {
			return err
		}
		program := types.GuestProgramSpec{
			ProgramPath: vCenterConfig.RouterProgram,
			Arguments:   fmt.Sprintf(vCenterConfig.RouterProgramArgs, pgOctet),
		}

		auth := types.NamePasswordAuthentication{
			Username: vCenterConfig.RouterUsername,
			Password: vCenterConfig.RouterPassword,
		}

		err = RunProgramOnVM(router, program, auth)
		if err != nil {
			log.Println(errors.Wrap(err, "Error running program on router"))
			return err
		}
	}
	SnapshotVMs(vmClonesMo, "Base")

    permission := types.Permission{
        Principal: strings.Join([]string{mainConfig.Domain, username}, "\\"),
        RoleId:    customCloneRole.RoleId,
        Propagate: true,
    }
	AssignPermissionToObjects(&permission, []types.ManagedObjectReference{newFolder.Reference()})

	return nil
}

func InitializeClone(podName, username string, portGroup int) (*types.ManagedObjectReference, object.NetworkReference, *object.Folder, error) {
	strPortGroup := strconv.Itoa(int(portGroup))
	pgName := strings.Join([]string{strPortGroup, vCenterConfig.PortGroupSuffix}, "_")
	tagName := strings.Join([]string{strPortGroup, podName, username}, "_")

	tag, err := CreateTag(tagName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating tag"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}

	targetRP, err := CreateResourcePool(tagName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating resource pool"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}

	pg, err := CreatePortGroup(pgName, portGroup)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating portgroup"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}

	err = AssignTagToObject(tag, pg)
	if err != nil {
		log.Println(errors.Wrap(err, "Error assigning tag"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}

	newFolder, err := CreateVMFolder(tagName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating VM folder"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}
	return &targetRP, pg, newFolder, nil
}

func DestroyResources(podId string) error {
	folder, err := finder.Folder(vSphereClient.ctx, podId)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding folder"))
		return err
	}
	DestroyFolder(folder)

	resourcePool, err := GetResourcePool(podId)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool"))
		return err
	}
	DestroyResourcePool(resourcePool)

	pg, err := GetPortGroup(strings.Join([]string{strings.Split(podId, "_")[0], vCenterConfig.PortGroupSuffix}, "_"))
	err = DestroyPortGroup(pg.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying portgroup"))
		return err
	}

	availablePortGroups.Mu.Lock()
	deleted_pg, _ := strconv.Atoi(strings.Split(podId, "_")[0])
	delete(availablePortGroups.Data, deleted_pg)
	availablePortGroups.Mu.Unlock()

	err = DestroyTag(podId)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying tag"))
		return err
	}

	return nil
}

func GetNatOctet(pg string) (int, error) {
	pgInt, err := strconv.Atoi(pg)
	if err != nil {
		return -1, errors.New("Port group is not a number")
	}

	if pgInt < vCenterConfig.StartingPortGroup || pgInt > vCenterConfig.EndingPortGroup || pgInt > vCenterConfig.StartingPortGroup+255 {
		return -1, errors.New("Port group out of range")
	}

	return pgInt - vCenterConfig.StartingPortGroup + 1, nil
}
