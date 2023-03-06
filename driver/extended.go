package driver

import (
	"net/url"
	"time"

	"github.com/Fred78290/vmrest-go-client/client"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	"github.com/hashicorp/go-hclog"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	vagrant_settings "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/settings"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type Driver interface {
	vagrant_driver.Driver

	GetDriver() vagrant_driver.Driver
	GetVmwarePaths() *utility.VmwarePaths
	GetVmrun() service.Vmrun
	GetVMRestApiClient() *client.APIClient
}

type ExtendedDriver struct {
	vmwarePaths *utility.VmwarePaths
	vmrun       service.Vmrun
	client      *client.APIClient
}

type BaseDriver struct {
	vagrant_driver.BaseDriver
	ExtendedDriver
}

type driverImpl struct {
	driver vagrant_driver.Driver
	ExtendedDriver
}

func (d *ExtendedDriver) GetVMRestApiClient() *client.APIClient {
	return d.client
}

func (d *ExtendedDriver) GetVmwarePaths() *utility.VmwarePaths {
	return d.vmwarePaths
}

func (d *ExtendedDriver) GetVmrun() service.Vmrun {
	return d.vmrun
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

func (d *driverImpl) Settings() *vagrant_settings.Settings {
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

func CreateDriver(vmxPath *string, b *BaseDriver, logger hclog.Logger) (Driver, error) {
	if driver, err := vagrant_driver.CreateDriver(vmxPath, &b.BaseDriver, logger); err != nil {
		return nil, err
	} else {
		var result Driver

		if simple, ok := driver.(*vagrant_driver.SimpleDriver); ok {
			result = &SimpleDriver{
				SimpleDriver: *simple,
				ExtendedDriver: ExtendedDriver{
					vmwarePaths: b.GetVmwarePaths(),
					vmrun:       b.GetVmrun(),
					client:      b.GetVMRestApiClient(),
				},
			}
		} else if advanced, ok := driver.(*vagrant_driver.AdvancedDriver); ok {
			result = &AdvancedDriver{
				AdvancedDriver: *advanced,
				ExtendedDriver: ExtendedDriver{
					vmwarePaths: b.GetVmwarePaths(),
					vmrun:       b.GetVmrun(),
					client:      b.GetVMRestApiClient(),
				},
			}
		} else {
			result = &driverImpl{
				driver: driver,
				ExtendedDriver: ExtendedDriver{
					vmwarePaths: b.GetVmwarePaths(),
					vmrun:       b.GetVmrun(),
					client:      b.GetVMRestApiClient(),
				},
			}
		}
		return result, nil
	}
}

func NewVMRestClient(c *settings.CommonConfig) (*client.APIClient, error) {
	var configuration *client.Configuration

	if c.VMRestURL != "" {
		if u, err := url.Parse(c.VMRestURL); err != nil {
			return nil, err
		} else {
			var username string
			var password string
			var set bool

			if password, set = u.User.Password(); set {
				username = u.User.Username()
			} else {
				password = ""
			}

			u.User = nil

			configuration = &client.Configuration{
				Endpoint:    u.String(),
				UserName:    username,
				Password:    password,
				UserAgent:   utils.UserAgent(),
				Timeout:     c.Timeout / time.Second,
				UnsecureTLS: true,
			}

		}
	} else {
		configuration = &client.Configuration{
			Endpoint:    "http://127.0.0.1:8697",
			UserAgent:   utils.UserAgent(),
			UnsecureTLS: true,
		}
	}

	return client.NewAPIClient(configuration)
}

func NewBaseDriver(vmxPath *string, c *settings.CommonConfig, logger hclog.Logger) (*BaseDriver, error) {
	if baseDriver, err := vagrant_driver.NewBaseDriver(vmxPath, c.LicenseOverride, logger); err != nil {
		return nil, err
	} else if paths, err := utility.LoadVmwarePaths(logger); err != nil {
		return nil, err
	} else if vmrun, err := service.NewVmrun(c, paths.Vmrun, paths.Vdiskmanager, logger); err != nil {
		return nil, err
	} else if client, err := NewVMRestClient(c); err != nil {
		return nil, err
	} else {

		driver := &BaseDriver{
			BaseDriver: *baseDriver,
			ExtendedDriver: ExtendedDriver{
				vmwarePaths: paths,
				vmrun:       vmrun,
				client:      client,
			},
		}

		return driver, nil
	}

}
