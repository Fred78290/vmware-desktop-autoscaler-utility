package driver

import (
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type SimpleDriver struct {
	driver      *vagrant_driver.SimpleDriver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

func (b *SimpleDriver) GetDriver() vagrant_driver.Driver {
	return b.driver
}

func (b *SimpleDriver) GetVmwarePaths() *utility.VmwarePaths {
	return b.vmwarePaths
}

func (b *SimpleDriver) GetVmrun() service.Vmrun {
	return b.vmrun
}

func NewSimpleDriver(vmxPath *string, b BaseDriver, logger hclog.Logger) (*SimpleDriver, error) {
	if driver, err := vagrant_driver.NewSimpleDriver(vmxPath, b.GetBaseDriver(), logger); err != nil {
		return nil, err
	} else {
		driver := &SimpleDriver{
			driver:      driver,
			vmwarePaths: b.GetVmwarePaths(),
			vmrun:       b.GetVmrun(),
		}

		return driver, nil
	}
}
