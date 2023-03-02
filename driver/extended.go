package driver

import (
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"

	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/settings"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type Driver interface {
	vagrant_driver.Driver

	GetDriver() vagrant_driver.Driver
	GetVmwarePaths() *utility.VmwarePaths
	GetVmrun() service.Vmrun
}

type BaseDriver struct {
	vagrant_driver.BaseDriver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

type driverImpl struct {
	driver      vagrant_driver.Driver
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
}

func (d *driverImpl) GetDriver() vagrant_driver.Driver {
	return d.driver
}

func (d *driverImpl) AddInternalPortForward(fwd *vagrant_driver.PortFwd) error {
	return d.driver.AddInternalPortForward(fwd)
}

func (d *driverImpl) AddPortFwd(fwds []*vagrant_driver.PortFwd) error {
	return d.driver.AddPortFwd(fwds)
}

func (d *driverImpl) AddVmnet(v *vagrant_driver.Vmnet) error {
	return d.driver.AddVmnet(v)
}

func (d *driverImpl) DeleteInternalPortForward(fwd *vagrant_driver.PortFwd) error {
	return d.driver.DeleteInternalPortForward(fwd)
}

func (d *driverImpl) DeletePortFwd(fwds []*vagrant_driver.PortFwd) error {
	return d.driver.DeletePortFwd(fwds)
}

func (d *driverImpl) DeleteVmnet(v *vagrant_driver.Vmnet) error {
	return d.driver.DeleteVmnet(v)
}

func (d *driverImpl) EnableInternalPortForwarding() error {
	return d.driver.EnableInternalPortForwarding()
}

func (d *driverImpl) InternalPortFwds() (fwds []*vagrant_driver.PortFwd, err error) {
	return d.driver.InternalPortFwds()
}

func (d *driverImpl) LoadNetworkingFile() (f utility.NetworkingFile, err error) {
	return d.driver.LoadNetworkingFile()
}

func (d *driverImpl) LookupDhcpAddress(device, mac string) (addr string, err error) {
	return d.driver.LookupDhcpAddress(device, addr)
}

func (d *driverImpl) Path() (path *string, err error) {
	return d.driver.Path()
}

func (d *driverImpl) PortFwds(device string) (fwds *vagrant_driver.PortFwds, err error) {
	return d.driver.PortFwds(device)
}

func (d *driverImpl) PrunePortFwds(fwds func(string) (*vagrant_driver.PortFwds, error), deleter func([]*vagrant_driver.PortFwd) error) error {
	return d.driver.PrunePortFwds(fwds, deleter)
}

func (d *driverImpl) ReserveDhcpAddress(slot int, mac, ip string) error {
	return d.driver.ReserveDhcpAddress(slot, mac, ip)
}

func (d *driverImpl) Settings() *settings.Settings {
	return d.driver.Settings()
}

func (d *driverImpl) UpdateVmnet(v *vagrant_driver.Vmnet) error {
	return d.driver.UpdateVmnet(v)
}

func (d *driverImpl) Validated() bool {
	return d.driver.Validated()
}

func (d *driverImpl) Validate() bool {
	return d.driver.Validate()
}

func (d *driverImpl) ValidationReason() string {
	return d.driver.ValidationReason()
}

func (d *driverImpl) VerifyVmnet() error {
	return d.driver.VerifyVmnet()
}

func (d *driverImpl) Vmnets() (v *vagrant_driver.Vmnets, err error) {
	return d.driver.Vmnets()
}

func (d *driverImpl) VmwareInfo() (info *vagrant_driver.VmwareInfo, err error) {
	return d.driver.VmwareInfo()
}

func (d *driverImpl) VmwarePaths() *utility.VmwarePaths {
	return d.driver.VmwarePaths()
}

func (d *driverImpl) GetVmwarePaths() *utility.VmwarePaths {
	return d.vmwarePaths
}

func (d *driverImpl) GetVmrun() service.Vmrun {
	return d.vmrun
}

func (d *BaseDriver) GetVmwarePaths() *utility.VmwarePaths {
	return d.vmwarePaths
}

func (d *BaseDriver) GetVmrun() service.Vmrun {
	return d.vmrun
}

func CreateDriver(vmxPath *string, b *BaseDriver, logger hclog.Logger) (Driver, error) {
	if driver, err := vagrant_driver.CreateDriver(vmxPath, &b.BaseDriver, logger); err != nil {
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

func NewBaseDriver(vmxPath *string, licenseOverride string, logger hclog.Logger) (*BaseDriver, error) {
	if baseDriver, err := vagrant_driver.NewBaseDriver(vmxPath, licenseOverride, logger); err != nil {
		return nil, err
	} else if paths, err := utility.LoadVmwarePaths(logger); err != nil {
		return nil, err
	} else if vmrun, err := service.NewVmrun(paths.Vmrun, logger); err != nil {
		return nil, err
	} else {

		driver := &BaseDriver{
			BaseDriver:  *baseDriver,
			vmwarePaths: paths,
			vmrun:       vmrun,
		}

		return driver, nil
	}

}
