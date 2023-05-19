package settings

import "time"

type CommonConfig struct {
	Address         string
	Driver          string
	LicenseOverride string
	Listen          string
	LogDisplay      bool
	Port            int64
	Timeout         time.Duration
	VMFolder        string
	VMRestURL       string

	Paddress         *string        `hcl:"address"`
	Pdriver          *string        `hcl:"driver"`
	PlicenseOverride *string        `hcl:"license_override"`
	Plisten          *string        `hcl:"listen"`
	Pport            *int64         `hcl:"port"`
	Ptimeout         *time.Duration `hcl:"timeout"`
	Pvmfolder        *string        `hcl:"vmfolder"`
	Pvmrest          *string        `hcl:"vmrest"`
}
