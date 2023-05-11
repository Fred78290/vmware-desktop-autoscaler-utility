package utility

import (
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func expandPath(ePath string) string {
	expandedPath := strings.ToLower(ePath)
	systemDrive := os.Getenv("SystemRoot")[0:2]
	expandedPath = strings.Replace(expandedPath, "%homedrive%", os.Getenv("HOMEDRIVE"), -1)
	expandedPath = strings.Replace(expandedPath, "%systemroot%", os.Getenv("SystemRoot"), -1)
	expandedPath = strings.Replace(expandedPath, "%systemdrive%", systemDrive, -1)
	return expandedPath
}

func installDirectory() string {
	return expandPath(filepath.Join("%systemdrive%", "ProgramData",
		"AlduneLabs", "vmware-desktop-autoscaler-utility"))
}

func certificatDirectory() string {
	return expandPath(filepath.Join(os.UserHomeDir(), "AppData"
		"AlduneLabs", "vmware-desktop-autoscaler-utility"))
}

func directoryForVirtualMachine(vmfolder, name string) string {
	return path.Join(vmfolder, name, name+".vmx")
}

func vmfolderDirectory() string {
	if home, err := os.UserHomeDir(); err != nil {
		return ""
	} else {
		home = path.Join(home, "Virtual Machines")
		if _, err := os.Stat(home); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(home, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}

		return home
	}
}
