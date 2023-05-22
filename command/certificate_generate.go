package command

import (
	"flag"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	"github.com/mitchellh/cli"
)

type CertificateGenerateCommand struct {
	Command
	generate bool
	override bool
}

func BuildCertificateGenerateCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("certificate generate", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		data["cert_override"] = flags.Bool("cert-override", false, "Override existing cert")

		return &CertificateGenerateCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " certificate generate",
				SynopsisText:  "Generate required certificates",
				UI:            ui,
				flagdata:      data,
			},
			generate: true,
		}, nil
	}
}

func BuildCertificateGetCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("certificate generate", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		return &CertificateGenerateCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " certificate get",
				SynopsisText:  "Get path to required certificates",
				UI:            ui,
				flagdata:      data,
			},
			generate: false,
		}, nil
	}
}

func (c *CertificateGenerateCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)
	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}

	paths, err := utility.GetCertificatePaths()
	if err != nil {
		c.UI.Error("Certificate generation setup failed: " + err.Error())
		return exitCode
	}

	if c.generate {
		if c.override || !utils.FileExists(paths.Certificate) || !utils.FileExists(paths.PrivateKey) || !utils.FileExists(paths.ClientCertificate) || !utils.FileExists(paths.ClientKey) {
			if err := utility.GenerateCertificate(); err != nil {
				c.UI.Error("Certificate generation failed: " + err.Error())
				return exitCode
			}
		}
	}

	c.UI.Output(utils.ToJSON(&paths))

	return 0
}

func (c *CertificateGenerateCommand) setup(args []string) (err error) {
	err = c.defaultSetup(args)

	defaultValue := false

	if c.generate {
		c.override = c.GetConfigBool("cert_override", &defaultValue)
	}

	return
}
