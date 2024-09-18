package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func CreateTag(name string) (tags.Tag, error) {
	tag := tags.Tag{
		Name:        name,
		Description: "Tag created by Kamino",
		CategoryID:  "CloneOnDemand",
	}

	_, err := tagManager.CreateTag(vSphereClient.ctx, &tag)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating tag"))
		return tags.Tag{}, err
	}

	tag, err = GetTagByName(name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tag by name"))
		return tags.Tag{}, err
	}
	return tag, nil
}

func GetTagByName(name string) (tags.Tag, error) {
	tag, err := tagManager.GetTag(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tag by name"))
		return tags.Tag{}, err
	}
	return *tag, nil
}

func CreatePortGroup(name string, vlanID int) (object.NetworkReference, error) {
	dvsObj := object.NewDistributedVirtualSwitch(vSphereClient.client, dvsMo.Reference())
	spec := types.DVPortgroupConfigSpec{
		Name:     name,
		Type:     string(types.DistributedVirtualPortgroupPortgroupTypeEarlyBinding),
		NumPorts: 128,
		DefaultPortConfig: &types.VMwareDVSPortSetting{
			Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
				VlanId: int32(vlanID),
			},
		},
	}

	task, err := dvsObj.AddPortgroup(vSphereClient.ctx, []types.DVPortgroupConfigSpec{spec})
	if err != nil {
		log.Println(errors.Wrap(err, "Error adding portgroup"))
		return object.Network{}, err
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
		return object.Network{}, err
	}

	pgReference, err := finder.Network(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding portgroup"))
		return object.Network{}, err
	}

	return pgReference, nil
}

func GetPortGroup(name string) (object.NetworkReference, error) {
	pg, err := finder.Network(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding portgroup"))
		return object.Network{}, err
	}

	return pg, nil
}

func AssignTagToObject(tag tags.Tag, entity mo.Reference) error {
	err := tagManager.AttachTag(vSphereClient.ctx, tag.ID, entity)
	if err != nil {
		log.Println(errors.Wrap(err, "Error assigning tag"))
		return err
	}
	return nil
}

func GetTagsFromObject(entity types.ManagedObjectReference) ([]tags.Tag, error) {
	tagList, err := tagManager.GetAttachedTags(vSphereClient.ctx, entity)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting attached objects from tags"))
		return []tags.Tag{}, err
	}

	return tagList, nil
}

func CreateResourcePool(name string, compPod bool) (types.ManagedObjectReference, error) {
	rpSpec := types.ResourceConfigSpec{
		CpuAllocation: types.ResourceAllocationInfo{
			Shares:                &types.SharesInfo{Level: types.SharesLevelNormal},
			Reservation:           types.NewInt64(0),
			Limit:                 types.NewInt64(-1),
			ExpandableReservation: types.NewBool(true),
		},
		MemoryAllocation: types.ResourceAllocationInfo{
			Shares:                &types.SharesInfo{Level: types.SharesLevelNormal},
			Reservation:           types.NewInt64(0),
			Limit:                 types.NewInt64(-1),
			ExpandableReservation: types.NewBool(true),
		},
	}

	rp, err := finder.ResourcePool(vSphereClient.ctx, name)
	if err == nil {
		log.Println("Resource pool already exists")
		return rp.Reference(), nil
	}

    rpDest := targetResourcePool
    if compPod {
        rpDest = competitionResourcePool
    }

	child, err := rpDest.Create(vSphereClient.ctx, name, rpSpec)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating resource pool"))
	}

	tag, err := tagManager.GetTag(vSphereClient.ctx, name)
	err = AssignTagToObject(*tag, child.Reference())

	return child.Reference(), nil
}

func GetResourcePool(name string) (*object.ResourcePool, error) {
	rpObj, err := finder.ResourcePool(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool"))
		return &object.ResourcePool{}, err
	}

	return rpObj, nil
}

func CreateVMFolder(name string) (*object.Folder, error) {
	vmFolder, err := finder.Folder(vSphereClient.ctx, "vm")
	if err != nil {
		log.Println(errors.Wrap(err, "Cannot find vm folder"))
	}

	newFolder, err := vmFolder.CreateFolder(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to create folder"))
	}

	tag, err := tagManager.GetTag(vSphereClient.ctx, name)
	err = AssignTagToObject(*tag, newFolder.Reference())

	return newFolder, nil
}

func GetVMsInResourcePool(rp types.ManagedObjectReference) ([]mo.VirtualMachine, error) {
	rpData := mo.ResourcePool{}
	pc := property.DefaultCollector(vSphereClient.client)
	err := pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{rp}, []string{"vm"}, &rpData)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to retrieve VMs from resource pool"))
		return nil, err
	}

	var vms []mo.VirtualMachine
	err = pc.Retrieve(vSphereClient.ctx, rpData.Vm, []string{"name"}, &vms)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to get references for Virtual Machines"))
		return nil, err
	}

	return vms, nil
}

func GetVMsToHide(vms []mo.VirtualMachine) ([]*mo.VirtualMachine, error) {
	var wg sync.WaitGroup
	var hiddenVMs []*mo.VirtualMachine
	for _, vm := range vms {
		wg.Add(1)
		go IsHidden(&wg, &vm, &hiddenVMs)
	}
	wg.Wait()
	return hiddenVMs, nil
}

func IsHidden(wg *sync.WaitGroup, vm *mo.VirtualMachine, hiddenVMs *[]*mo.VirtualMachine) {
	defer wg.Done()
	tags, err := GetTagsFromObject(vm.Reference())
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to get tags"))
	}

	for _, tag := range tags {
		if tag.Name == "hidden" {
			*hiddenVMs = append(*hiddenVMs, vm)
		}
	}
}

func HideVMs(vmsToHide []*mo.VirtualMachine, clonedVMs []mo.VirtualMachine, username string) {
	var wg sync.WaitGroup
	for _, vm := range vmsToHide {
		for _, clonedVM := range clonedVMs {
			if strings.Contains(clonedVM.Name, vm.Name) {
				wg.Add(1)
				go HideVM(&wg, &clonedVM, username)
			}
		}
	}
	wg.Wait()
}

func HideVM(wg *sync.WaitGroup, vm *mo.VirtualMachine, username string) {
	defer wg.Done()
	permission := types.Permission{
		Principal: strings.Join([]string{mainConfig.Domain, username}, "\\"),
		RoleId:    noAccessRole.RoleId,
		Propagate: true,
	}
	err := AssignPermissionToObjects(&permission, []types.ManagedObjectReference{vm.Reference()})
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to assign permission to VM"))
	}
}

func CreateSnapshot(vms []mo.VirtualMachine, name string) error {
	for _, vm := range vms {
		vm := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		_, err := vm.FindSnapshot(vSphereClient.ctx, name)
		if err == nil {
			continue
		}

		task, err := vm.CreateSnapshot(vSphereClient.ctx, name, "", false, false)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to create snapshot"))
			return err
		}

		err = task.Wait(vSphereClient.ctx)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to wait for task"))
			return err
		}

		log.Printf("Snapshot created for VM %s\n", vm.Name())
	}

	return nil
}

func GetSnapshot(vms []mo.VirtualMachine, name string) []*object.VirtualMachine {
	var vmsWithoutSnapshot []*object.VirtualMachine
	for _, vm := range vms {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		_, err := vmObj.FindSnapshot(vSphereClient.ctx, name)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to find snapshot"))
			vmsWithoutSnapshot = append(vmsWithoutSnapshot, vmObj)
		}
	}

	return vmsWithoutSnapshot
}

func GetSnapshotRef(vm mo.VirtualMachine, name string) types.ManagedObjectReference {
	vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
	snapshot, err := vmObj.FindSnapshot(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to find snapshot"))
		return types.ManagedObjectReference{}
	}

	return snapshot.Reference()
}

func CloneVMs(vms []mo.VirtualMachine, folder *object.Folder, resourcePool, ds, pg types.ManagedObjectReference, pgNum string) {
	var wg sync.WaitGroup
	for _, vm := range vms {
		var configSpec types.VirtualMachineConfigSpec
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())

		configSpec, err := ConfigureVMNetwork(vmObj, pg)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to configure VM network"))
		}

		snapshotRef := GetSnapshotRef(vm, "SnapshotForCloning")
		spec := types.VirtualMachineCloneSpec{
			Snapshot: &snapshotRef,
			Location: types.VirtualMachineRelocateSpec{
				DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsCreateNewChildDiskBacking),
				Datastore:    &ds,
				Pool:         &resourcePool,
			},
			Config: &configSpec,
		}

		vm.Name = strings.Join([]string{pgNum, vm.Name}, "-")
		folderObj := object.NewFolder(vSphereClient.client, folder.Reference())
		wg.Add(1)
		go CloneVM(&wg, vm, *folderObj, spec)
	}
	wg.Wait()
}

func CloneVMsFromTemplates(templates []mo.VirtualMachine, folder *object.Folder, resourcePool, ds, pg types.ManagedObjectReference, pgNum string) {
	var wg sync.WaitGroup
	for _, template := range templates {
		vmObj := object.NewVirtualMachine(vSphereClient.client, template.Reference())
		configSpec, err := ConfigureVMNetwork(vmObj, pg)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to configure VM network"))
		}

		spec := types.VirtualMachineCloneSpec{
			Location: types.VirtualMachineRelocateSpec{
				Datastore: &ds,
				Pool:      &resourcePool,
			},
			Config: &configSpec,
		}

		folderObj := object.NewFolder(vSphereClient.client, folder.Reference())
		wg.Add(1)
		go CloneVM(&wg, template, *folderObj, spec)
	}
	wg.Wait()
}

func CloneVM(wg *sync.WaitGroup, vm mo.VirtualMachine, folder object.Folder, spec types.VirtualMachineCloneSpec) {
	defer wg.Done()

	vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
	task, err := vmObj.Clone(vSphereClient.ctx, &folder, vm.Name, spec)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to clone VM"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to wait for task"))
	}
}

func ConfigureVMNetwork(vmObj *object.VirtualMachine, pg types.ManagedObjectReference) (types.VirtualMachineConfigSpec, error) {
	var configSpec types.VirtualMachineConfigSpec
	devices, err := vmObj.Device(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to get devices"))
		return types.VirtualMachineConfigSpec{}, err
	}
	for _, device := range devices {
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
			deviceChange := ConfigureNIC(device, pg)
			configSpec = types.VirtualMachineConfigSpec{
				DeviceChange: []types.BaseVirtualDeviceConfigSpec{deviceChange},
			}
		}
	}
	return configSpec, nil
}

func CreateRouter(srcRP, ds types.ManagedObjectReference, folder *object.Folder, natted bool, rpName string) (*mo.VirtualMachine, error) {
	var templateName, cloneName string

	if natted {
		templateName = vCenterConfig.NattedRouterPath
		cloneName = strings.Join([]string{rpName, "Natted-PodRouter"}, "-")
	} else {
		templateName = vCenterConfig.RouterPath
		cloneName = strings.Join([]string{rpName, "PodRouter"}, "-")
	}

	template, err := finder.VirtualMachine(vSphereClient.ctx, templateName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding template"))
		return &mo.VirtualMachine{}, err
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: types.VirtualMachineRelocateSpec{
			Datastore: &ds,
			Pool:      &srcRP,
		},
	}

	task, err := template.Clone(vSphereClient.ctx, folder, cloneName, cloneSpec)
	if err != nil {
		log.Println(errors.Wrap(err, "Error cloning template"))
		return &mo.VirtualMachine{}, err
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
		return &mo.VirtualMachine{}, err
	}

	routerObj, err := finder.VirtualMachine(vSphereClient.ctx, cloneName)
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding router"))
		return &mo.VirtualMachine{}, err
	}

	routerMo := mo.VirtualMachine{}
	pc := property.DefaultCollector(vSphereClient.client)
	err = pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{routerObj.Reference()}, []string{"name"}, &routerMo)
	if err != nil {
		log.Println(errors.Wrap(err, "Error retrieving router"))
		return &mo.VirtualMachine{}, err
	}

	return &routerMo, nil
}

func GetRouter(srcRPRef types.ManagedObjectReference) (*mo.VirtualMachine, error) {
	router, err := finder.VirtualMachine(vSphereClient.ctx, "PodRouter")
	if err != nil {
		log.Println(errors.Wrap(err, "Error finding router"))
		return &mo.VirtualMachine{}, err
	}

	routerMo := mo.VirtualMachine{}
	pc := property.DefaultCollector(vSphereClient.client)
	err = pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{router.Reference()}, []string{"name"}, &routerMo)
	if err != nil {
		log.Println(errors.Wrap(err, "Error retrieving router"))
		return &mo.VirtualMachine{}, err
	}

	return &routerMo, nil
}

func ConfigRouter(pg, wanPG types.ManagedObjectReference, router *mo.VirtualMachine, pgStr string) error {
	templateObj := object.NewVirtualMachine(vSphereClient.client, router.Reference())
	devices, err := templateObj.Device(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting devices"))
		return err
	}

	var deviceList []types.BaseVirtualDeviceConfigSpec
	for _, device := range devices {
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
			deviceChange := ConfigureNIC(device, wanPG)
			deviceList = append(deviceList, deviceChange)
		}
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 2" {
			deviceChange := ConfigureNIC(device, pg)
			deviceList = append(deviceList, deviceChange)
		}
	}

	configSpec := types.VirtualMachineConfigSpec{
		DeviceChange: deviceList,
	}

	_, err = templateObj.Reconfigure(vSphereClient.ctx, configSpec)
	if err != nil {
		log.Println(errors.Wrap(err, "Error reconfiguring router"))
		return err
	}

	task, err := templateObj.PowerOn(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error powering on router"))
		return err
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
		return err
	}

	return nil
}

func ConfigureNIC(nic types.BaseVirtualDevice, pg types.ManagedObjectReference) *types.VirtualDeviceConfigSpec {
	nic.GetVirtualDevice().Backing = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
		Port: types.DistributedVirtualSwitchPortConnection{
			PortgroupKey: pg.Reference().Value,
			SwitchUuid:   dvsMo.Uuid,
		},
	}
	deviceChange := types.VirtualDeviceConfigSpec{
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
		Device:    nic,
	}
	return &deviceChange
}

func DestroyFolder(folderObj *object.Folder) {
	vms, err := folderObj.Children(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting children"))
	}

	for _, vm := range vms {
		err = PowerOffVM(vm.(*object.VirtualMachine))
		if err != nil {
			log.Println(errors.Wrap(err, "Error powering off VM"))
		}
	}

	task, err := folderObj.Destroy(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying folder"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}
}

func DestroyResourcePool(rpObj *object.ResourcePool) {
	task, err := rpObj.Destroy(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying resource pool"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}
}

func DestroyPortGroup(pg types.ManagedObjectReference) error {
	pgObj := object.NewNetwork(vSphereClient.client, pg.Reference())
	task, err := pgObj.Destroy(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying port group"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}

	return nil
}

func DestroyTag(name string) error {
	tag, err := GetTagByName(name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tag by name"))
		return err
	}

	err = tagManager.DeleteTag(vSphereClient.ctx, &tag)
	if err != nil {
		log.Println(errors.Wrap(err, "Error deleting tag"))
		return err
	}
	return nil
}

func RunProgramOnVM(vm *mo.VirtualMachine, program types.GuestProgramSpec, auth types.NamePasswordAuthentication) error {
	pc := property.DefaultCollector(vSphereClient.client)

	timeout := time.After(2 * time.Minute)
	ticker := time.Tick(2 * time.Second)
	for {
		select {
		case <-timeout:
			return errors.New("Timeout waiting for VM to be ready")
		case <-ticker:
			err := pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{vm.Reference()}, []string{"guest"}, vm)
			if err != nil {
				log.Println(errors.Wrap(err, "Error retrieving router"))
				return err
			}
			if vm.Guest != nil && vm.Guest.ToolsRunningStatus == "guestToolsRunning" {
				gom := guest.NewOperationsManager(vSphereClient.client, vm.Reference())

				procMan, err := gom.ProcessManager(vSphereClient.ctx)
				if err != nil {
					log.Println(errors.Wrap(err, "Error getting process manager"))
					return err
				}

				_, err = procMan.StartProgram(vSphereClient.ctx, &auth, &program)
				if err != nil {
					log.Println(errors.Wrap(err, "Error starting program"))
					return err
				}

				return nil
			}
		}
	}
}

func PowerOffVM(vm *object.VirtualMachine) error {
	task, err := vm.PowerOff(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error powering off VM"))
		return err
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
		return err
	}

	return nil
}

func SnapshotVMs(vms []mo.VirtualMachine, name string) {
	var wg sync.WaitGroup
	for _, vm := range vms {
		wg.Add(1)
		go SnapshotVM(&wg, &vm, name)
	}
	wg.Wait()
}

func SnapshotVM(wg *sync.WaitGroup, vm *mo.VirtualMachine, name string) {
	defer wg.Done()
	vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
	task, err := vmObj.CreateSnapshot(vSphereClient.ctx, name, "", false, false)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating snapshot"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}
}

func AssignPermissionToObjects(permission *types.Permission, object []types.ManagedObjectReference) error {
	for _, obj := range object {
		err := authManager.SetEntityPermissions(vSphereClient.ctx, obj, []types.Permission{*permission})
		if err != nil {
			log.Println(errors.Wrap(err, "Error setting entity permissions"))
			return err
		}
	}

	return nil
}

func GetChildResourcePools(resourcePool string) ([]*object.ResourcePool, error) {
	templateParentPool, err := finder.ResourcePool(vSphereClient.ctx, resourcePool)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool list"))
		return nil, err
	}

	rpData := mo.ResourcePool{}
	poolObj := object.NewResourcePool(vSphereClient.client, templateParentPool.Reference())
	err = poolObj.Properties(vSphereClient.ctx, templateParentPool.Reference(), []string{"resourcePool"}, &rpData)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool children"))
		return nil, err
	}

    var rpList []*object.ResourcePool
	for _, rp := range rpData.ResourcePool {
		rpObj := object.NewResourcePool(vSphereClient.client, rp.Reference())
        rpList = append(rpList, rpObj)
	}

    return rpList, nil
}
