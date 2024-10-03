package vm

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VM struct {
    Ref mo.Reference
    Name string
    Username string
    Password string
    Ctx *context.Context
    IsRouter bool
    IsHidden bool
    GuestOS string
}

func (vm *VM) String() string {
    return fmt.Sprintf("VM: %s\nUsername: %s\nPassword: %s\nIs Router: %v\nIs Hidden: %v\nGuest OS: %s\n", vm.Name, vm.Username, vm.Password, vm.IsRouter, vm.IsHidden, vm.GuestOS)
}

func (vm *VM) PowerOn() error {
    fmt.Println("Powering on VM")
    task, err := vm.Ref.(*object.VirtualMachine).PowerOn(*vm.Ctx)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) PowerOff() error {
    task, err := vm.Ref.(*object.VirtualMachine).PowerOff(*vm.Ctx)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) SetSnapshot(name string) error {
    task, err := vm.Ref.(*object.VirtualMachine).CreateSnapshot(*vm.Ctx, name, "", false, false)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) RevertSnapshot(name string) error {
    fmt.Println("Reverting snapshot")
    task, err := vm.Ref.(*object.VirtualMachine).RevertToSnapshot(*vm.Ctx, name, true)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) CloneVM(wg *sync.WaitGroup, spec *types.VirtualMachineCloneSpec, folder *object.Folder) error {
    defer wg.Done()
    fmt.Println("Cloning VM")
    task, err := vm.Ref.(*object.VirtualMachine).Clone(*vm.Ctx, folder, vm.Name, *spec)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) ConfigureVMNetwork(portGroup *types.ManagedObjectReference, dvsMo mo.DistributedVirtualSwitch) (types.VirtualMachineConfigSpec, error) {
    if vm.IsRouter {
        return types.VirtualMachineConfigSpec{}, errors.New("Cannot configure VM networks for router")
    }

    var configSpec types.VirtualMachineConfigSpec
    devices, err := vm.Ref.(*object.VirtualMachine).Device(*vm.Ctx)
    if err != nil {
        return types.VirtualMachineConfigSpec{}, err
    }
    for _, device := range devices {
        if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
            deviceChange := configureNIC(device, *portGroup, dvsMo)
            configSpec = types.VirtualMachineConfigSpec{
                DeviceChange: []types.BaseVirtualDeviceConfigSpec{deviceChange},
            }
        }
    }
    return configSpec, nil
}

func (vm *VM) ConfigureRouterNetworks(wanPortGroup *types.ManagedObjectReference, lanPortGroup *types.ManagedObjectReference, dvsMo mo.DistributedVirtualSwitch) (types.VirtualMachineConfigSpec, error) {
    if !vm.IsRouter {
        return types.VirtualMachineConfigSpec{}, errors.New("Cannot configure router networks for non-router")
    }

    var configSpec types.VirtualMachineConfigSpec
    devices, err := vm.Ref.(*object.VirtualMachine).Device(*vm.Ctx)
    if err != nil {
        return types.VirtualMachineConfigSpec{}, err
    }
    for _, device := range devices {
        if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
            deviceChange := configureNIC(device, *wanPortGroup, dvsMo)
            configSpec = types.VirtualMachineConfigSpec{
                DeviceChange: []types.BaseVirtualDeviceConfigSpec{deviceChange},
            }
        } else if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 2" {
            deviceChange := configureNIC(device, *lanPortGroup, dvsMo)
            configSpec.DeviceChange = append(configSpec.DeviceChange, deviceChange)
        }
    }
    return configSpec, nil
}

func (vm *VM) GetGuestOS() string {
    return vm.Ref.(*mo.VirtualMachine).Config.GuestFullName
}

func configureNIC(nic types.BaseVirtualDevice, pg types.ManagedObjectReference, dvsMo mo.DistributedVirtualSwitch) *types.VirtualDeviceConfigSpec {
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
