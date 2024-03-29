package command

import (
	"flag"
	"runtime"

	"github.com/mitchellh/cli"
)

type ServiceUninstallCommand struct {
	Command
	Config *ServiceInstallConfig
}

func BuildServiceUninstallCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("service uninstall", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		if runtime.GOOS != "windows" {
			data["runit_sv"] = flags.String("runit-sv", RUNIT_DIR, "Path to runit sv directory")
			data["init"] = flags.String("init-style", "", "Init in use (systemd, runit, sysv)")
		}

		return &ServiceUninstallCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " service uninstall",
				SynopsisText:  "Uninstall service script",
				UI:            ui,
				flagdata:      data},
			Config: &ServiceInstallConfig{}}, nil
	}

}

func (c *ServiceUninstallCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)
	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}
	err = c.uninstall()
	if err != nil {
		c.UI.Error("Failed to uninstall service: " + err.Error())
		return exitCode
	}
	c.UI.Info("Service has been uninstalled!")
	return 0
}

func (c *ServiceUninstallCommand) setup(args []string) (err error) {
	var sc ServiceInstallConfig

	if err = c.defaultSetup(args); err != nil {
		return
	}

	if c.DefaultConfig.ConfigFile != nil {
		sc = *c.DefaultConfig.ConfigFile
	}

	if runtime.GOOS != "windows" {
		c.Config.Init = c.GetConfigValue("init", sc.Pinit)
		c.Config.RunitDir = c.GetConfigValue("runit_sv", sc.PrunitDir)
	}
	return
}
