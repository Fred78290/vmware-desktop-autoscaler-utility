package command

import (
	"flag"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	vagrant_utility "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
	"github.com/mitchellh/cli"
)

const RUNIT_DIR = "/etc/sv"

type ServiceInstallCommand struct {
	Command
	Config *ServiceInstallConfig
}

type ServiceInstallConfig struct {
	settings.CommonConfig `hcl:"core,block"`

	ConfigPath  string  // used for init printing
	ConfigWrite string  // used for init printing
	ExePath     string  // used for init printing
	Init        string  // used on linux (style)
	Print       bool    // used for init printing
	RunitDir    string  // used on linux
	Pinit       *string `hcl:"init"`      // used on linux (style)
	PrunitDir   *string `hcl:"runit_dir"` // used on linux
}

func BuildServiceInstallCommand(name string, ui cli.Ui) cli.CommandFactory {
	return func() (cli.Command, error) {
		flags := flag.NewFlagSet("service install", flag.ContinueOnError)
		data := make(map[string]interface{})
		setDefaultFlags(flags, data)

		if runtime.GOOS != "windows" {
			data["runit_sv"] = flags.String("runit-sv", RUNIT_DIR, "Path to runit sv directory")
			data["init"] = flags.String("init-style", "", "Init in use (systemd, runit, sysv)")
		}

		data["vmrest"] = flags.String("vmrest", DEFAULT_VMREST_ADDRESS, "Address for external vmrest api when driver is not vmrest")
		data["listen"] = flags.String("listen", DEFAULT_GRPCAPI_ADDRESS, "Address for Grpc to listen")
		data["address"] = flags.String("address", DEFAULT_RESTAPI_ADDRESS, "Address for API to listen")
		data["port"] = flags.Int64("port", DEFAULT_RESTAPI_PORT, "Port for API to listen")
		data["driver"] = flags.String("driver", "", "Driver to use (simple, advanced, or vmrest)")
		data["license_override"] = flags.String("license-override", "", "Override VMware license detection (standard or professional)")
		data["print"] = flags.Bool("print", false, "Print init file to STDOUT")
		data["exe_path"] = flags.String("exe-path", "", "Path used for executable (used for print only)")
		data["config_path"] = flags.String("config-path", "", "Path for configuration file (used for print only)")
		data["config_write"] = flags.String("config-write", "./service.hcl", "Path to write configuration file (used for print only)")
		data["timeout"] = flags.Duration("timeout", 120*time.Second, "Timeout for operation")
		data["vmfolder"] = flags.String("vmfolder", utility.VMFolder(), "Location for vm")

		return &ServiceInstallCommand{
			Command: Command{
				DefaultConfig: &Config{},
				Name:          name,
				Flags:         flags,
				HelpText:      name + " service install",
				SynopsisText:  "Install service script",
				UI:            ui,
				flagdata:      data,
			},
			Config: &ServiceInstallConfig{},
		}, nil
	}
}

func (c *ServiceInstallCommand) Run(args []string) int {
	exitCode := 1
	err := c.setup(args)

	if err != nil {
		c.UI.Error("Failed to initialize: " + err.Error())
		return exitCode
	}

	if c.Config.Print {
		err = c.print()
		if err != nil {
			c.UI.Error("Failed to print service: " + err.Error())
			return exitCode
		}
	} else {
		err = c.install()

		if err != nil {
			c.UI.Error("Failed to install service: " + err.Error())
			return exitCode
		}

		c.UI.Info("Service has been installed!")
	}

	return 0
}

func (c *ServiceInstallCommand) setup(args []string) (err error) {
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

	c.Config.Address = c.GetConfigValue("address", sc.Paddress)
	c.Config.ConfigPath = c.GetConfigValue("config_path", nil)
	c.Config.ConfigWrite = c.GetConfigValue("config_write", nil)
	c.Config.Driver = c.GetConfigValue("driver", sc.Pdriver)
	c.Config.ExePath = c.GetConfigValue("exe_path", nil)
	c.Config.LicenseOverride = c.GetConfigValue("license_override", sc.PlicenseOverride)
	c.Config.Listen = c.GetConfigValue("listen", sc.Plisten)
	c.Config.Port = c.GetConfigInt64("port", sc.Pport)
	c.Config.Print = c.GetConfigBool("print", nil)
	c.Config.Timeout = c.GetConfigDuration("timeout", sc.Ptimeout)
	c.Config.VMFolder = c.GetConfigValue("vmfolder", sc.Pvmfolder)
	c.Config.VMRestURL = c.GetConfigValue("vmrest", sc.Pvmrest)

	return
}

func (c *ServiceInstallCommand) writeConfig(fpath string) (cpath string, err error) {

	if fpath != "" {
		cpath = fpath
	} else {
		cpath = filepath.Join(vagrant_utility.DirectoryFor("config"), "service.hcl")
	}

	config := Config{
		ConfigFile: &ServiceInstallConfig{},
	}

	if c.Config.Address != "" {
		config.ConfigFile.Paddress = &c.Config.Address
	}

	if c.Config.Driver != "" {
		config.ConfigFile.Pdriver = &c.Config.Driver
	}

	if c.Config.LicenseOverride != "" {
		config.ConfigFile.PlicenseOverride = &c.Config.LicenseOverride
	}

	if c.Config.Listen != "" {
		config.ConfigFile.Plisten = &c.Config.Listen
	}

	if c.Config.Timeout != 0 {
		config.ConfigFile.Ptimeout = &c.Config.Timeout
	}

	if c.Config.Port != 0 {
		config.ConfigFile.Pport = &c.Config.Port
	}

	if c.Config.VMFolder != "" {
		config.ConfigFile.Pvmfolder = &c.Config.VMFolder
	}

	if c.Config.VMRestURL != "" {
		config.ConfigFile.Pvmrest = &c.Config.VMRestURL
	}

	if c.Config.RunitDir != "" {
		config.ConfigFile.PrunitDir = &c.Config.RunitDir
	}

	if c.Config.Init != "" {
		config.ConfigFile.Pinit = &c.Config.Init
	}

	if c.DefaultConfig.Debug {
		config.Pdebug = &c.DefaultConfig.Debug
	}

	if c.DefaultConfig.Level == "" {
		c.DefaultConfig.Level = "info"
	}

	config.Plevel = &c.DefaultConfig.Level

	if c.DefaultConfig.LogFile != "" {
		config.PlogFile = &c.DefaultConfig.LogFile
	}

	if c.DefaultConfig.LogAppend {
		config.PlogAppend = &c.DefaultConfig.LogAppend
	}

	err = vagrant_utility.WriteConfigFile(cpath, &config)

	if err != nil {
		c.logger.Debug("failed to create configuration file", "path", cpath, "error", err)
		return
	}
	return
}
