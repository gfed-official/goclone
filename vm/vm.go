package vm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VM struct {
    Ref mo.Reference
    Name string
    Username string
    Password string
    Ctx *context.Context
    Client *vim25.Client
    IsRouter bool
    IsHidden bool
    GuestOS string
}

func (vm *VM) String() string {
    return fmt.Sprintf("VM: %s\nUsername: %s\nPassword: %s\nIs Router: %v\nIs Hidden: %v\nGuest OS: %s\n", vm.Name, vm.Username, vm.Password, vm.IsRouter, vm.IsHidden, vm.GuestOS)
}

func (vm *VM) PowerOn() error {
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.PowerOn(*vm.Ctx)
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
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.PowerOff(*vm.Ctx)
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
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.CreateSnapshot(*vm.Ctx, name, "", false, false)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) RemoveSnapshot(name string) error {
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.RemoveSnapshot(*vm.Ctx, name, true, types.NewBool(true))
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
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.RevertToSnapshot(*vm.Ctx, name, true)
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
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    task, err := vmObj.Clone(*vm.Ctx, folder, vm.Name, *spec)
    if err != nil {
        fmt.Println(err)
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        fmt.Println(err)
        return err
    }
    return nil
}

func (vm *VM) ConfigureVMNetwork(portGroup *types.ManagedObjectReference, dvsMo mo.DistributedVirtualSwitch) (types.VirtualMachineConfigSpec, error) {
    if vm.IsRouter {
        return types.VirtualMachineConfigSpec{}, errors.New("Cannot configure VM networks for router")
    }

    var configSpec types.VirtualMachineConfigSpec
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    devices, err := vmObj.Device(*vm.Ctx)
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

func (vm *VM) ConfigureRouterNetworks(wanPortGroup *object.DistributedVirtualPortgroup, lanPortGroup *object.DistributedVirtualPortgroup, dvsMo mo.DistributedVirtualSwitch) error {
    if !vm.IsRouter {
        return errors.New("Cannot configure router networks for non-router")
    }

    var configSpec types.VirtualMachineConfigSpec
    vmObj := object.NewVirtualMachine(vm.Client, vm.Ref.Reference())
    devices, err := vmObj.Device(*vm.Ctx)
    if err != nil {
        return nil
    }
    for _, device := range devices {
        if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 1" {
            deviceChange := configureNIC(device, wanPortGroup.Reference(), dvsMo)
            configSpec = types.VirtualMachineConfigSpec{
                DeviceChange: []types.BaseVirtualDeviceConfigSpec{deviceChange},
            }
        } else if device.GetVirtualDevice().DeviceInfo.GetDescription().Label == "Network adapter 2" {
            deviceChange := configureNIC(device, lanPortGroup.Reference(), dvsMo)
            configSpec.DeviceChange = append(configSpec.DeviceChange, deviceChange)
        }
    }

    task, err := vmObj.Reconfigure(*vm.Ctx, configSpec)
    if err != nil {
        return err
    }
    err = task.Wait(*vm.Ctx)
    if err != nil {
        return err
    }
    return nil
}

func (vm *VM) GetGuestOS() string {
    return vm.Ref.(*mo.VirtualMachine).Config.GuestFullName
}

func (vm *VM) RunProgramOnVM(program types.GuestProgramSpec, auth types.NamePasswordAuthentication) error {
    pc := property.DefaultCollector(vm.Client)

    timeout := time.After(2 * time.Minute)
    ticker := time.Tick(2 * time.Second)
    retries := 0
    for {
        select {
        case <-timeout:
            return errors.New("Timeout waiting for VM to be ready")
        case <-ticker:
            vmMo := mo.VirtualMachine{}
            err := pc.Retrieve(*vm.Ctx, []types.ManagedObjectReference{vm.Ref.Reference()}, []string{"guest"}, &vmMo)
            if err != nil {
                return err
            }
            if vmMo.Guest != nil && vmMo.Guest.ToolsRunningStatus == "guestToolsRunning" {
                gom := guest.NewOperationsManager(vm.Client, vm.Ref.Reference())

                procMan, err := gom.ProcessManager(*vm.Ctx)
                if err != nil {
                    return err
                }

                _, err = procMan.StartProgram(*vm.Ctx, &auth, &program)
                if err != nil {
                    if retries < 2 && strings.Contains(err.Error(), "Failed to authenticate") {
                        retries++
                        time.Sleep(time.Second * 20)
                        continue
                    }
                    return err
                }
                return nil
            }
        }
    }
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

