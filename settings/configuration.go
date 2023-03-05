package settings

import "time"

type CommonConfig struct {
	Listen          string
	Port            int64
	Driver          string
	LicenseOverride string
	LogDisplay      bool
	VMRestURL       string
	Timeout         time.Duration
	VMFolder        string

	Ptimeout         *time.Duration `hcl:"timeout"`
	Plisten          *string        `hcl:"listen"`
	Pport            *int64         `hcl:"port"`
	Pvmrest          *string        `hcl:"vmrest"`
	Pdriver          *string        `hcl:"driver"`
	PlicenseOverride *string        `hcl:"license_override"`
	Pvmfolder        *string        `hcl:"vmfolder"`
}
