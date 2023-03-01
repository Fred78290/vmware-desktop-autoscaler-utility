package utility

import (
	"path/filepath"
)

func installDirectory() string {
	return ExpandPath(filepath.Join("%systemdrive%", "ProgramData",
		"AlduneLabs", "vmware-desktop-autoscaler-utility"))
}
