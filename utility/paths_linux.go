//go:build !windows
// +build !windows

package utility

import (
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func installDirectory() string {
	if home, err := os.UserHomeDir(); err != nil {
		return ""
	} else {
		idir := path.Join(home, ".local/vmware/autoscaler-utility/vmware-desktop-autoscaler-utility")
		exePath, err := os.Executable()
		if err == nil && !strings.HasPrefix(exePath, idir) {
			idir = filepath.Dir(exePath)
		}
		return idir
	}
}

func directoryForVirtualMachine(vmfolder, name string) string {
	return path.Join(vmfolder, name, name+".vmx")
}

func certificatDirectory() string {
	if home, err := os.UserHomeDir(); err != nil {
		return ""
	} else {
		home = path.Join(home, ".local/vmware/autoscaler-utility")
		if _, err := os.Stat(home); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(home, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}

		return home
	}
}

func vmfolderDirectory() string {
	if home, err := os.UserHomeDir(); err != nil {
		return ""
	} else {
		home = path.Join(home, ".local/vmware/vm")
		if _, err := os.Stat(home); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(home, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}

		return home
	}
}
