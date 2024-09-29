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
	VMs            []mo.VirtualMachine
	VMObjects      []object.VirtualMachine
	Natted         bool
	NoRouter       bool
	CompetitionPod bool
	WanPG          *object.DistributedVirtualPortgroup
	VMsToHide      []*mo.VirtualMachine
	VMAddresses    map[string]string
	VMGuestOS      map[string]string
	VMUsername     map[string]string
	VMPassword     map[string]string
	VMDomain       map[string]string
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
	err = pc.Retrieve(vSphereClient.ctx, trp.ResourcePool, []string{"name"}, &rps)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to collect references for preset templates")
	}

	for _, rp := range rps {
		tagsOnTmpl, err := GetTagsFromObject(rp.Reference())
		if err != nil {
			log.Println(errors.Wrap(err, "Error getting tags"))
			return nil, err
		}
		tagNames := []string{}
		for _, tag := range tagsOnTmpl {
			tagNames = append(tagNames, tag.Name)
		}

		if !isAdm && slices.Contains(tagNames, "AdminOnly") {
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

	templateSubfolderRefs, err := templateFolder.Children(vSphereClient.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to find template sub-folders")
	}

	pc := property.DefaultCollector(vSphereClient.client)

	for _, subfolderRef := range templateSubfolderRefs {
		var subfolder []mo.Folder
		if subfolderRef.Reference().Type != templateFolder.Reference().Type {
			continue
		}

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

func vSphereGetPods(owner string) ([]Pod, error) {
	var pods []Pod

	ownerPods, err := finder.ResourcePoolList(vSphereClient.ctx, fmt.Sprintf("*_%s", owner)) // hard coded based on our naming scheme

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

	var vmClonesMo []mo.VirtualMachine
	var router *mo.VirtualMachine
	for _, vm := range vmClones {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		var vm mo.VirtualMachine

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

	var routerPG *object.DistributedVirtualPortgroup
	if templateMap[sourceRP].CompetitionPod {
		routerPG = competitionPG
	} else {
		routerPG = templateMap[sourceRP].WanPG
	}

	if !templateMap[sourceRP].NoRouter {
		err = ConfigRouter(pg.Reference(), routerPG.Reference(), router, pgStr)
		if err != nil {
			log.Println(errors.Wrap(err, "Error cloning router"))
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

	eg := errgroup.Group{}
	for _, vm := range vmClonesMo {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		vmName, err := vmObj.ObjectName(vSphereClient.ctx)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM name"))
			return err
		}

		username := templateMap[sourceRP].VMUsername[vmName]
		password := templateMap[sourceRP].VMPassword[vmName]
		domain := templateMap[sourceRP].VMDomain[vmName]
		if username == "" || password == "" {
			continue
		}

		auth := types.NamePasswordAuthentication{
			Username: username,
			Password: password,
		}

		eg.Go(func() error {
			fmt.Println("Changing hostname", vmName, username, password)
			return ChangeHostname(sourceRP, &vm, vmName, domain, auth)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	SnapshotVMs(vmClonesMo, "Base")
	permission := types.Permission{
		Principal: strings.Join([]string{mainConfig.Domain, username}, "\\"),
		RoleId:    cloneRole.RoleId,
		Propagate: true,
	}
	AssignPermissionToObjects(&permission, []types.ManagedObjectReference{newFolder.Reference()})

	HideVMs(templateMap[sourceRP].VMsToHide, vmClonesMo, username)

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

	router, err := CreateRouter(targetRP.Reference(), datastore.Reference(), newFolder, natted, podName)
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
	tagsOnTmpl, err := GetTagsFromObject(rp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tags"))
		return Template{}, err
	}

	natted := false
	noRouter := false
	competitionPod := false
	pg := wanPG
	for _, tag := range tagsOnTmpl {
		switch tag.Name {
		case "CompetitionPod":
			competitionPod = true
		case "NoRouter":
			noRouter = true
		case "natted":
			natted = true
		}
	}

	vms, err := GetVMsInResourcePool(rp.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VMs in resource pool"))
		return Template{}, err
	}

	guestOSMap, err := GetVMGuestOS(vms)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VM guest OS"))
		return Template{}, err
	}

	usernameMap := make(map[string]string)
	passwordMap := make(map[string]string)
	domainMap := make(map[string]string)
	for _, vm := range vms {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		vmName, err := vmObj.ObjectName(vSphereClient.ctx)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM name"))
			return Template{}, err
		}
		username, err := GetVMAttribute(&vm, usernameKeyID)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM username"))
			usernameMap[vmName] = ""
			passwordMap[vmName] = ""
			domainMap[vmName] = ""
			continue
		}
		password, err := GetVMAttribute(&vm, passwordKeyID)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM password"))
			usernameMap[vmName] = ""
			passwordMap[vmName] = ""
			domainMap[vmName] = ""
			continue
		}
		fmt.Println("username: ", username)
		fmt.Println("password: ", password)
		usernameMap[vmName] = username
		passwordMap[vmName] = password
		domainMap[vmName] = ""
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

	err = CreateSnapshot(vms, "SnapshotForCloning")
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating snapshot"))
		return Template{}, err
	}

	vmsToHide, err := GetVMsToHide(vms)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting VMs to hide"))
		return Template{}, err
	}

	template := Template{
		Name:           name,
		SourceRP:       rp,
		VMs:            vms,
		Natted:         natted,
		CompetitionPod: competitionPod,
		NoRouter:       noRouter,
		WanPG:          pg,
		VMsToHide:      vmsToHide,
		VMGuestOS:      guestOSMap,
		VMUsername:     usernameMap,
		VMPassword:     passwordMap,
		VMDomain:       domainMap,
	}

	return template, nil
}

func GetVMGuestOS(vms []mo.VirtualMachine) (map[string]string, error) {
	var vmGuestOS = make(map[string]string)
	for _, vm := range vms {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		vmName, err := vmObj.ObjectName(vSphereClient.ctx)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Error getting VM name"))
			return nil, err
		}
		vmGuestOS[vmName] = strings.ToLower(vm.Config.GuestFullName)
	}
	return vmGuestOS, nil
}
