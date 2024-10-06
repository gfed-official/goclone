package main

import (
	"fmt"
	"log"
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
        vmList, err := GetVMsInResourcePool(rp.Reference())
        if err != nil {
            t.Error(err)
        }
        if vms == nil {
            continue
        }
        for _, vm := range vmList {
            vmObj := object.NewVirtualMachine(vSphereClient.client, vm.Reference())
            vmName, err := vmObj.ObjectName(vSphereClient.ctx)
            if err != nil {
                t.Error(err)
            }
            log.Println(vmName)
        }
        vms = append(vms, vmList...)
    }

    assert.NotEmpty(t, vms)
}

func TestVMObjects(t *testing.T) {
    rp, err := GetResourcePool(mainConfig.VCenterConfig.PresetTemplateResourcePool)
    if err != nil {
        t.Error(err)
    }


    vmList := []vm.VM{}
    vms, err := GetVMsInResourcePool(rp.Reference())
    for _, v := range vms {
        vmObj := object.NewVirtualMachine(vSphereClient.client, v.Reference())
        vmName, err := vmObj.ObjectName(vSphereClient.ctx)
        if err != nil {
            t.Error(err)
        }
        log.Println(vmName)
        newVM := vm.VM{
            Name: vmName,
            Ref: v.Reference(),
            Client: vSphereClient.client,
            Ctx: &vSphereClient.ctx,
            IsRouter: false,
            IsHidden: false,
            GuestOS: "Test",
        }

        resString := fmt.Sprintf("VM: %s\nUsername: %s\nPassword: %s\nIs Router: %v\nIs Hidden: %v\nGuest OS: %s\n", newVM.Name, newVM.Username, newVM.Password, newVM.IsRouter, newVM.IsHidden, newVM.GuestOS)
        assert.Equal(t, resString, newVM.String())
        vmList = append(vmList, newVM)
    }

    assert.NotEmpty(t, vmList)
}
