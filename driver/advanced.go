package driver

import (
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type AdvancedDriver struct {
	driver      *vagrant_driver.AdvancedDriver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

func (b *AdvancedDriver) GetDriver() vagrant_driver.Driver {
	return b.driver
}

func (b *AdvancedDriver) GetVmwarePaths() *utility.VmwarePaths {
	return b.vmwarePaths
}

func (b *AdvancedDriver) GetVmrun() service.Vmrun {
	return b.vmrun
}

func NewAdvancedDriver(vmxPath *string, b BaseDriver, logger hclog.Logger) (*AdvancedDriver, error) {
	if driver, err := vagrant_driver.NewAdvancedDriver(vmxPath, b.GetBaseDriver(), logger); err != nil {
		return nil, err
	} else {
		driver := &AdvancedDriver{
			driver:      driver,
			vmwarePaths: b.GetVmwarePaths(),
			vmrun:       b.GetVmrun(),
		}

		return driver, nil
	}
}
