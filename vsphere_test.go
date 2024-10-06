package main

import (
	"fmt"
	"testing"

    "goclone/vm"

	"github.com/stretchr/testify/assert"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

func TestPods(t *testing.T) {
    pods, err := GetAllPods()
    if err != nil {
        t.Error(err)
    }
    assert.NotEmpty(t, pods)
}

func TestGetChildResourcePools(t *testing.T) {
    resourcePools, err := GetChildResourcePools(mainConfig.VCenterConfig.PresetTemplateResourcePool)
    if err != nil {
        t.Error(err)
    }
    assert.NotEmpty(t, resourcePools)
}

func TestGetVMsInResourcePool(t *testing.T) {
    childRPs, err := GetChildResourcePools(mainConfig.VCenterConfig.PresetTemplateResourcePool)
    if err != nil {
        t.Error(err)
    }

    vms := []mo.VirtualMachine{}
    for _, rp := range childRPs {
        vms, _ := GetVMsInResourcePool(rp.Reference())
        if vms == nil {
            continue
        }
        vms = append(vms, vms...)
    }
    assert.NotEmpty(t, vms)
}

func TestVMObjects(t *testing.T) {
    rp, err := GetResourcePool(mainConfig.VCenterConfig.PresetTemplateResourcePool)
    if err != nil {
        t.Error(err)
    }

    vms, err := GetVMsInResourcePool(rp.Reference())

    for _, v := range vms {
        vmObj := object.NewVirtualMachine(vSphereClient.client, v.Reference())
        vmName, err := vmObj.ObjectName(vSphereClient.ctx)
        if err != nil {
            t.Error(err)
        }
        newVM := vm.VM{
            Name: vmName,
            Ref: v.Reference(),
            Client: vSphereClient.client,
            Ctx: &vSphereClient.ctx,
            IsRouter: false,
            IsHidden: false,
            GuestOS: "Ubuntu 22.04",
        }

        resString := fmt.Sprintf("VM: %s\nUsername: %s\nPassword: %s\nIs Router: %v\nIs Hidden: %v\nGuest OS: %s\n", newVM.Name, newVM.Username, newVM.Password, newVM.IsRouter, newVM.IsHidden, newVM.GuestOS)
        assert.Equal(t, resString, newVM.String())
    }

}
