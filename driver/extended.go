package driver

import (
	"context"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"

	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type ExtendedDriver struct {
	vagrant_driver.Driver
	VmwarePaths *utility.VmwarePaths
	Vmrun       service.Vmrun
}

type BaseDriver interface {
	GetBaseDriver() *vagrant_driver.BaseDriver
	GetVmwarePaths() *utility.VmwarePaths
	GetVmrun() service.Vmrun
}

type Driver interface {
	GetDriver() vagrant_driver.Driver
	GetVmwarePaths() *utility.VmwarePaths
	GetVmrun() service.Vmrun
}

type baseDriverImpl struct {
	baseDriver  *vagrant_driver.BaseDriver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

type driverImpl struct {
	driver      vagrant_driver.Driver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

type VmrestDriver struct {
	vagrant_driver.Driver
	VmwarePaths *utility.VmwarePaths
	Vmrun       service.Vmrun
}

func (b *baseDriverImpl) GetBaseDriver() *vagrant_driver.BaseDriver {
	return b.baseDriver
}

func (b *baseDriverImpl) GetVmwarePaths() *utility.VmwarePaths {
	return b.vmwarePaths
}

func (b *baseDriverImpl) GetVmrun() service.Vmrun {
	return b.vmrun
}

func (b *driverImpl) GetDriver() vagrant_driver.Driver {
	return b.driver
}

func (b *driverImpl) GetVmwarePaths() *utility.VmwarePaths {
	return b.vmwarePaths
}

func (b *driverImpl) GetVmrun() service.Vmrun {
	return b.vmrun
}

func NewVmrestDriver(ctx context.Context, f Driver, logger hclog.Logger) (Driver, error) {
	if driver, err := vagrant_driver.NewVmrestDriver(ctx, f.GetDriver(), logger); err != nil {
		return nil, err
	} else {
		driver := &driverImpl{
			driver:      driver,
			vmwarePaths: f.GetVmwarePaths(),
			vmrun:       f.GetVmrun(),
		}

		return driver, nil
	}
}

func CreateDriver(vmxPath *string, b BaseDriver, logger hclog.Logger) (Driver, error) {
	if driver, err := vagrant_driver.CreateDriver(vmxPath, b.GetBaseDriver(), logger); err != nil {
		return nil, err
	} else {
		driver := &driverImpl{
			driver:      driver,
			vmwarePaths: b.GetVmwarePaths(),
			vmrun:       b.GetVmrun(),
		}

		return driver, nil
	}
}

func NewBaseDriver(vmxPath *string, licenseOverride string, logger hclog.Logger) (BaseDriver, error) {
	if baseDriver, err := vagrant_driver.NewBaseDriver(vmxPath, licenseOverride, logger); err != nil {
		return nil, err
	} else if paths, err := utility.LoadVmwarePaths(logger); err != nil {
		return nil, err
	} else if vmrun, err := service.NewVmrun(paths.Vmrun, logger); err != nil {
		return nil, err
	} else {

		driver := &baseDriverImpl{
			baseDriver:  baseDriver,
			vmwarePaths: paths,
			vmrun:       vmrun,
		}

		return driver, nil
	}

}
