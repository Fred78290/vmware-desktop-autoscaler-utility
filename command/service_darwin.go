package command

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/service"
	vagrant_utility "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

// Expected variables:
// * string - executable path
// * integer - listen port
// * string - configuration file path
// * integer - listen port
// * string - log file path
// * string - log file path
const LAUNCHD_JOB = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.aldunelabs.vmware-desktop-autoscaler-utility</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>full</string>
        <string>-config-file=%s</string>
    </array>
    <key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
    <key>RunAtLoad</key>
        <true/>
    <key>StandardErrorPath</key>
        <string>%s</string>
    <key>StandardOutPath</key>
        <string>%s</string>
    <key>AbandonProcessGroup</key>
        <true/>
	  <!--
	    default is 256 which can be low if we are handling port
	    forwarding so increase to a reasonably larger amount
	  -->
	  <key>SoftResourceLimits</key>
	  <dict>
	      <key>NumberOfFiles</key>
	      <integer>4096</integer>
	  </dict>
</dict>
</plist>
`

var LAUNCHD_JOB_PATH = "%s/Library/LaunchAgents/com.aldunelabs.vmware-desktop-autoscaler-utility.plist"
var SERVICE_CONFIGURATION_FILE = "%s/Library/Application Support/vmware-desktop-autoscaler-utility/config.hcl"
var SERVICE_LOG_FILE = "%s/Library/Application Support/vmware-desktop-autoscaler-utility/service.log"

func init() {
	homeDir, _ := os.UserHomeDir()

	LAUNCHD_JOB_PATH = fmt.Sprintf(LAUNCHD_JOB_PATH, homeDir)
	SERVICE_CONFIGURATION_FILE = fmt.Sprintf(SERVICE_CONFIGURATION_FILE, homeDir)
	SERVICE_LOG_FILE = fmt.Sprintf(SERVICE_LOG_FILE, homeDir)
}

func (c *ServiceInstallCommand) install() error {
	if vagrant_utility.FileExists(LAUNCHD_JOB_PATH) {
		return errors.New("service is already installed")
	}

	exePath, err := os.Executable()
	if err != nil {
		c.logger.Error("failed to determine executable path", "error", err)
		return errors.New("failed to determine executable path")
	}

	config, err := c.writeConfig(SERVICE_CONFIGURATION_FILE)
	if err != nil {
		c.logger.Error("failed to create service configuration", "error", err)
		return err
	}

	launchctl, err := service.NewLaunchctl(c.logger)
	if err != nil {
		c.logger.Debug("launchctl service creation failure", "error", err)
		return err
	}

	c.logger.Trace("create service file", "path", LAUNCHD_JOB_PATH)
	lfile, err := os.Create(LAUNCHD_JOB_PATH)
	if err != nil {
		c.logger.Debug("create service file failure", "path", LAUNCHD_JOB_PATH, "error", err)
		return err
	}

	defer lfile.Close()

	if _, err = lfile.WriteString(fmt.Sprintf(LAUNCHD_JOB, exePath, config, SERVICE_LOG_FILE, SERVICE_LOG_FILE)); err != nil {
		c.logger.Debug("service file write failure", "path", LAUNCHD_JOB_PATH, "error", err)
		return err
	}

	c.logger.Trace("loading service", "path", LAUNCHD_JOB_PATH)
	if err = launchctl.Load(LAUNCHD_JOB_PATH); err != nil {
		c.logger.Debug("service load failure", "path", LAUNCHD_JOB_PATH, "error", err)
		return err
	}

	return nil
}

func (c *ServiceInstallCommand) print() error {
	return errors.New("service setup printing unavailable on darwin")
}

func (c *ServiceUninstallCommand) uninstall() error {
	if !vagrant_utility.FileExists(LAUNCHD_JOB_PATH) {
		c.logger.Warn("service is not currently installed")
		return nil
	}

	launchctl, err := service.NewLaunchctl(c.logger)

	if err != nil {
		c.logger.Debug("launchctl service creation failure", "error", err)
		return err
	}

	c.logger.Trace("unloading service", "path", LAUNCHD_JOB_PATH)
	if err = launchctl.Unload(LAUNCHD_JOB_PATH); err != nil {
		c.logger.Debug("service unload failure", "path", LAUNCHD_JOB_PATH, "error", err)
		return err
	}

	c.logger.Trace("removing service file", "path", LAUNCHD_JOB_PATH)
	if err = os.Remove(LAUNCHD_JOB_PATH); err != nil {
		c.logger.Debug("service file remove failure", "path", LAUNCHD_JOB_PATH, "error", err)
		return err
	}

	return nil
}
