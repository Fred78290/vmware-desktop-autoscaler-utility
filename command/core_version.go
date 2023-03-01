package command

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"
)

var phVersion = "v0.0.0-unset"
var phBuildDate = ""

// Command to display version
type VersionCommand struct {
	Command
}

func BuildVersionCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("api", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["address"] = flags.String("address", DEFAULT_GRPCAPI_ADDRESS, "Address for Grpc to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")

		return &VersionCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " grpc",
				SynopsisText:  "Get VMware desktop Utility version",
				UI:            ui,
				flagdata:      data,
			},
		}, nil
	}
}

func (c *VersionCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}

	c.UI.Info(fmt.Sprintf("The current version is:%s, build at:%s", phVersion, phBuildDate))

	return 0
}

func (c *VersionCommand) setup(args []string) error {
	return c.defaultSetup(args)
}
