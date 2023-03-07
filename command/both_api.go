package command

import (
	"flag"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/mitchellh/cli"
)

// Command for starting the GRPC API
type BothApiCommand struct {
	Command
	RestCommand *RestApiCommand
	GrpcCommand *GrpcApiCommand
	Config      *BothApiConfig
}

type BothApiConfig struct {
	settings.CommonConfig
}

func BuildBothApiCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		var restCommand cli.Command
		var grpcCommand cli.Command
		var err error

		flags := flag.NewFlagSet("full", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["address"] = flags.String("address", DEFAULT_RESTAPI_ADDRESS, "Address for Grpc to listen")
		data["port"] = flags.Int64("port", DEFAULT_RESTAPI_PORT, "Port for API to listen")
		data["listen"] = flags.String("listen", DEFAULT_GRPCAPI_ADDRESS, "Address for Grpc to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["vmrest"] = flags.String("vmrest", DEFAULT_VMREST_ADDRESS, "Address for external vmrest api when driver is not vmrest")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")
		data["timeout"] = flags.Duration("timeout", 120*time.Second, "Timeout for operation")
		data["vmfolder"] = flags.String("vmfolder", utility.VMFolder(), "Location for vm")

		if restCommand, err = BuildRestApiCommand("api", ui)(); err != nil {
			return nil, err
		}

		if grpcCommand, err = BuildGrpcApiCommand("grpc", ui)(); err != nil {
			return nil, err
		}

		command := &BothApiCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " full",
				SynopsisText:  "Run VMware desktop Utility api and grpc mode",
				UI:            ui,
				flagdata:      data,
			},
			RestCommand: restCommand.(*RestApiCommand),
			GrpcCommand: grpcCommand.(*GrpcApiCommand),
			Config:      &BothApiConfig{}}

		command.RestCommand.Flags = flags
		command.GrpcCommand.Flags = flags
		command.RestCommand.flagdata = data
		command.GrpcCommand.flagdata = data

		return command, nil
	}
}

func (c *BothApiCommand) Run(args []string) (exitCode int) {
	if exitCode = c.RestCommand.Run(args); exitCode == 0 {
		exitCode = c.GrpcCommand.SharedRun(args, c.driver)
	}

	return exitCode
}
