package service_test

import (
	"os"
	"testing"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/command"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/util"
)

func TestWithVMRestEmbeded(t *testing.T) {

	if _, err := loadConfig(); err != nil {
		t.Errorf("unable to load config: %v", err)
	} else {

		logger := hclog.New(&hclog.LoggerOptions{
			Name:   "test",
			Output: os.Stdout,
			Level:  hclog.LevelFromString("debug"),
		})

		c := &settings.CommonConfig{
			Driver:   "vmrest",
			Port:     command.DEFAULT_RESTAPI_PORT,
			VMFolder: os.Getenv("VMFOLDER"),
		}

		if drv, err := driver.NewVMRestDriver(c, logger); err != nil {
			t.Errorf("vmrest api client failed: %v", err)
		} else {
			vmrun := drv.GetVmrun()

			if _, err := vmrun.ListVirtualMachines(); err != nil {
				t.Errorf("failed to list vm: %v", err)
			} /*else if guestInfos, err := config.buildCloudInit(); err != nil {
				t.Errorf("failed to create guestInfos: %v", err)
			} else {
				failOnError := func(vm *service.VirtualMachine, message string, err error) {
					t.Errorf(message, err)

					if vm != nil {
						vmrun.PowerOff(vm.Uuid)
						vmrun.Delete(vm.Uuid)
					}
				}

				request := service.CreateVirtualMachine{
					Template:     config.Template,
					Name:         config.Name,
					Vcpus:        config.Vcpus,
					Memory:       config.MemorySize,
					DiskSizeInMb: config.DiskSize,
					GuestInfos:   guestInfos,
					Networks:     config.buildNetworkInterface(),
				}

				if vm, err := vmrun.Create(&request); err != nil {
					failOnError(vm, "failed to create vm: %v", err)
				} else if _, err := vmrun.Status(vm.Uuid); err != nil {
					failOnError(vm, "failed to get status vm: %v", err)
				} else if _, err := vmrun.PowerOn(vm.Uuid); err != nil {
					failOnError(vm, "failed to poweron vm: %v", err)
				} else if err = waitForPowerState(vmrun, vm.Uuid, true); err != nil {
					failOnError(vm, "failed to wait poweroff vm: %v", err)
				} else if _, err := vmrun.WaitForToolsRunning(vm.Uuid); err != nil {
					failOnError(vm, "failed to wait tools vm: %v", err)
				} else if _, err := vmrun.WaitForIP(vm.Uuid); err != nil {
					failOnError(vm, "failed to wait ip vm: %v", err)
				} else if _, err := vmrun.Status(vm.Uuid); err != nil {
					failOnError(vm, "failed to get status vm: %v", err)
				} else if _, err := vmrun.PowerOff(vm.Uuid); err != nil {
					failOnError(vm, "failed to poweroff vm: %v", err)
				} else if err = waitForPowerState(vmrun, vm.Uuid, false); err != nil {
					failOnError(vm, "failed to wait poweroff vm: %v", err)
				} else if _, err := vmrun.Delete(vm.Uuid); err != nil {
					failOnError(vm, "failed to delete vm: %v", err)
				}
			}*/
		}
	}

	util.RunShutdownTasks()
}
