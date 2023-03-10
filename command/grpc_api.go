package command

import (
	"context"
	"errors"
	"flag"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/server"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/util"
	"github.com/mitchellh/cli"
)

const DEFAULT_GRPCAPI_ADDRESS = "tcp://0.0.0.0:5323"
const DEFAULT_VMREST_ADDRESS = "http://127.0.0.1:8697"

// Command for starting the GRPC API
type GrpcApiCommand struct {
	Command
	Config *GrpcApiConfig
}

type GrpcApiConfig struct {
	settings.CommonConfig
}

func BuildGrpcApiCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("grpc", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["listen"] = flags.String("listen", DEFAULT_GRPCAPI_ADDRESS, "Address for Grpc to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["vmrest"] = flags.String("vmrest", DEFAULT_VMREST_ADDRESS, "Address for external vmrest api when driver is not vmrest")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")
		data["timeout"] = flags.Duration("timeout", 120*time.Second, "Timeout for operation")
		data["vmfolder"] = flags.String("vmfolder", utility.VMFolder(), "Location for vm")

		return &GrpcApiCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " grpc",
				SynopsisText:  "Run VMware desktop Utility grpc mode",
				UI:            ui,
				flagdata:      data,
			},
			Config: &GrpcApiConfig{}}, nil
	}
}

func (c *GrpcApiCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
	} else if grpc, err := c.buildGrpc(c.Config.Driver); err != nil {
		if c.Config.LogDisplay {
			c.logger.Error("api setup failure", "error", err)
		} else {
			c.UI.Error("Failed to setup VMWare desktop utility API service - " + err.Error())
		}
	} else {

		if c.Config.LogDisplay {
			c.logger.Info("starting service")
		} else {
			c.UI.Info("Starting the VMWare desktop utility API service")
		}

		if err = grpc.Start(); err != nil {
			if c.Config.LogDisplay {
				c.logger.Error("startup failure", "error", err)
			} else {
				c.UI.Error("Failed to start the VMWare desktop utility API service - " + err.Error())
			}
		} else {
			exitCode = 0

			util.RegisterShutdownTask(func() {
				if c.Config.LogDisplay {
					c.logger.Info("halting serivce")
				} else {
					c.UI.Info("Halting the VMWare desktop utility API service")
				}

				grpc.Stop()
			})

			<-grpc.HaltedChan
		}

	}

	return exitCode
}

func (c *GrpcApiCommand) buildGrpc(driverName string) (a *server.Grpc, err error) {
	bindAddr := c.Config.Listen

	// Start with building the base driver
	b, err := driver.NewBaseDriver(nil, &c.Config.CommonConfig, c.logger)
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
		if drv, err = driver.NewVmrestDriver(context.Background(), &c.Config.CommonConfig, drv, c.logger); err != nil {
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

	c.driver = drv

	a, err = server.CreateGrpc(bindAddr, drv, c.logger)

	if err != nil {
		c.logger.Debug("utility server setup failure", "error", err)
		return nil, errors.New("failed to setup VMWare desktop utility API service - " + err.Error())
	}

	return
}

func (c *GrpcApiCommand) SharedRun(args []string, drv driver.Driver) int {
	bindAddr := c.Config.Listen
	exitCode := 1
	err := c.setup(args)

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
	} else if grpc, err := server.CreateGrpc(bindAddr, drv, c.logger); err != nil {
		if c.Config.LogDisplay {
			c.logger.Error("api setup failure", "error", err)
		} else {
			c.UI.Error("Failed to setup VMWare desktop utility API service - " + err.Error())
		}
	} else {

		if c.Config.LogDisplay {
			c.logger.Info("starting service")
		} else {
			c.UI.Info("Starting the VMWare desktop utility API service")
		}

		if err = grpc.Start(); err != nil {
			if c.Config.LogDisplay {
				c.logger.Error("startup failure", "error", err)
			} else {
				c.UI.Error("Failed to start the VMWare desktop utility API service - " + err.Error())
			}
		} else {
			exitCode = 0

			util.RegisterShutdownTask(func() {
				if c.Config.LogDisplay {
					c.logger.Info("halting serivce")
				} else {
					c.UI.Info("Halting the VMWare desktop utility API service")
				}

				grpc.Stop()
			})

			<-grpc.HaltedChan
		}

	}

	return exitCode
}

func (c *GrpcApiCommand) setup(args []string) (err error) {
	if err = c.defaultSetup(args); err == nil {

		var rc GrpcApiConfig

		if c.DefaultConfig.configFile != nil && c.DefaultConfig.configFile.GrpcApiConfig != nil {
			rc = *c.DefaultConfig.configFile.GrpcApiConfig
		}

		c.Config.Listen = c.GetConfigValue("listen", rc.Plisten)
		c.Config.Timeout = c.GetConfigDuration("timeout", rc.Ptimeout)
		c.Config.VMFolder = c.GetConfigValue("vmfolder", rc.Pvmfolder)
		c.Config.VMRestURL = c.GetConfigValue("vmrest", rc.Pvmrest)
		c.Config.Driver = c.GetConfigValue("driver", rc.Pdriver)
		c.Config.LicenseOverride = c.GetConfigValue("license_override", rc.PlicenseOverride)
		c.Config.LogDisplay = c.DefaultConfig.LogFile != ""
	}

	return
}
