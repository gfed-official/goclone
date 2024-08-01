package main

import (
	"log"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func CreateTag(name string) (tags.Tag, error) {
	m := tags.NewManager(vSphereClient.restClient)
	tag := tags.Tag{
		Name:        name,
		Description: "Tag created by Kamino",
		CategoryID:  "CloneOnDemand",
	}

	_, err := m.CreateTag(vSphereClient.ctx, &tag)
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
	m := tags.NewManager(vSphereClient.restClient)
	tag, err := m.GetTag(vSphereClient.ctx, name)
	if err != nil {
		log.Println(errors.Wrap(err, "Error getting tag by name"))
		return tags.Tag{}, err
	}
	return *tag, nil
}

func CreatePortGroup(name string, vlanID int32) (types.ManagedObjectReference, error) {
	dvs, err := finder.Network(vSphereClient.ctx, tomlConf.MainDistributedSwitch)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding distributed switch"))
		return types.ManagedObjectReference{}, err
	}

	dvsObj := dvs.(*object.DistributedVirtualSwitch)
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
		return types.ManagedObjectReference{}, err
	}

	err = task.Wait(vSphereClient.ctx)
	if err != nil {
		log.Println(errors.Wrap(err, "Error waiting for task"))
		return types.ManagedObjectReference{}, err
	}

	pgReference := types.ManagedObjectReference{
		Type:  "DistributedVirtualPortgroup",
		Value: name,
	}
	return pgReference, nil
}

func AssignTagToObject(tag tags.Tag, entity mo.Reference) error {
	m := tags.NewManager(vSphereClient.restClient)

	err := m.AttachTag(vSphereClient.ctx, tag.ID, entity)
	if err != nil {
		log.Println(errors.Wrap(err, "Error assigning tag"))
		return err
	}
	return nil
}

func GetTagsFromObject(entity types.ManagedObjectReference) ([]tags.Tag, error) {
	m := tags.NewManager(vSphereClient.restClient)

	tagList, err := m.GetAttachedTags(vSphereClient.ctx, entity)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error getting attached objects from tags"))
		return []tags.Tag{}, err
	}

	return tagList, nil
}

func GetResourcePool(name string) (types.ManagedObjectReference, error) {
	rpRef, err := finder.ResourcePool(vSphereClient.ctx, name)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error getting resource pool"))
		return types.ManagedObjectReference{}, err
	}

	return rpRef.Reference(), nil
}

func CreateVMFolder(name string) (types.ManagedObjectReference, error) {
	vmFolder, err := finder.Folder(vSphereClient.ctx, "vm")
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Cannot find vm folder"))
	}

	newFolder, err := vmFolder.CreateFolder(vSphereClient.ctx, name)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Failed to create folder"))
	}

	return newFolder.Reference(), nil
}

func GetVMsInResourcePool(rp types.ManagedObjectReference) ([]*object.VirtualMachine, error) {
	rpData := mo.ResourcePool{}
	pc := property.DefaultCollector(vSphereClient.client)
	err := pc.Retrieve(vSphereClient.ctx, []types.ManagedObjectReference{rp}, []string{"vm"}, &rpData)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Failed to retrieve VMs from resource pool"))
		return nil, err
	}

	var vms []mo.VirtualMachine
	err = pc.Retrieve(vSphereClient.ctx, rpData.Vm, []string{"name"}, &vms)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Failed to get references for Virtual Machines"))
		return nil, err
	}

	vmObjs := []*object.VirtualMachine{}

	for _, vm := range vms {
		vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
		vmObjs = append(vmObjs, vmObj)
	}

	return vmObjs, nil
}

func CreateSnapshot(vms []*object.VirtualMachine, name string) error {
	for _, vm := range vms {
		task, err := vm.CreateSnapshot(vSphereClient.ctx, name, "", false, false)
		if err != nil {
			log.Fatalln(errors.Wrap(err, "Failed to create snapshot"))
			return err
		}

		err = task.Wait(vSphereClient.ctx)
		if err != nil {
			log.Fatalln(errors.Wrap(err, "Failed to wait for task"))
			return err
		}

		log.Printf("Snapshot created for VM %s\n", vm.Name())
	}

	return nil
}

func GetSnapshot(vms []*object.VirtualMachine, name string) []*object.VirtualMachine {
	var vmsWithoutSnapshot []*object.VirtualMachine
	for _, vm := range vms {
		_, err := vm.FindSnapshot(vSphereClient.ctx, name)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to find snapshot"))
			vmsWithoutSnapshot = append(vmsWithoutSnapshot, vm)
		}
	}

	return vmsWithoutSnapshot
}

func CloneVMs(vms []*object.VirtualMachine, folder, resourcePool, ds types.ManagedObjectReference) error {
	for _, vm := range vms {
		spec := types.VirtualMachineInstantCloneSpec{
			Name: vm.Name() + "-clone",
			Location: types.VirtualMachineRelocateSpec{
				Datastore: &ds,
				Pool:      &resourcePool,
				Folder:    &folder,
			},
		}

		task, err := vm.InstantClone(vSphereClient.ctx, spec)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to clone VM"))
			return err
		}

		err = task.Wait(vSphereClient.ctx)
		if err != nil {
			log.Println(errors.Wrap(err, "Failed to wait for task"))
			return err
		}
	}
	return nil
}
