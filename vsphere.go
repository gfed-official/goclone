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

	"goclone/vm"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/sync/errgroup"
)

type RWPortGroupMap struct {
	Mu   sync.Mutex
	Data map[int]string
}

type Pod struct {
	Name          string
	ResourceGroup string
	ServerGUID    string
}

type Template struct {
	Name           string
	SourceRP       *object.ResourcePool
	VMs            []vm.VM
	VMObjects      []object.VirtualMachine
	Natted         bool
	NoRouter       bool
	CompetitionPod bool
	AdminOnly      bool
	WanPG          *object.DistributedVirtualPortgroup
}

var (
	availablePortGroups = &RWPortGroupMap{
		Data: make(map[int]string),
	}
)

func refreshSession() {
	for {
		time.Sleep(time.Second * 30)

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
		if (pgNumber >= vCenterConfig.StartingPortGroup && pgNumber < vCenterConfig.EndingPortGroup) || (pgNumber >= vCenterConfig.CompetitionStartPortGroup && pgNumber < vCenterConfig.CompetitionEndPortGroup) {
			availablePortGroups.Data[pgNumber] = pg.Name
		}
	}
	availablePortGroups.Mu.Unlock()
	log.Printf("Found %d port groups", len(availablePortGroups.Data))
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

func vSphereGetPresetTemplates(username string) ([]string, error) {
	var templates []string

	ldapClient := Client{}
	err := ldapClient.Connect()
	defer ldapClient.Disconnect()

	isAdm, err := ldapClient.IsAdmin(username)
	if err != nil {
		return nil, err
	}

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
	err = pc.Retrieve(vSphereClient.ctx, trp.ResourcePool, []string{"name", "customValue"}, &rps)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to collect references for preset templates")
	}

	for _, rp := range rps {
		rpObj := object.NewResourcePool(vSphereClient.client, rp.Reference())
		rpName, err := rpObj.ObjectName(vSphereClient.ctx)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get resource pool name")
		}

		adminOnly := templateMap[rpName].AdminOnly

		if !isAdm && adminOnly {
			continue
		}

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

	folderChildren, err := templateFolder.Children(vSphereClient.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to find template sub-folders")
	}

	pc := property.DefaultCollector(vSphereClient.client)
	for _, subfolderRef := range folderChildren {
		var subfolder []mo.Folder
		switch subfolderRef.(type) {
		case *object.Folder:
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
		default:
			continue
		}
	}
	return templates, nil
}

func vSphereGetPods(owner string) ([]Pod, error) {
	var pods []Pod

	ownerPods, err := finder.ResourcePoolList(vSphereClient.ctx, fmt.Sprintf("*_%s", owner)) // hard coded based on our naming scheme
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			return pods, nil
		}
		return nil, errors.Wrap(err, "Failed to get pod list")
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

	for _, rp := range rps {
		pods = append(pods, Pod{Name: rp.Name, ResourceGroup: rp.Config.Entity.Value, ServerGUID: vSphereClient.client.ServiceContent.About.InstanceUuid})
	}

	return pods, nil
}

func vSphereTemplateClone(templateId string, username string) error {
	err := vSpherePodLimit(username)
	if err != nil {
		return err
	}

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

	pgStr := strconv.Itoa(portGroup)
	CloneVMs(templateMap[sourceRP].VMs, newFolder, targetRP.Reference(), datastore.Reference(), pg.Reference(), pgStr)

	vmClones, err := newFolder.Children(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting children"))
		return err
	}

	var vms []vm.VM
    var router vm.VM
	for _, v := range vmClones {
		vmObj := object.NewVirtualMachine(vSphereClient.client, v.Reference())
        vmName, err := vmObj.ObjectName(vSphereClient.ctx)
        if err != nil {
            log.Println(errors.Wrap(err, "Error getting VM name"))
            return err
        }

        isRouter := strings.Contains(vmName, "PodRouter")
        newVM := vm.VM{
            Name: vmName,
            Ref: v.Reference(),
            Ctx: &vSphereClient.ctx,
            Client: vSphereClient.client,
            IsRouter: isRouter,
        }

        if isRouter {
            router = newVM
        }
        vms = append(vms, newVM)
	}

	var routerPG *object.DistributedVirtualPortgroup
	if templateMap[sourceRP].CompetitionPod {
		routerPG = competitionPG
	} else {
		routerPG = templateMap[sourceRP].WanPG
	}

	if !templateMap[sourceRP].NoRouter {
        err := router.ConfigureRouterNetworks(pg.(*object.DistributedVirtualPortgroup), routerPG, dvsMo)
        if err != nil {
            log.Println(errors.Wrap(err, "Error configuring router networks"))
            return err
        }

		if templateMap[sourceRP].Natted {
			pgOctet, err := GetNatOctet(strconv.Itoa(portGroup))
			if err != nil {
				return err
			}

			var networkID string
			if templateMap[sourceRP].CompetitionPod {
				octets := strings.Split(vCenterConfig.CompetitionNetworkID, ".")
				networkID = fmt.Sprintf("%s.%s", octets[0], octets[1])
			} else {
				octets := strings.Split(vCenterConfig.DefaultNetworkID, ".")
				networkID = fmt.Sprintf("%s.%s", octets[0], octets[1])
			}

			program := types.GuestProgramSpec{
				ProgramPath: vCenterConfig.RouterProgram,
				Arguments:   fmt.Sprintf(vCenterConfig.RouterProgramArgs, pgOctet, networkID),
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

    wg := errgroup.Group{}
    for _, vm := range vms {
        wg.Go(func() error {
            return vm.SetSnapshot("Base")
        })
    }

    if err := wg.Wait(); err != nil {
        return errors.Wrap(err, "Error setting snapshot")
    }

	permission := types.Permission{
		Principal: strings.Join([]string{mainConfig.Domain, username}, "\\"),
		RoleId:    cloneRole.RoleId,
		Propagate: true,
	}
	AssignPermissionToObjects(&permission, []types.ManagedObjectReference{newFolder.Reference()})

    hiddenVMs := []vm.VM{}
    for _, vm := range templateMap[sourceRP].VMs {
        if vm.IsHidden {
            hiddenVMs = append(hiddenVMs, vm)
        }
    }

	HideVMs(hiddenVMs, username)

	return nil
}

func CustomClone(podName string, vmsToClone []string, natted bool, username string, portGroup int) error {
	targetRP, pg, newFolder, err := InitializeClone(podName, username, portGroup)
	if err != nil {
		log.Println(errors.Wrap(err, "Error initializing clone"))
		return err
	}

	var vms []vm.VM
	for _, v := range vmsToClone {
		vmObj, err := finder.VirtualMachine(vSphereClient.ctx, v)
		if err != nil {
			log.Println(errors.Wrap(err, "Error finding VM"))
			return err
		}
        vmName, err := vmObj.ObjectName(vSphereClient.ctx)
        if err != nil {
            log.Println(errors.Wrap(err, "Error getting VM name"))
            return err
        }

        newVM := vm.VM{
            Name: vmName,
            Ref: vmObj.Reference(),
            Ctx: &vSphereClient.ctx,
            Client: vSphereClient.client,
            IsRouter: strings.Contains(vmName, "PodRouter"),
        }
		vms = append(vms, newVM)
	}

	pgStr := strconv.Itoa(portGroup)
	CloneVMsFromTemplates(vms, newFolder, targetRP.Reference(), datastore.Reference(), pg.Reference(), pgStr)

    hasRouter := false
    for _, vm := range vms {
        if vm.IsRouter {
            hasRouter = true
            break
        }
    }

    router := vm.VM{}
    if !hasRouter && natted {
        router, err := CreateRouter(targetRP.Reference(), datastore.Reference(), newFolder, natted, podName)
        if err != nil {
            log.Println(errors.Wrap(err, "Error creating router"))
            return err
        }
        newVM := vm.VM{
            Name: router.Name,
            Ref: router.Reference(),
            Ctx: &vSphereClient.ctx,
            Client: vSphereClient.client, 
            IsRouter: true,
        }
        vms = append(vms, newVM)
    }

	if natted {
		pgOctet, err := GetNatOctet(strconv.Itoa(portGroup))
		if err != nil {
			return err
		}

		octets := strings.Split(vCenterConfig.DefaultNetworkID, ".")
		networkID := fmt.Sprintf("%s.%s", octets[0], octets[1])

		program := types.GuestProgramSpec{
			ProgramPath: vCenterConfig.RouterProgram,
			Arguments:   fmt.Sprintf(vCenterConfig.RouterProgramArgs, pgOctet, networkID),
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

    wg := errgroup.Group{}
    for _, vm := range vms {
        wg.Go(func() error {
            return vm.SetSnapshot("Base")
        })
    }

    if err := wg.Wait(); err != nil {
        return errors.Wrap(err, "Error setting snapshot")
    }

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

	targetRP, err := CreateResourcePool(tagName, templateMap[podName].CompetitionPod)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating resource pool"))
		return &types.ManagedObjectReference{}, &object.Network{}, &object.Folder{}, err
	}

	pg, err := CreatePortGroup(pgName, portGroup)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating portgroup"))
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
	resourcePool, err := GetResourcePool(podId)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool"))
		return err
	}
	DestroyResourcePool(resourcePool)

	folder, err := finder.Folder(vSphereClient.ctx, podId)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding folder"))
	} else {
		DestroyFolder(folder)
	}

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

	return nil
}

func GetNatOctet(pg string) (int, error) {
	pgInt, err := strconv.Atoi(pg)
	if err != nil {
		return -1, errors.New("Port group is not a number")
	}

	var start int
	if pgInt < vCenterConfig.CompetitionStartPortGroup {
		if pgInt < vCenterConfig.StartingPortGroup || pgInt > vCenterConfig.EndingPortGroup || pgInt > vCenterConfig.StartingPortGroup+255 {
			return -1, errors.New("Port group out of range")
		}
		start = vCenterConfig.StartingPortGroup
	} else {
		if pgInt < vCenterConfig.CompetitionStartPortGroup || pgInt > vCenterConfig.CompetitionEndPortGroup || pgInt > vCenterConfig.CompetitionStartPortGroup+255 {
			return -1, errors.New("Port group out of range")
		}
		start = vCenterConfig.CompetitionStartPortGroup
	}

	return pgInt - start + 1, nil
}

func LoadTemplates() error {
	rpList, err := GetChildResourcePools(vCenterConfig.PresetTemplateResourcePool)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting child resource pools"))
		return err
	}

	for _, rp := range rpList {
		rpName, err := rp.ObjectName(vSphereClient.ctx)
		if err != nil {
			log.Println(errors.Wrap(err, "Error getting resource pool name"))
			return err
		}
		template, err := LoadTemplate(rp, rpName)
		if err != nil {
			log.Println(errors.Wrap(err, "Error loading template"))
			return err
		}
		templateMap[rpName] = template
	}

	return nil
}

func LoadTemplate(rp *object.ResourcePool, name string) (Template, error) {
	attrs, err := GetAllAttributes(rp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting attributes"))
		return Template{}, err
	}

	natted := false
	noRouter := false
	competitionPod := false
	adminOnly := false
	pg := wanPG
	for key, value := range attrs {
		switch key {
		case "goclone.template.natted":
			if value == "true" {
				natted = true
			}
		case "goclone.template.noRouter":
			if value == "true" {
				noRouter = true
			}
		case "goclone.template.competitionPod":
			if value == "true" {
				competitionPod = true
			}
		case "goclone.template.adminOnly":
			if value == "true" {
				adminOnly = true
			}
		}
	}

	vms, err := GetVMsInResourcePool(rp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VMs in resource pool"))
		return Template{}, err
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
			router, err = CreateRouter(rp.Reference(), datastore.Reference(), templateFolder, natted, name)
			vms = append(vms, *router)
		}
	}

    vmList := []vm.VM{}
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VM guest OS"))
		return Template{}, err
	}

	for _, v := range vms {
		vmObj := object.NewVirtualMachine(vSphereClient.client, v.Reference())
		vmName, err := vmObj.ObjectName(vSphereClient.ctx)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM name"))
			return Template{}, err
		}

        username := ""
        password := ""
        isHidden := ""
        attrs, err := GetAllAttributes(v.Reference())
        for key, value := range attrs {
            switch key {
            case "goclone.vm.username":
                username = value
            case "goclone.vm.password":
                password = value
            case "goclone.vm.isHidden":
                isHidden = value
            }
        }
        newVM := vm.VM{
            Name: vmName,
            Ref: v.Reference(),
            Ctx: &vSphereClient.ctx,
            Client: vSphereClient.client,
            Username: username,
            Password: password,
            IsRouter: strings.Contains(vmName, "PodRouter"),
            IsHidden: strings.Contains(strings.ToLower(isHidden), "true"),
            GuestOS: v.Config.GuestFullName,
        }
        vmList = append(vmList, newVM)
        fmt.Println(newVM.String())
	}


	err = CreateSnapshot(vms, "SnapshotForCloning")
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating snapshot"))
		return Template{}, err
	}

	template := Template{
		Name:           name,
		SourceRP:       rp,
		VMs:            vmList,
		Natted:         natted,
		AdminOnly:      adminOnly,
		CompetitionPod: competitionPod,
		NoRouter:       noRouter,
		WanPG:          pg,
	}

	return template, nil
}
