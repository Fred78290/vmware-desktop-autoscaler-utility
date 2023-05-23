package driver

import (
	"context"
	"errors"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/hashicorp/go-hclog"
)

func buildDriver(driverName string, b *BaseDriver, logger hclog.Logger) (drv Driver, attempt bool, err error) {
	attempt = true

	switch driverName {
	case "simple":
		logger.Warn("creating simple driver via user request")
		drv, err = NewSimpleDriver(nil, b, logger)
		attempt = false

	case "advanced":
		logger.Warn("creating advanced driver via user request")
		drv, err = NewAdvancedDriver(nil, b, logger)
		attempt = false

	default:
		if driverName != "" {
			logger.Warn("unknown driver name provided, detecting appropriate driver", "name", driverName)
		}
		drv, err = CreateDriver(nil, b, logger)
	}

	if err != nil {
		logger.Error("driver setup failure", "error", err)
		return nil, false, errors.New("failed to setup VMWare desktop utility driver - " + err.Error())
	}

	return
}

func NewVMRestDriver(config *settings.CommonConfig, logger hclog.Logger) (drv Driver, err error) {
	// Start with building the base driver
	if b, err := NewBaseDriver(nil, config, logger); err != nil {
		logger.Error("base driver setup failure", "error", err)
		return nil, err
	} else {
		var attempt bool

		// Allow the user to define the driver. It may not work, but they're the boss
		if drv, attempt, err = buildDriver(config.Driver, b, logger); err != nil {
			return nil, err
		}

		// Now that we are setup, we can attempt to upgrade the driver to the
		// vmrest driver if possible or requested
		if attempt {
			logger.Info("attempting to upgrade to vmrest driver")

			if drv, err = NewVmrestDriver(context.Background(), config, drv, logger); err != nil {
				logger.Error("failed to upgrade to vmrest driver", "error", err)
				return nil, err
			}
		}

		if !drv.GetDriver().Validate() {
			// NOTE: We only log the failure and allow the process to start. This
			//       lets the plugin communicate with the service, but all requests
			//       result in an error which includes the validation failure.
			logger.Error("vmware validation failed")
		}
	}

	return
}
