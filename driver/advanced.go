package driver

import (
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
)

type AdvancedDriver struct {
	vagrant_driver.AdvancedDriver
	ExtendedDriver
}

func (d *AdvancedDriver) GetDriver() vagrant_driver.Driver {
	return &d.AdvancedDriver
}

func NewAdvancedDriver(vmxPath *string, b *BaseDriver, logger hclog.Logger) (*AdvancedDriver, error) {
	if driver, err := vagrant_driver.NewAdvancedDriver(vmxPath, &b.BaseDriver, logger); err != nil {
		return nil, err
	} else {
		driver := &AdvancedDriver{
			AdvancedDriver: *driver,
			ExtendedDriver: ExtendedDriver{
				vmwarePaths: b.GetVmwarePaths(),
				vmrun:       b.GetVmrun(),
				client:      b.GetVMRestApiClient(),
			},
		}

		return driver, nil
	}
}
