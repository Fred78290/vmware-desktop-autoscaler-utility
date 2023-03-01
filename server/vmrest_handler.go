package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
)

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type RestResponse struct {
	Error  *Error      `json:"error,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func newResponseWithKeyValue(keyvalues ...string) RestResponse {
	result := make(map[string]string)

	for i := 0; i < len(keyvalues); i += 2 {
		key := keyvalues[i]
		value := keyvalues[i+1]

		result[key] = value
	}

	return RestResponse{
		Result: result,
	}
}

func newResponse(result interface{}) RestResponse {
	return RestResponse{
		Result: result,
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func (r *RegexpHandler) handleVmrestProxy(wr http.ResponseWriter, req *http.Request) {

	r.netLock.Lock()
	defer r.netLock.Unlock()

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocal scheme " + req.URL.Scheme

		http.Error(wr, msg, http.StatusBadRequest)

		r.logger.Error(msg)
	} else {
		var client vagrant_driver.Client
		var ok bool

		//http: Request.RequestURI can't be set in client requests.
		//http://golang.org/src/pkg/net/http/client.go
		req.RequestURI = ""

		delHopHeaders(req.Header)

		if client, ok = r.api.Driver.GetDriver().(vagrant_driver.Client); !ok {
			target := fmt.Sprintf("http://localhost:8697%s", req.URL.Path)

			if newreq, err := http.NewRequest(req.Method, target, req.Body); err != nil {
				msg := fmt.Sprintf("Unable to create request: %v", err)

				http.Error(wr, msg, http.StatusBadRequest)
				r.logger.Error(msg)

				return
			} else {
				copyHeader(req.Header, newreq.Header)

				client = &http.Client{
					Timeout: 30 * time.Second,
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					},
				}

				if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
					appendHostToXForwardHeader(req.Header, clientIP)
				}

				req = newreq
			}

		}

		if resp, err := client.Do(req); err != nil {
			http.Error(wr, "Server Error", http.StatusInternalServerError)

			r.logger.Error("ServeHTTP: %v", err)
		} else {
			defer resp.Body.Close()

			delHopHeaders(resp.Header)

			copyHeader(wr.Header(), resp.Header)

			wr.WriteHeader(resp.StatusCode)
			io.Copy(wr, resp.Body)
		}
	}
}

func (r *RegexpHandler) handleCreateVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "POST" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("create vm")

		if done, err := r.vmrun.Delete(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", utils.BoolToStr(done)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleDeleteVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "DELETE" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm delete request", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.Delete(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", utils.BoolToStr(done)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handlePowerOnVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm power on", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.PowerOn(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", utils.BoolToStr(done)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handlePowerOffVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm power off", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.PowerOff(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", utils.BoolToStr(done)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleShutdownGuestVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm shutdown", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.ShutdownGuest(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", utils.BoolToStr(done)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleStatusVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm shutdown", "vmuuid", params["vmuuid"])

		if status, err := r.vmrun.Status(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponse(status), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleWaitForIP(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm wait for ip", "vmuuid", params["vmuuid"])

		if address, err := r.vmrun.WaitForIP(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("address", address), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleWaitForToolsRunning(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm wait tools running", "vmuuid", params["vmuuid"])

		if running, err := r.vmrun.WaitForToolsRunning(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("running", utils.BoolToStr(running)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleSetAutoStart(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm set autostart", "vmuuid", params["vmuuid"], "autostart", params["autostart"])

		if autostart, err := r.vmrun.SetAutoStart(params["vmuuid"], params["autostart"] == "true"); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponseWithKeyValue("autostart", utils.BoolToStr(autostart)), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleVirtualMachineByName(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm by name", "name", params["name"])

		if vm, err := r.vmrun.VirtualMachineByName(params["name"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponse(vm), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleVirtualMachineByUUID(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm by uuid", "vmuuid", params["vmuuid"])

		if vm, err := r.vmrun.VirtualMachineByUUID(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), 404)
		} else {
			r.respond(wr, newResponse(vm), 200)
		}
	} else {
		r.notFound(wr)
	}
}

func (r *RegexpHandler) handleListVirtualMachines(wr http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("list vm")

		if vms, err := r.vmrun.ListVirtualMachines(); err != nil {
			r.error(wr, err.Error(), 500)
		} else {
			r.respond(wr, newResponse(vms), 200)
		}
	} else {
		r.notFound(wr)
	}
}
