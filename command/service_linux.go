package command

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

// Temlate for systemd
//
// string - exectuable path
// string - configuration path
const SYSTEMD_TEMPLATE = `[Unit]
Description=VMWare Desktop utility
After=network.target

[Service]
Type=simple
ExecStart=%s full -config-file=%s
Restart=on-abort

[Install]
WantedBy=multi-user.target
`

const SYSV_INITD = "/etc/init.d"

func (c *Command) systemdServicePath(exePath string) (result string) {
	if homeDir, err := os.UserHomeDir(); err != nil {
		log.Fatalf("Can't get home directory, reason: %v", err)
	} else {
		result = path.Join(homeDir, ".config/systemd/user/", c.Name+".service")

		os.Mkdir(path.Dir(result), os.ModePerm)
	}

	return
}

// Attached to generic command so both install and uninstall can access
func (c *Command) detectInit() string {
	// Get the command name for init
	exitCode, out := utility.ExecuteWithOutput(exec.Command("ps", "-xo", "comm=", "1"))

	c.logger.Trace("init process check", "exitcode", exitCode, "output", out)

	if exitCode == 0 {
		out = strings.TrimSpace(out)

		if strings.Contains(out, "systemd") {
			return "systemd"
		} else if strings.Contains(out, "runit") {
			return "runit"
		}
	}

	// Check if sys-v directory exists
	if utility.FileExists(SYSV_INITD) {
		c.logger.Trace("sysv init check", "path", SYSV_INITD)
		return "sysv"
	}

	return "unknown"
}

func (c *ServiceInstallCommand) print() (err error) {
	initStyle := c.getInitStyle()

	if initStyle == "systemd" {
		config := ""
		exePath := c.Config.ExePath

		if exePath == "" {
			exePath, err = os.Executable()
			if err != nil {
				c.logger.Error("executable path detection failure", "error", err)
				return
			}
		}

		if config, err = c.writeConfig(c.Config.ConfigWrite); err != nil {
			c.logger.Debug("service install failure", "error", err)
		} else if c.Config.ConfigPath != "" {
			config = c.Config.ConfigPath
		}

		fmt.Printf(SYSTEMD_TEMPLATE, exePath, config)
	} else {
		err = fmt.Errorf("%s not supported for installation", initStyle)
	}

	return
}

func (c *ServiceInstallCommand) getInitStyle() string {
	initStyle := c.Config.Init

	if initStyle == "" {
		initStyle = c.detectInit()
	}

	return initStyle
}

func (c *ServiceInstallCommand) install() (err error) {
	exePath := ""
	config := ""
	initStyle := c.getInitStyle()

	if initStyle == "systemd" {
		if exePath, err = os.Executable(); err != nil {
			c.logger.Error("executable path detection failure", "error", err)
		} else if config, err = c.writeConfig(""); err != nil {
			c.logger.Error("failed to create configuration file", "path", config, "error", err)
		} else {
			c.logger.Trace("installing service", "init", initStyle, "exe", exePath, "port", c.Config.Port)

			if err = c.installSystemd(exePath, config); err != nil {
				c.logger.Debug("service install failure", "error", err)
			}
		}
	} else {
		err = fmt.Errorf("%s not supported for installation", initStyle)
	}

	return
}

func (c *ServiceInstallCommand) installSystemd(exePath, configPath string) (err error) {
	var ifile *os.File

	servicePath := c.systemdServicePath(exePath)

	if utility.FileExists(servicePath) {
		return errors.New("service is already installed")
	}

	if ifile, err = os.OpenFile(servicePath, os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		c.logger.Debug("create service file failure", "path", servicePath, "error", err)
	} else {
		defer ifile.Close()

		if _, err = ifile.WriteString(fmt.Sprintf(SYSTEMD_TEMPLATE, exePath, configPath)); err != nil {
			c.logger.Debug("service file write failure", "path", servicePath, "error", err)
		} else {
			ifile.Close()

			if exitCode, out := utility.ExecuteWithOutput(exec.Command("systemctl", "--user", "enable", servicePath)); exitCode != 0 {
				c.logger.Debug("service enable failure", "path", servicePath, "exitcode", exitCode, "output", out)
				err = errors.New("failed to enable service")
			} else if exitCode, out := utility.ExecuteWithOutput(exec.Command("systemctl", "--user", "start", path.Base(servicePath))); exitCode != 0 {
				c.logger.Debug("service start failure", "name", path.Base(servicePath), "exitcode", exitCode, "output", out)
				err = errors.New("failed to start service")
			}
		}
	}

	return err
}

func (c *ServiceUninstallCommand) getInitStyle() string {
	initStyle := c.Config.Init

	if initStyle == "" {
		initStyle = c.detectInit()
	}

	return initStyle
}

func (c *ServiceUninstallCommand) uninstall() (err error) {
	exePath := ""
	initStyle := c.getInitStyle()

	if exePath, err = os.Executable(); err != nil {
		c.logger.Debug("path detection failure", "error", err)
	} else {

		c.logger.Trace("uninstalling service", "init", initStyle, "exe", exePath)

		if initStyle == "systemd" {
			if err = c.uninstallSystemd(exePath); err != nil {
				c.logger.Debug("service install failure", "error", err)
			}
		} else {
			err = fmt.Errorf("%s not supported for installation", initStyle)
		}
	}

	return
}

func (c *ServiceUninstallCommand) uninstallSystemd(exePath string) (err error) {
	servicePath := c.systemdServicePath(exePath)
	serviceName := path.Base(servicePath)

	// Check if service is enabled
	exitCode, out := utility.ExecuteWithOutput(exec.Command("systemctl", "--user", "is-enabled", serviceName))

	c.logger.Trace("service enable check", "name", serviceName, "exitcode", exitCode, "output", out)

	if exitCode == 0 {
		exitCode, out = utility.ExecuteWithOutput(exec.Command("systemctl", "--user", "disable", serviceName))

		c.logger.Trace("service disable", "name", serviceName, "exitcode", exitCode, "output", out)

		if exitCode != 0 {
			return errors.New("failed to disable service")
		}

		// clean up systemd unit list
		utility.Execute(exec.Command("systemctl", "--user", "reset-failed"))
	}

	if utility.FileExists(servicePath) {
		if err = os.Remove(servicePath); err != nil {
			c.logger.Warn("service file remove failure", "path", servicePath, "error", err)

			err = fmt.Errorf("failed to remove systemd unit file, %v", err)
		}
	}

	return err
}
