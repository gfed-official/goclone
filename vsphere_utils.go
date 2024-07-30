package main

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
)

func CreateTag(name string) (tags.Tag, error) {
    fmt.Println(vSphereClient.restClient.Valid())
    m := tags.NewManager(vSphereClient.restClient)
    tag := tags.Tag{
        Name: name,
        Description: "Tag created by Kamino",
        CategoryID: "CloneOnDemand",
    }
    _, err := m.CreateTag(vSphereClient.ctx, &tag)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error creating tag"))
        return tags.Tag{}, err
    }
    return tag, nil
}

func GetTagByName(name string) (tags.Tag, error) {
    m := tags.NewManager(vSphereClient.restClient)
    tag, err := m.GetTag(vSphereClient.ctx, name)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error getting tag by name"))
        return tags.Tag{}, err
    }
    return *tag, nil
}

func CreatePortGroup(name string, vlanID int32) error {
    dvs, err := finder.Network(vSphereClient.ctx, tomlConf.MainDistributedSwitch)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error finding distributed switch"))
        return err
    }

    dvsObj := dvs.(*object.DistributedVirtualSwitch)
    spec := types.DVPortgroupConfigSpec{
        Name: name,
        Type: string(types.DistributedVirtualPortgroupPortgroupTypeEarlyBinding),
        NumPorts: 128,
        DefaultPortConfig: &types.VMwareDVSPortSetting{
            Vlan: &types.VmwareDistributedVirtualSwitchVlanIdSpec{
                VlanId: vlanID,
            },
        },
    }
    task, err := dvsObj.AddPortgroup(vSphereClient.ctx, []types.DVPortgroupConfigSpec{spec})
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error adding portgroup"))
        return err
    }

    err = task.Wait(vSphereClient.ctx)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error waiting for task"))
        return err
    }

    log.Println("Portgroup created successfully")
    return nil
}

func AssignTagToPortGroup(tag tags.Tag, name string) error {
    m := tags.NewManager(vSphereClient.restClient)
    entity := types.ManagedObjectReference{
        Type: "DistributedVirtualPortgroup",
        Value: name,
    }
    err := m.AttachTag(vSphereClient.ctx, tag.ID, entity)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error assigning tag"))
        return err
    }
    return nil
}
