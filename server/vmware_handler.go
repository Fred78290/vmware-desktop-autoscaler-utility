package server

import (
	"net/http"

	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type LicenseFeature struct {
	Product string `json:"product"`
	Version string `json:"version"`
}

type VagrantVmwareValidate struct {
	Features []LicenseFeature `json:"features"`
}

func (r *RegexpHandler) handleVmwareInfo(writ http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		r.logger.Debug("vmware info")
		r.getVmwareInfo(writ)
	default:
		r.notFound(writ)
	}
}

func (r *RegexpHandler) handleVmwarePaths(writ http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		r.logger.Debug("vmware paths")
		paths, err := utility.LoadVmwarePaths(r.logger)
		if err != nil {
			r.error(writ, err.Error(), 400)
			return
		}
		r.respond(writ, paths, 200)
	default:
		r.notFound(writ)
	}
}

func (r *RegexpHandler) getVmwareInfo(writ http.ResponseWriter) {
	info, err := r.api.Driver.GetDriver().VmwareInfo()
	if err != nil {
		r.logger.Debug("vmware info error", "error", err)
		r.error(writ, err.Error(), 400)
		return
	}
	r.logger.Trace("vmware version info", "version", info.Version, "product", info.Product,
		"type", info.Type, "build", info.Build)
	r.respond(writ, info, 200)
}
