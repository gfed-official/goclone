package main

import (
	"log"
	"sync"

	"github.com/pkg/errors"
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

func CreatePortGroup(name string, vlanID int32) (object.NetworkReference, error) {
	dvsObj := object.NewDistributedVirtualSwitch(vSphereClient.client, dvsMo.Reference())
	spec := types.DVPortgroupConfigSpec{
		Name:     name,
		Type:     string(types.DistributedVirtualPortgroupPortgroupTypeEarlyBinding),
		NumPorts: 128,
		DefaultPortConfig: &types.VMwareDVSPortSetting{
			Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
				VlanId: vlanID,
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

func CreateResourcePool(name string) (types.ManagedObjectReference, error) {
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

	child, err := targetResourcePool.Create(vSphereClient.ctx, name, rpSpec)
	if err != nil {
		log.Println(errors.Wrap(err, "Error creating resource pool"))
	}

	tag, err := tagManager.GetTag(vSphereClient.ctx, name)
	err = AssignTagToObject(*tag, child.Reference())

	return child.Reference(), nil
}

func GetResourcePool(name string) (types.ManagedObjectReference, error) {
	rpRef, err := finder.ResourcePool(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting resource pool"))
		return types.ManagedObjectReference{}, err
	}

	return rpRef.Reference(), nil
}

func CreateVMFolder(name string) (types.ManagedObjectReference, error) {
	vmFolder, err := finder.Folder(vSphereClient.ctx, "vm")
	if err != nil {
		log.Println(errors.Wrap(err, "Cannot find vm folder"))
	}

	newFolder, err := vmFolder.CreateFolder(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to create folder"))
	}

	return newFolder.Reference(), nil
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

func CreateSnapshot(vms []*object.VirtualMachine, name string) error {
	for _, vm := range vms {
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

func CloneVMs(vms []mo.VirtualMachine, folder, resourcePool, ds, pg types.ManagedObjectReference) {
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

		folderObj := object.NewFolder(vSphereClient.client, folder)
		wg.Add(1)
		go CloneVM(&wg, vm, *folderObj, spec)
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

func ConfigureVMNetwork(vmObj *object.VirtualMachine, network types.ManagedObjectReference) (types.VirtualMachineConfigSpec, error) {
	var configSpec types.VirtualMachineConfigSpec
	devices, err := vmObj.Device(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Failed to get devices"))
		return types.VirtualMachineConfigSpec{}, err
	}
	for _, device := range devices {
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
			device.GetVirtualDevice().Backing = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
				Port: types.DistributedVirtualSwitchPortConnection{
					PortgroupKey: network.Reference().Value,
					SwitchUuid:   dvsMo.Uuid,
				},
			}
			configSpec = types.VirtualMachineConfigSpec{
				DeviceChange: []types.BaseVirtualDeviceConfigSpec{
					&types.VirtualDeviceConfigSpec{
						Operation: types.VirtualDeviceConfigSpecOperationEdit,
						Device:    device,
					},
				},
			}
		}
	}
	return configSpec, nil
}

func CreateRouter(srcRP, ds types.ManagedObjectReference, folder *object.Folder, natted bool) (*mo.VirtualMachine, error) {
	var templateName, cloneName string
	if natted {
		templateName = tomlConf.NattedRouterPath
		cloneName = "Natted-PodRouter"
	} else {
		templateName = tomlConf.RouterPath
		cloneName = "PodRouter"
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

func ConfigRouter(pg, wanPG types.ManagedObjectReference, router *mo.VirtualMachine) error {
	templateObj := object.NewVirtualMachine(vSphereClient.client, router.Reference())
	devices, err := templateObj.Device(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting devices"))
		return err
	}

	var deviceList []types.BaseVirtualDeviceConfigSpec
	for _, device := range devices {
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
			device.GetVirtualDevice().Backing = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
				Port: types.DistributedVirtualSwitchPortConnection{
					PortgroupKey: wanPG.Reference().Value,
					SwitchUuid:   dvsMo.Uuid,
				},
			}
			deviceChange := types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    device,
			}
			deviceList = append(deviceList, &deviceChange)
		}
		if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 2" {
			device.GetVirtualDevice().Backing = &types.VirtualEthernetCardDistributedVirtualPortBackingInfo{
				Port: types.DistributedVirtualSwitchPortConnection{
					PortgroupKey: pg.Reference().Value,
					SwitchUuid:   dvsMo.Uuid,
				},
			}
			deviceChange := types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    device,
			}
			deviceList = append(deviceList, &deviceChange)
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

	return nil
}

func DestroyFolder(wg *sync.WaitGroup, folder *types.ManagedObjectReference) {
	defer wg.Done()
	folderObj := object.NewFolder(vSphereClient.client, *folder)
	task, err := folderObj.Destroy(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying folder"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}
}

func DestroyVM(wg *sync.WaitGroup, vm *types.ManagedObjectReference) {
	defer wg.Done()
	vmObj := object.NewVirtualMachine(vSphereClient.client, *vm)
	task, err := vmObj.Destroy(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error destroying VM"))
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
	}
}

func DestroyResourcePool(wg *sync.WaitGroup, rp *types.ManagedObjectReference) {
	defer wg.Done()
	rpObj := object.NewResourcePool(vSphereClient.client, *rp)
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
