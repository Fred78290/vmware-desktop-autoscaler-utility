package utility

import (
	"path/filepath"
)

func InstallDirectory() string {
	return installDirectory()
}

func CertificatDirectory() string {
	return certificatDirectory()
}

func VMFolder() string {
	return vmfolderDirectory()
}

func DirectoryForVirtualMachine(vmfolder, name string) string {
	return directoryForVirtualMachine(vmfolder, name)
}

func DirectoryFor(thing string) string {
	return filepath.Join(installDirectory(), thing)
}

func DirectoryForConfig(thing string) string {
	return filepath.Join(directoryForConfig(), thing)
}
