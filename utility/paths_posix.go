//go:build !windows
// +build !windows

package utility

import (
	"os"
	"path/filepath"
	"strings"
)

func installDirectory() string {
	idir := "/opt/vmware-desktop-autoscaler-utility"
	exePath, err := os.Executable()
	if err == nil && !strings.HasPrefix(exePath, idir) {
		idir = filepath.Dir(exePath)
	}
	return idir
}
