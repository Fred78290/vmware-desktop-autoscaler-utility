package command

import (
	"context"
	"errors"
	"flag"
	"sync"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/server"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/util"
	"github.com/mitchellh/cli"
)

var Shutdown sync.Cond

const DEFAULT_RESTAPI_PORT = 5322
const DEFAULT_RESTAPI_ADDRESS = "127.0.0.1"

// Command for starting the REST API
type RestApiCommand struct {
	Command
	Config *RestApiConfig
}

type RestApiConfig struct {
	Address         string
	Port            int64
	Driver          string
	LicenseOverride string
	LogDisplay      bool

	Paddress         *string `hcl:"address"`
	Pport            *int64  `hcl:"port"`
	Pdriver          *string `hcl:"driver"`
	PlicenseOverride *string `hcl:"license_override"`
}

func BuildRestApiCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("api", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["address"] = flags.String("address", DEFAULT_RESTAPI_ADDRESS, "Address for API to listen")
		data["port"] = flags.Int64("port", DEFAULT_RESTAPI_PORT, "Port for API to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")

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
			Config: &RestApiConfig{}}, nil
	}
}

func (c *RestApiCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)
	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}

	restApi, err := c.buildRestApi(c.Config.Driver, c.Config.Port)
	if err != nil {
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

	err = restApi.Start()
	if err != nil {
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
	<-restApi.HaltedChan
	return 0
}

func (c *RestApiCommand) buildRestApi(driverName string, port int64) (a *server.Api, err error) {
	bindAddr := c.Config.Address
	bindPort := int(port)

	// Start with building the base driver
	b, err := driver.NewBaseDriver(nil, c.Config.LicenseOverride, c.logger)
	if err != nil {
		c.logger.Error("base driver setup failure", "error", err)
		return
	}

	// Allow the user to define the driver. It may not work, but they're the boss
	attempt_vmrest := true
	var drv driver.Driver
	switch driverName {
	case "simple":
		c.logger.Warn("creating simple driver via user request")
		drv, err = driver.NewSimpleDriver(nil, b, c.logger)
		attempt_vmrest = false
	case "advanced":
		c.logger.Warn("creating advanced driver via user request")
		drv, err = driver.NewAdvancedDriver(nil, b, c.logger)
		attempt_vmrest = false
	default:
		if driverName != "" {
			c.logger.Warn("unknown driver name provided, detecting appropriate driver", "name", driverName)
		}
		drv, err = driver.CreateDriver(nil, b, c.logger)
	}
	if err != nil {
		c.logger.Error("driver setup failure", "error", err)
		return nil, errors.New("failed to setup VMWare desktop utility driver - " + err.Error())
	}

	// Now that we are setup, we can attempt to upgrade the driver to the
	// vmrest driver if possible or requested
	if attempt_vmrest {
		c.logger.Info("attempting to upgrade to vmrest driver")
		drv, err = driver.NewVmrestDriver(context.Background(), drv, c.logger)
		if err != nil {
			c.logger.Error("failed to upgrade to vmrest driver", "error", err)
			return
		}
	}

	if !drv.GetDriver().Validate() {
		// NOTE: We only log the failure and allow the process to start. This
		//       lets the plugin communicate with the service, but all requests
		//       result in an error which includes the validation failure.
		c.logger.Error("vmware validation failed")
	}

	a, err = server.CreateRestApi(bindAddr, bindPort, drv, c.logger)

	if err != nil {
		c.logger.Debug("utility server setup failure", "error", err)
		return nil, errors.New("failed to setup VMWare desktop utility API service - " + err.Error())
	}

	return
}

func (c *RestApiCommand) setup(args []string) (err error) {
	err = c.defaultSetup(args)
	if err != nil {
		return
	}

	var rc RestApiConfig

	if c.DefaultConfig.configFile != nil && c.DefaultConfig.configFile.RestApiConfig != nil {
		rc = *c.DefaultConfig.configFile.RestApiConfig
	}

	c.Config.Address = c.GetConfigValue("address", rc.Paddress)
	c.Config.Port = c.GetConfigInt64("port", rc.Pport)
	c.Config.Driver = c.GetConfigValue("driver", rc.Pdriver)
	c.Config.LicenseOverride = c.GetConfigValue("license_override", rc.PlicenseOverride)
	c.Config.LogDisplay = c.DefaultConfig.LogFile != ""

	return
}
