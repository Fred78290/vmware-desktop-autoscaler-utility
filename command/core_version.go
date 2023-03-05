package command

import (
	"flag"
	"fmt"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/version"
	"github.com/mitchellh/cli"
)

// Command to display version
type VersionCommand struct {
	Command
}

func BuildVersionCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("version", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		return &VersionCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " version",
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

	c.UI.Info(fmt.Sprintf("The current version is:%s, build at:%s", version.VERSION, version.BUILD_DATE))

	return 0
}

func (c *VersionCommand) setup(args []string) error {
	return c.defaultSetup(args)
}
