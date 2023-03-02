package driver

import (
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type AdvancedDriver struct {
	vagrant_driver.AdvancedDriver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

func (d *AdvancedDriver) GetDriver() vagrant_driver.Driver {
	return &d.AdvancedDriver
}

func (b *AdvancedDriver) GetVmwarePaths() *utility.VmwarePaths {
	return b.vmwarePaths
}

func (b *AdvancedDriver) GetVmrun() service.Vmrun {
	return b.vmrun
}

func NewAdvancedDriver(vmxPath *string, b *BaseDriver, logger hclog.Logger) (*AdvancedDriver, error) {
	if driver, err := vagrant_driver.NewAdvancedDriver(vmxPath, &b.BaseDriver, logger); err != nil {
		return nil, err
	} else {
		driver := &AdvancedDriver{
			AdvancedDriver: *driver,
			vmwarePaths:    b.GetVmwarePaths(),
			vmrun:          b.GetVmrun(),
		}

		return driver, nil
	}
}
