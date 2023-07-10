package command

import (
	"errors"
	"flag"
	"sync"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/server"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/util"
	"github.com/mitchellh/cli"
)

var Shutdown sync.Cond

const DEFAULT_RESTAPI_PORT = 5622
const DEFAULT_RESTAPI_ADDRESS = "127.0.0.1"

// Command for starting the REST API
type RestApiCommand struct {
	Command
	Config      *RestApiConfig
	waitRestApi bool
}

type RestApiConfig struct {
	settings.CommonConfig
}

func BuildRestApiCommand(name string, waitRestApi bool, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("api", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["address"] = flags.String("address", DEFAULT_RESTAPI_ADDRESS, "Address for API to listen")
		data["port"] = flags.Int64("port", DEFAULT_RESTAPI_PORT, "Port for API to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")
		data["timeout"] = flags.Duration("timeout", 120*time.Second, "Timeout for operation")
		data["vmfolder"] = flags.String("vmfolder", utility.VMFolder(), "Location for vm")
		data["vmrest"] = flags.String("vmrest", DEFAULT_VMREST_ADDRESS, "Address for external vmrest api when driver is not vmrest")

		return &RestApiCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " api",
				SynopsisText:  "Run VMware desktop Utility rest mode",
				UI:            ui,
				flagdata:      data,
			},
			Config:      &RestApiConfig{},
			waitRestApi: waitRestApi,
		}, nil
	}
}

func (c *RestApiCommand) Run(args []string) int {
	var err error
	var restApi *server.Api

	exitCode := 1

	if err = c.setup(args); err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}

	if restApi, err = c.buildRestApi(); err != nil {
		if c.Config.LogDisplay {
			c.logger.Error("api setup failure", "error", err)
		} else {
			c.UI.Error("Failed to setup VMWare desktop utility API service - " + err.Error())
		}

		return exitCode
	}

	if c.Config.LogDisplay {
		c.logger.Info("starting service")
	} else {
		c.UI.Info("Starting the VMWare desktop utility API service")
	}

	if err = restApi.Start(); err != nil {
		if c.Config.LogDisplay {
			c.logger.Error("startup failure", "error", err)
		} else {
			c.UI.Error("Failed to start the VMWare desktop utility API service - " + err.Error())
		}

		return exitCode
	}

	util.RegisterShutdownTask(func() {
		if c.Config.LogDisplay {
			c.logger.Info("halting serivce")
		} else {
			c.UI.Info("Halting the VMWare desktop utility API service")
		}

		restApi.Stop()
	})

	if c.waitRestApi {
		<-restApi.HaltedChan
	}

	return 0
}

func (c *RestApiCommand) buildRestApi() (a *server.Api, err error) {
	bindAddr := c.Config.Address
	bindPort := int(c.Config.Port)

	// Start with building the base driver
	if drv, err := driver.NewVMRestDriver(&c.Config.CommonConfig, c.logger); err != nil {
		return nil, err
	} else {
		c.driver = drv

		if a, err = server.CreateRestApi(bindAddr, bindPort, drv, c.logger); err != nil {
			c.logger.Debug("utility server setup failure", "error", err)
			return nil, errors.New("failed to setup VMWare desktop utility API service - " + err.Error())
		}

		return a, nil
	}
}

func (c *RestApiCommand) setup(args []string) (err error) {
	err = c.defaultSetup(args)
	if err != nil {
		return
	}

	var rc RestApiConfig

	if c.DefaultConfig.ConfigFile != nil {
		rc.CommonConfig = c.DefaultConfig.ConfigFile.CommonConfig
	}

	c.Config.Address = c.GetConfigValue("address", rc.Paddress)
	c.Config.Timeout = c.GetConfigDuration("timeout", rc.Ptimeout)
	c.Config.VMFolder = c.GetConfigValue("vmfolder", rc.Pvmfolder)
	c.Config.VMRestURL = c.GetConfigValue("vmrest", rc.Pvmrest)
	c.Config.Port = c.GetConfigInt64("port", rc.Pport)
	c.Config.Driver = c.GetConfigValue("driver", rc.Pdriver)
	c.Config.LicenseOverride = c.GetConfigValue("license_override", rc.PlicenseOverride)
	c.Config.LogDisplay = c.DefaultConfig.LogFile != ""

	return
}
