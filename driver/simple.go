package driver

import (
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
)

type SimpleDriver struct {
	vagrant_driver.SimpleDriver
	ExtendedDriver
}

func (d *SimpleDriver) GetDriver() vagrant_driver.Driver {
	return &d.SimpleDriver
}

func NewSimpleDriver(vmxPath *string, b *BaseDriver, logger hclog.Logger) (*SimpleDriver, error) {
	if driver, err := vagrant_driver.NewSimpleDriver(vmxPath, &b.BaseDriver, logger); err != nil {
		return nil, err
	} else {
		driver := &SimpleDriver{
			SimpleDriver: *driver,
			ExtendedDriver: ExtendedDriver{
				vmwarePaths: b.GetVmwarePaths(),
				vmrun:       b.GetVmrun(),
				client:      b.GetVMRestApiClient(),
			},
		}

		return driver, nil
	}
}
