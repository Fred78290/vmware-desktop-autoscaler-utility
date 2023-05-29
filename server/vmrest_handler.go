package server

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
)

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type RestResponse struct {
	Error  *Error      `json:"error,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

type VMDetail struct {
	*service.VirtualMachine
	EthernetCards []*service.EthernetCard `json:"ethernet,omitempty"`
}

var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func newResponseWithKeyValue(keyvalues ...interface{}) RestResponse {
	result := make(map[string]interface{})

	for i := 0; i < len(keyvalues); i += 2 {
		key := keyvalues[i]
		value := keyvalues[i+1]

		strKey := key.(string)

		result[strKey] = value
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

	httperror := func(code int, err error) {
		wr.Header().Set("Content-Type", driver.VMREST_CONTENT_TYPE)

		msg := utils.ToJSON(map[string]interface{}{
			"code":    code,
			"message": err.Error(),
		})

		http.Error(wr, msg, code)
		r.logger.Error("ServeHTTP: %v", err)
	}

	forwardResponse := func(resp *http.Response) {
		defer resp.Body.Close()

		delHopHeaders(resp.Header)

		copyHeader(wr.Header(), resp.Header)

		wr.WriteHeader(resp.StatusCode)
		io.Copy(wr, resp.Body)
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		httperror(http.StatusBadRequest, fmt.Errorf("unsupported protocal scheme "+req.URL.Scheme))
	} else {
		var vmrestdriver *driver.VmrestDriver
		var ok bool

		delHopHeaders(req.Header)

		if vmrestdriver, ok = r.api.Driver.(*driver.VmrestDriver); !ok {
			target := fmt.Sprintf("http://localhost:8697%s", req.URL.Path)

			if newreq, err := http.NewRequest(req.Method, target, req.Body); err != nil {
				httperror(http.StatusBadRequest, fmt.Errorf("unable to create request: %v", err))
			} else {
				copyHeader(req.Header, newreq.Header)

				client := &http.Client{
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

				if resp, err := client.Do(req); err != nil {
					httperror(http.StatusInternalServerError, err)
				} else {
					forwardResponse(resp)
				}
			}
		} else if resp, err := vmrestdriver.Request(req.Method, req.URL.Path, req.Body); err == nil {
			forwardResponse(resp)
		} else {
			httperror(http.StatusInternalServerError, err)
		}
	}
}

func (r *RegexpHandler) handleCreateVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	var vmdefs service.CreateVirtualMachine

	if req.Method == "POST" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("create vm")

		if err := r.readBody(req, &vmdefs); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else if vm, err := r.vmrun.Create(&vmdefs); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponse(&vm), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleDeleteVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "DELETE" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm delete request", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.Delete(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", done), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handlePowerOnVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm power on", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.PowerOn(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", done), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handlePowerOffVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm power off", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.PowerOff(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", done), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handlePowerStateVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm power state", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.PowerState(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("powered", done), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleShutdownGuestVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm shutdown", "vmuuid", params["vmuuid"])

		if done, err := r.vmrun.ShutdownGuest(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", done), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleStatusVirtualMachine(wr http.ResponseWriter, req *http.Request) {
	r.handleVirtualMachineByUUID(wr, req)
}

func (r *RegexpHandler) handleWaitForIP(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm wait for ip", "vmuuid", params["vmuuid"])

		if address, err := r.vmrun.WaitForIP(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("address", address), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleWaitForToolsRunning(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm wait tools running", "vmuuid", params["vmuuid"])

		if running, err := r.vmrun.WaitForToolsRunning(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("running", running), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleSetAutoStart(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "PUT" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm set autostart", "vmuuid", params["vmuuid"], "autostart", params["autostart"])

		if autostart, err := r.vmrun.SetAutoStart(params["vmuuid"], params["autostart"] == "true"); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("autostart", autostart), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleVirtualMachineByName(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm by name", "name", params["name"])

		var detail VMDetail
		var err error
		var status *service.VirtualMachineStatus

		if detail.VirtualMachine, err = r.vmrun.VirtualMachineByName(params["name"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else if status, err = r.vmrun.Status(detail.Uuid); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else {
			detail.EthernetCards = status.EthernetCards
			r.respond(wr, newResponse(&detail), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) handleVirtualMachineByUUID(wr http.ResponseWriter, req *http.Request) {
	params := r.pathParams(req.URL.Path)

	if req.Method == "GET" {
		r.netLock.Lock()
		defer r.netLock.Unlock()

		r.logger.Debug("vm by uuid", "vmuuid", params["vmuuid"])

		var detail VMDetail
		var err error
		var status *service.VirtualMachineStatus

		if detail.VirtualMachine, err = r.vmrun.VirtualMachineByUUID(params["vmuuid"]); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else if status, err = r.vmrun.Status(detail.Uuid); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else {
			detail.EthernetCards = status.EthernetCards
			r.respond(wr, newResponse(&detail), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

type ParamVnet struct {
	Vnet string
	Nic  int
}

func (r *RegexpHandler) handleNetworkInterface(wr http.ResponseWriter, req *http.Request) {
	var vnet ParamVnet

	params := r.pathParams(req.URL.Path)

	r.netLock.Lock()
	defer r.netLock.Unlock()

	vmuuid := params["vmuuid"]

	r.logger.Debug("vnet by uuid", "vmuuid", vmuuid)

	if req.Method == "GET" {
		if info, err := r.vmrun.Status(vmuuid); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else {
			r.respond(wr, newResponse(info.EthernetCards), http.StatusOK)
		}
	} else if req.Method == "POST" {
		if err := r.readBody(req, &vnet); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else if err = r.vmrun.AddNetworkInterface(vmuuid, vnet.Vnet); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", true), http.StatusOK)
		}
	} else if req.Method == "PUT" {
		if err := r.readBody(req, &vnet); err != nil {
			r.error(wr, err.Error(), http.StatusInternalServerError)
		} else if err = r.vmrun.ChangeNetworkInterface(vmuuid, vnet.Vnet, vnet.Nic); err != nil {
			r.error(wr, err.Error(), http.StatusNotFound)
		} else {
			r.respond(wr, newResponseWithKeyValue("done", true), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
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
			r.respond(wr, newResponse(vms), http.StatusOK)
		}
	} else {
		r.notSupported(wr)
	}
}

func (r *RegexpHandler) readBody(req *http.Request, target interface{}) error {
	defer req.Body.Close()

	if body, err := io.ReadAll(req.Body); err != nil {
		return err
	} else {
		contentType := req.Header.Get("Content-Type")

		if strings.Contains(contentType, "xml") {
			if err = xml.Unmarshal(body, target); err != nil {
				return err
			}
		} else if strings.Contains(contentType, "json") {
			if err = json.Unmarshal(body, target); err != nil {
				return err
			}
		}
	}

	return nil
}
