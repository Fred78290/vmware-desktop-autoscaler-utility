package command

import (
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

// const DEFAULT_VMREST_ADDRESS = "http://127.0.0.1:8697"
const DEFAULT_VMREST_ADDRESS = ""

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

func (c *GrpcApiCommand) logError(err error) {
	if c.Config.LogDisplay {
		c.logger.Error("api setup failure", "error", err)
	} else {
		c.UI.Error("Failed to setup VMWare desktop utility gRPC service - " + err.Error())
	}
}

func (c *GrpcApiCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
	} else if grpc, err := c.buildGrpc(); err != nil {
		c.logError(err)
	} else {

		if c.Config.LogDisplay {
			c.logger.Info("starting service")
		} else {
			c.UI.Info("Starting the VMWare desktop utility gRPC service")
		}

		if err = grpc.Start(); err != nil {
			c.logError(err)
		} else {
			exitCode = 0

			util.RegisterShutdownTask(func() {
				if c.Config.LogDisplay {
					c.logger.Info("halting serivce")
				} else {
					c.UI.Info("Halting the VMWare desktop utility gRPC service")
				}

				grpc.Stop()
			})

			<-grpc.HaltedChan
		}

	}

	return exitCode
}

func (c *GrpcApiCommand) buildGrpc() (a *server.Grpc, err error) {
	bindAddr := c.Config.Listen

	// Start with building the base driver
	if drv, err := driver.NewVMRestDriver(&c.Config.CommonConfig, c.logger); err != nil {
		return nil, err
	} else {
		c.driver = drv

		a, err = server.CreateGrpc(bindAddr, drv, c.Config.LogDisplay, c.UI, c.logger)

		if err != nil {
			c.logger.Debug("utility server setup failure", "error", err)
			return nil, errors.New("failed to setup VMWare desktop utility gRPC service - " + err.Error())
		}

		return a, nil
	}
}

func (c *GrpcApiCommand) SharedRun(args []string, drv driver.Driver) int {
	err := c.setup(args)

	bindAddr := c.Config.Listen
	exitCode := 1

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
	} else if grpc, err := server.CreateGrpc(bindAddr, drv, c.Config.LogDisplay, c.UI, c.logger); err != nil {
		if c.Config.LogDisplay {
			c.logger.Error("api setup failure", "error", err)
		} else {
			c.UI.Error("Failed to setup VMWare desktop utility gRPC service - " + err.Error())
		}
	} else {

		if c.Config.LogDisplay {
			c.logger.Info("starting service")
		} else {
			c.UI.Info("Starting the VMWare desktop utility gRPC service")
		}

		if err = grpc.Start(); err != nil {
			if c.Config.LogDisplay {
				c.logger.Error("startup failure", "error", err)
			} else {
				c.UI.Error("Failed to start the VMWare desktop utility gRPC service - " + err.Error())
			}
		} else {
			exitCode = 0

			util.RegisterShutdownTask(func() {
				if c.Config.LogDisplay {
					c.logger.Info("halting serivce")
				} else {
					c.UI.Info("Halting the VMWare desktop utility gRPC service")
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

		if c.DefaultConfig.ConfigFile != nil {
			rc.CommonConfig = c.DefaultConfig.ConfigFile.CommonConfig
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
