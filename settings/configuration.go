package settings

import "time"

type CommonConfig struct {
	Driver           string
	LicenseOverride  string
	LogDisplay       bool
	VMRestURL        string
	Timeout          time.Duration
	VMFolder         string
	Address          string
	Port             int64
	Listen           string
	Plisten          *string        `hcl:"listen"`
	Paddress         *string        `hcl:"address"`
	Pport            *int64         `hcl:"port"`
	Ptimeout         *time.Duration `hcl:"timeout"`
	Pvmrest          *string        `hcl:"vmrest"`
	Pdriver          *string        `hcl:"driver"`
	PlicenseOverride *string        `hcl:"license_override"`
	Pvmfolder        *string        `hcl:"vmfolder"`
}
