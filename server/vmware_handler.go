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
		if paths, err := utility.LoadVmwarePaths(r.logger); err != nil {
			r.error(writ, err.Error(), http.StatusBadRequest)
		} else {
			r.respond(writ, paths, http.StatusOK)
		}
	default:
		r.notFound(writ)
	}
}

func (r *RegexpHandler) getVmwareInfo(writ http.ResponseWriter) {
	if info, err := r.api.Driver.GetDriver().VmwareInfo(); err != nil {
		r.logger.Debug("vmware info error", "error", err)
		r.error(writ, err.Error(), http.StatusBadRequest)
	} else {
		r.logger.Trace("vmware version info", "version", info.Version, "product", info.Product, "type", info.Type, "build", info.Build)
		r.respond(writ, info, http.StatusOK)

	}
}
