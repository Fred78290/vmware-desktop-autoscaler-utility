package command

import (
	"flag"

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

		flags := flag.NewFlagSet("api grpc", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["address"] = flags.String("address", DEFAULT_RESTAPI_ADDRESS, "Address for Grpc to listen")
		data["listen"] = flags.String("listen", DEFAULT_GRPCAPI_ADDRESS, "Address for Grpc to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["vmrest"] = flags.String("vmrest", DEFAULT_VMREST_ADDRESS, "Address for external vmrest api when driver is not vmrest")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")
		data["timeout"] = flags.String("timeout", "120s", "Timeout for operation")
		data["vmfolder"] = flags.String("vmfolder", utility.VMFolder(), "Location for vm")

		if restCommand, err = BuildRestApiCommand("api", ui)(); err != nil {
			return nil, err
		}

		if grpcCommand, err = BuildGrpcApiCommand("grpc", ui)(); err != nil {
			return nil, err
		}

		return &BothApiCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " grpc",
				SynopsisText:  "Run VMware desktop Utility grpc mode",
				UI:            ui,
				flagdata:      data,
			},
			RestCommand: restCommand.(*RestApiCommand),
			GrpcCommand: grpcCommand.(*GrpcApiCommand),
			Config:      &BothApiConfig{}}, nil
	}
}

func (c *BothApiCommand) Run(args []string) (exitCode int) {
	if exitCode = c.RestCommand.Run(args); exitCode == 0 {
		exitCode = c.GrpcCommand.SharedRun(args, c.driver)
	}

	return exitCode
}
