package driver

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	apiclient "github.com/Fred78290/vmrest-go-client/client"

	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	hclog "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/go-retryablehttp"
	vagrant_version "github.com/hashicorp/go-version"
	vagrant_driver "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/driver"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/util"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
	"golang.org/x/crypto/bcrypt"
)

var Shutdown sync.Cond

type Client interface {
	Do(req *http.Request) (r *http.Response, err error)
}

type VmrestDriver struct {
	vagrant_driver.BaseDriver
	ExtendedDriver
	client      Client
	ctx         context.Context
	isBigSurMin bool
	fallback    Driver
	vmrest      *vmrest
	logger      hclog.Logger
}

type vmrest struct {
	access      sync.Mutex
	activity    chan struct{}
	command     *exec.Cmd
	config_path string
	ctx         context.Context
	home        string
	logger      hclog.Logger
	path        string
	password    string
	port        int
	username    string
	vmrestURL   string
}

const lowers = "abcdefghijklmnopqrstuvwxyz"
const uppers = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const numbers = "0123456789"
const symbols = "!#$%&'()*+,-./:;<=>?@[]^_`{|}~"

const HOME_DIR_ENV = "HOME"
const VMREST_VERSION_CONSTRAINT = ">= 1.2.0"
const VMREST_URL = "http://localhost:%d/api"
const VMREST_CONFIG = ".vmrestCfg"
const WINDOWS_VMREST_CONFIG = "vmrest.cfg"
const VMREST_CONTENT_TYPE = "application/vnd.vmware.vmw.rest-v1+json"
const VMREST_VAGRANT_DESC = "vagrant: managed port"
const VMREST_KEEPALIVE_SECONDS = 300
const DESKTOP_CONFIG = "desktop-autoscaler-utility.cfg"
const VMWARE_NETDEV_PREFIX = "vmnet"
const VAGRANT_NETDEV_PREFIX = "vgtnet"

type ConfigStorage struct {
	User     string `json:"username"`
	Password string `json:"password"`
}

func (v *vmrest) Init() error {
	var err error

	if len(v.vmrestURL) > 0 {
		var u *url.URL

		if u, err = url.Parse(v.vmrestURL); err != nil {
			return err
		}

		v.password, _ = u.User.Password()
		v.username = u.User.Username()

		return nil
	}

	if err = v.validate(); err != nil {
		return err
	} else {
		if v.home, err = os.UserHomeDir(); err != nil {
			v.logger.Trace("failed to determine user home directory", "error", err)
			return err
		}

		if u, err := user.Current(); err != nil {
			v.logger.Trace("failed to determine current user", "error", err)
			return err
		} else {
			var configPath string

			if v.isWindows() {
				// On Windows, home directory will be based on the user
				// running the command. When running as a service under
				// the SYSTEM user, the path returned by os.UserHomeDir()
				// will be incorrect due to us being a 64bit executable
				// and vmrest.exe being a 32 bit executable. The home
				// directory ends up being different for 64 and 32 bit
				// executables for the SYSTEM user, so if the SYSTEM
				// user is detected as running we must use a customized
				// path to drop the configuration file in the right place
				// On Windows, home must be %USERPROFILE%
				if strings.ToLower(u.Name) == "system" {
					home := v.home
					v.home = strings.Replace(v.home, "system32", "SysWOW64", 1)
					v.logger.Info("modified user home directory for SYSTEM", "user", u.Name, "original", home, "updated", v.home)
				}

				v.config_path = path.Join(v.home, WINDOWS_VMREST_CONFIG)
				configPath = path.Join(v.home, DESKTOP_CONFIG)
			} else {
				v.config_path = path.Join(v.home, VMREST_CONFIG)
				configPath = path.Join(v.home, ".local", "vmware", DESKTOP_CONFIG)
				utils.MkDir(path.Dir(configPath))
			}

			config := &ConfigStorage{}

			if utils.FileExists(v.config_path) {
				if !utils.FileExists(configPath) {
					v.logger.Warn(fmt.Sprintf("the config: %s, doesn't exist. create it as { \"username\": \"<username>\", \"password\": \"<password>\" } with values from %s or remove vmrestCfg", configPath, v.config_path))
					return fmt.Errorf("the config: %s, doesn't exist", configPath)
				}

				if err = utils.LoadJsonFromFile(configPath, config); err != nil {
					return err
				}

				v.username = config.User
				v.password = config.Password

				// Avoid conflict generate new port
				if v.port, err = v.portgen(); err != nil {
					return err
				}
			} else {
				if v.username, err = v.stringgen(false, 0); err != nil {
					return err
				}

				if v.password, err = v.stringgen(true, 0); err != nil {
					return err
				}

				if v.port, err = v.portgen(); err != nil {
					return err
				}

				if err = v.configure(); err != nil {
					return err
				}

				config.User = v.username
				config.Password = v.password

				if err = utils.StoreJsonToFile(configPath, config); err != nil {
					return err
				}
			}
		}
	}

	v.logger.Trace("process configuration", "home", v.home, "username", v.username, "password", v.password, "port", v.port)

	util.RegisterShutdownTask(v.Cleanup)

	go v.Runner()

	return nil
}

func (v *vmrest) Cleanup() {
	if len(v.vmrestURL) == 0 {
		if v.command != nil {
			v.logger.Debug("halting running process")
			v.command.Process.Kill()
		}
	}
}

func (v *vmrest) Active() (url string) {
	if len(v.vmrestURL) == 0 {
		v.activity <- struct{}{}
	}

	return v.URL()
}

func (v *vmrest) URL() string {
	if len(v.vmrestURL) == 0 {
		return fmt.Sprintf(VMREST_URL, v.port)
	} else {
		return v.vmrestURL + "api"
	}
}

func (v *vmrest) Username() string {
	return v.username
}

func (v *vmrest) Password() string {
	return v.password
}

func (v *vmrest) Port() int {
	return v.port
}

func (v *vmrest) UserAgent() string {
	return utils.UserAgent()
}

func (v *vmrest) runCommand() bool {
	v.logger.Trace("activity request detected")

	if v.command == nil {
		v.logger.Debug("starting the process")
		v.command = exec.Command(v.path, "-p", strconv.Itoa(v.port))

		// Grab output from the process and send it to the logger.
		// Useful for debugging if something goes wrong so we can
		// see what the process is actually doing.
		stderr, err := v.command.StderrPipe()

		if err != nil {
			v.logger.Error("failed to get stderr pipe", "error", err)
			return true
		}

		stdout, err := v.command.StdoutPipe()

		if err != nil {
			v.logger.Error("failed to get stdout pipe", "error", err)
			return true
		}

		go func() {
			r := bufio.NewReader(stdout)

			for {
				if l, _, err := r.ReadLine(); err != nil {
					v.logger.Warn("stdout pipe error", "error", err)
					break
				} else {
					v.logger.Info("vmrest stdout", "output", string(l))
				}

			}
		}()

		go func() {
			r := bufio.NewReader(stderr)

			for {
				if l, _, err := r.ReadLine(); err != nil {
					v.logger.Warn("stderr pipe error", "error", err)
					break
				} else {
					v.logger.Info("vmrest stderr", "output", string(l))
				}
			}
		}()

		if err = v.homedStart(v.command); err != nil {
			v.logger.Error("failed to start", "error", err)
			return true
		}

		if _, err = os.FindProcess(v.command.Process.Pid); err != nil {
			v.logger.Error("failed to locate started vmrest process", "error", err)
			return true
		}

		// Start a cleanup function to prevent any unnoticed zombies from
		// hanging around
		go func() {
			v.command.Wait()
			v.command = nil
			v.logger.Debug("process has been completed and reaped")
		}()

		v.logger.Debug("process has been started")
	}

	/*		case <-time.After(VMREST_KEEPALIVE_SECONDS * time.Second):
			if v.command != nil {
				v.logger.Debug("halting running process")
				v.command.Process.Kill()
			}
	*/
	return false
}

func (v *vmrest) Runner() {
	if len(v.vmrestURL) == 0 {
		for {
			select {
			case <-v.activity:
				if v.runCommand() {
					continue
				}
			case <-v.ctx.Done():
				v.logger.Warn("halting due to context done")
				if v.command != nil {
					v.command.Process.Kill()
				}
			}
		}
	}
}

func (v *vmrest) isWindows() bool {
	return strings.HasSuffix(v.path, ".exe")
}

func (v *vmrest) homedStart(cmd *exec.Cmd) error {
	v.access.Lock()

	defer v.access.Unlock()

	if !v.isWindows() {
		// Ensure our home directory is set to properly pickup config
		curHome := os.Getenv(HOME_DIR_ENV)

		if err := os.Setenv(HOME_DIR_ENV, v.home); err != nil {
			v.logger.Error("failed to set HOME environment variable, cannot start", "error", err)
			return err
		}

		defer os.Setenv(HOME_DIR_ENV, curHome)
	}

	return cmd.Start()
}

func (v *vmrest) configure() error {
	if f, err := os.OpenFile(v.config_path, os.O_RDWR|os.O_CREATE, 0644); err != nil {
		v.logger.Error("failed to create config file", "error", err)

		return errors.New("failed to configure process")
	} else {

		defer f.Close()

		if salt, err := v.stringgen(true, 16); err != nil {
			v.logger.Error("failed to create salt config", "error", err)
			return errors.New("failed to generate config information")
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(salt+v.password), bcrypt.DefaultCost)

			if err != nil {
				v.logger.Error("failed to hash password", "error", err)
				return errors.New("failed to generate config hash")
			}

			if _, err = f.Write([]byte(fmt.Sprintf("port=%d\r\nusername=%s\r\npassword=%s\r\nsalt=%s\r\n", v.port, v.username, hash, salt))); err != nil {
				v.logger.Error("failed to write config file", "error", err)
				return errors.New("failed to store config")
			}
		}
	}

	return nil
}

func (v *vmrest) portgen() (int, error) {
	// Let the system generate a free port for us
	if a, err := net.ResolveTCPAddr("tcp", "localhost:0"); err != nil {

		v.logger.Trace("failed to setup free port detection", "error", err)
		return 0, err

	} else if l, err := net.ListenTCP("tcp", a); err != nil {

		v.logger.Trace("failed to locate free port", "error", err)
		return 0, err

	} else {
		defer l.Close()

		return l.Addr().(*net.TCPAddr).Port, nil
	}
}

func (v *vmrest) stringgen(syms bool, l int) (string, error) {
	var collections int

	g := strings.Builder{}
	if bl, err := rand.Int(rand.Reader, big.NewInt(3)); err != nil {
		v.logger.Trace("failed to produce random value", "error", err)

		return "", err

	} else {

		if l == 0 {
			l = int(bl.Int64()) + 8
		}

		g.Grow(l)

		if syms {
			collections = 4
		} else {
			collections = 3
		}

		for i := 0; i < l; i++ {
			set := i % collections

			switch set {
			case 3:
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(symbols))))
				if err != nil {
					v.logger.Trace("failed to produce random index", "error", err)
					return "", err
				}
				g.WriteByte(symbols[idx.Int64()])

			case 2:
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(uppers))))
				if err != nil {
					v.logger.Trace("failed to produce random index", "error", err)
					return "", err
				}
				g.WriteByte(uppers[idx.Int64()])

			case 1:
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(numbers))))
				if err != nil {
					v.logger.Trace("failed to produce random index", "error", err)
					return "", err
				}
				g.WriteByte(numbers[idx.Int64()])

			default:
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(lowers))))
				if err != nil {
					v.logger.Trace("failed to produce random index", "error", err)
					return "", err
				}
				g.WriteByte(lowers[idx.Int64()])
			}
		}

		return g.String(), nil
	}
}

func (v *vmrest) validate() error {

	if !utility.FileExists(v.path) {
		v.logger.Trace("missing vmrest executable", "path", v.path)
		return errors.New("failed to locate the vmrest executable")
	}

	cmd := exec.Command(v.path, "-v")
	_, o := utility.ExecuteWithOutput(cmd)

	if m, err := utility.MatchPattern(`vmrest (?P<version>[\d+.]+) `, o); err != nil {
		v.logger.Trace("failed to determine vmrest version information", "output", o)

		return errors.New("failed to determine vmrest version")
	} else {
		v.logger.Trace("detected vmrest version", "version", m["version"])

		if constraint, err := vagrant_version.NewConstraint(VMREST_VERSION_CONSTRAINT); err != nil {
			v.logger.Warn("failed to parse vmrest constraint", "constraint", VMREST_VERSION_CONSTRAINT, "error", err)

			return errors.New("failed to setup vmrest constraint for version check")

		} else if checkV, err := vagrant_version.NewVersion(m["version"]); err != nil {
			v.logger.Warn("failed to parse vmrest version for check", "version", m["version"], "error", err)

			return errors.New("failed to parse vmrest version for validation check")

		} else {
			v.logger.Trace("validating vmrest version", "constraint", constraint, "version", checkV)

			if !constraint.Check(checkV) {
				v.logger.Warn("installed vmrest does not meet constraint requirements", "constraint", constraint, "version", checkV)
				return errors.New("vmrest version is incompatible")
			}
		}
	}

	return nil
}

func NewVmrest(ctx context.Context, externalVMRestURL string, vmrestPath string, logger hclog.Logger) (*vmrest, error) {
	v := &vmrest{
		ctx:       ctx,
		logger:    logger.Named("process"),
		vmrestURL: externalVMRestURL,
		path:      vmrestPath,
	}

	if externalVMRestURL == "" {
		v.activity = make(chan struct{})
	}

	return v, v.Init()
}

func NewVmrestDriver(ctx context.Context, c *settings.CommonConfig, f Driver, logger hclog.Logger) (Driver, error) {
	logger = logger.Named("vmrest")

	if i, err := f.VmwareInfo(); err != nil {

		logger.Warn("failed to get vmware info", "error", err)
		logger.Info("using fallback driver")

		return f, nil

	} else if i.IsStandard() {

		logger.Warn("standard vmware license detected, using fallback")
		return f, nil

	} else {

		logger.Debug("attempting to setup vmrest")

		if v, err := NewVmrest(ctx, c.VMRestURL, f.VmwarePaths().Vmrest, logger); err != nil {
			logger.Warn("failed to create vmrest driver", "error", err)
			logger.Info("using fallback driver")

			return f, err

		} else {
			var b *vagrant_driver.BaseDriver

			if s, ok := f.(*SimpleDriver); ok {
				b = &s.BaseDriver
			} else {
				if a, ok := f.(*AdvancedDriver); ok {
					b = &a.BaseDriver
				} else {
					return nil, errors.New("failed to convert to known driver type")
				}
			}

			url := v.URL()
			configuration := &apiclient.Configuration{
				Endpoint:  url[0 : len(url)-4],
				UserAgent: v.UserAgent(),
				UserName:  v.Username(),
				Password:  v.Password(),
				Timeout:   c.Timeout / time.Second,
			}

			d := &VmrestDriver{
				BaseDriver: *b,
				ExtendedDriver: ExtendedDriver{
					vmwarePaths: f.GetVmwarePaths(),
					vmrun:       f.GetVmrun(),
				},
				client:      retryablehttp.NewClient().StandardClient(),
				ctx:         ctx,
				fallback:    f,
				vmrest:      v,
				isBigSurMin: utility.IsBigSurMin(),
				logger:      logger,
			}

			if _, err = d.Vmnets(); err != nil {
				logger.Error("vmrest driver failed to access networking functions, using fallback", "status", "invalid", "error", err)
				return f, err
			}

			// License detection is not always correct so we need to validate
			// that networking functionality is available via the vmrest process
			logger.Debug("validating that vmrest service provides networking functionality")

			if d.ExtendedDriver.client, err = apiclient.NewAPIClient(configuration); err != nil {
				logger.Error("vmrest api client failed", "status", "invalid", "error", err)
				return f, err
			}

			d.ExtendedDriver.vmrun.SetApiClient(d.ExtendedDriver.client)

			logger.Debug("validation of vmrest service is complete", "status", "valid")

			return d, nil
		}
	}
}

func (d *VmrestDriver) GetDriver() vagrant_driver.Driver {
	return d
}

func (d *VmrestDriver) AddInternalPortForward(fwd *vagrant_driver.PortFwd) error {
	return d.BaseDriver.AddInternalPortForward(fwd)
}

func (v *VmrestDriver) Vmnets() (*vagrant_driver.Vmnets, error) {
	v.logger.Trace("requesting list of current vmnets")

	if r, err := v.Do("get", "vmnet", nil); err != nil {
		v.logger.Error("vmnets list request failed", "error", err)

		return nil, err
	} else {
		vmns := &vagrant_driver.Vmnets{}
		err = json.Unmarshal(r, vmns)

		v.logger.Trace("current vmnets request list", "vmnets", vmns, "error", err)

		return vmns, err
	}
}

func (v *VmrestDriver) AddVmnet(vnet *vagrant_driver.Vmnet) error {
	v.logger.Trace("adding vmnet device", "vmnet", vnet)

	// Big Sur and beyond require using vmrest for vmnet management
	if v.isBigSurMin {
		// Check if a specific address is attempting to be set. If so,
		// we need to force an error since the subnet/mask is not available
		// for modification via the vmnet framework
		if vnet.Type != "bridged" && (vnet.Mask != "" || vnet.Subnet != "") {
			return errors.New("networks with custom subnet/mask values are not supported on this platform")
		}
		// we need a name, so if one is not set provide one
		if vnet.Name == "" {
			if err := v.setVmnetName(vnet); err != nil {
				return err
			}
		}

		if f, err := json.Marshal(vnet); err != nil {
			v.logger.Error("failed to encode vmnet", "vmnet", vnet, "error", err)
			return err
		} else if _, err := v.Do("post", "vmnets", bytes.NewBuffer(f)); err != nil {
			v.logger.Error("failed to create new network", "vmnet", vnet, "error", err)
		}
		return nil
	}

	return v.fallback.AddVmnet(vnet)
}

func (v *VmrestDriver) UpdateVmnet(vnet *vagrant_driver.Vmnet) (err error) {
	v.logger.Trace("updating vmnet device (proxy to create request)", "vmnet", vnet)

	// Big Sur and beyond require using vmrest for vmnet management and
	// vmrest does not support updating existing vmnet devices
	if v.isBigSurMin {
		return errors.New("VMware does not support updating vmnet device")
	}

	return v.fallback.UpdateVmnet(vnet)
}

func (v *VmrestDriver) DeleteVmnet(vnet *vagrant_driver.Vmnet) error {
	// The vmrest interface does not provide any method for removing
	// interfaces, only creating them. We can use the fallback driver
	// here, but it may have no affect on platforms like Big Sur where
	// the VMware vmnet implementation isn't actually being used
	v.logger.Trace("deleting vmnet device", "vmnet", vnet)

	if v.isBigSurMin {
		return errors.New("VMware does not support deleting vmnet device")
	}

	return v.fallback.DeleteVmnet(vnet)
}

func (v *VmrestDriver) PortFwds(slot string) (*vagrant_driver.PortFwds, error) {
	f := &vagrant_driver.PortFwds{}

	if v.InternalPortForwarding() {
		var err error
		f.PortForwards, err = v.InternalPortFwds()
		return f, err
	}

	fwds := []*vagrant_driver.PortFwd{}

	if v.InternalPortForwarding() {
		if iFwds, err := v.InternalPortFwds(); err != nil {
			return nil, err
		} else {
			fwds = append(fwds, iFwds...)
		}
	} else {
		device := "vmnet" + slot

		if slot == "" {
			if nat, err := v.detectNAT(v); err != nil {
				return nil, err
			} else {
				device = nat.Name
			}
		}

		if slotNum, err := strconv.Atoi(string(device[len(device)-1])); err != nil {
			v.logger.Error("failed to parse slot number from device", "device", device, "error", err)
			return nil, errors.New("error parsing vmnet device name for slot")
		} else {

			v.logger.Trace("requesting list of port forwards", "device", device)
			if r, err := v.Do("get", "vmnet/"+device+"/portforward", nil); err != nil {
				v.logger.Error("port forwards list request failed", "error", err)
				return nil, err
			} else {
				tmp := map[string]interface{}{}

				if err = json.Unmarshal(r, &tmp); err != nil {
					v.logger.Warn("failed initial port forward parsing", "error", err)
					return nil, err
				} else if ifwds, ok := tmp["port_forwardings"].([]interface{}); !ok {
					v.logger.Warn("failed to convert port forwardings", "forwards", tmp["port_forwardings"])
					return nil, errors.New("failed to parse port forwards")
				} else {

					for _, i := range ifwds {
						fwd := i.(map[string]interface{})
						g := fwd["guest"].(map[string]interface{})

						pfwd := &vagrant_driver.PortFwd{
							Port:        int(fwd["port"].(float64)),
							Protocol:    fwd["protocol"].(string),
							Description: fwd["desc"].(string),
							SlotNumber:  slotNum,
							Guest: &vagrant_driver.PortFwdGuest{
								Ip:   g["ip"].(string),
								Port: int(g["port"].(float64)),
							},
						}

						fwds = append(fwds, pfwd)
					}
				}
			}
		}
	}

	for _, pfwd := range fwds {
		for _, natFwd := range v.Settings().NAT.PortFwds() {
			nfwd := v.utilityToDriverFwd(natFwd)

			if pfwd.Matches(nfwd) {
				v.logger.Trace("updating port forward description", "portforward", pfwd, "description", nfwd.Description)
				pfwd.Description = nfwd.Description
			}

		}

		f.PortForwards = append(f.PortForwards, pfwd)
	}

	v.logger.Trace("current port forwards list", "portforwards", f)

	return f, nil
}

func (v *VmrestDriver) AddPortFwd(pfwds []*vagrant_driver.PortFwd) error {
	var err error

	v.logger.Trace("adding port forwards", "portforwards", pfwds)

	for _, fwd := range pfwds {
		if fwd.Description, err = v.validatePortFwdDescription(fwd.Description); err != nil {
			return err
		}

		v.logger.Trace("creating port forward", "portforward", fwd)
		// Check if we have the internal port forward service enabled, and if so
		// add the port forward there. Otherwise, call up to the vmrest service

		if v.InternalPortForwarding() {
			if err = v.AddInternalPortForward(fwd); err != nil {
				return err
			}
		} else {
			f := map[string]interface{}{
				"guestIp":   fwd.Guest.Ip,
				"guestPort": fwd.Guest.Port,
				"desc":      VMREST_VAGRANT_DESC,
			}

			if body, e := json.Marshal(f); e != nil {
				v.logger.Error("failed to encode portforward request", "content", fwd.Guest, "error", e)

				return errors.New("failed to generate port forward request")
			} else {
				v.logger.Trace("new port forward request", "body", string(body))

				if _, err = v.Do("put", fmt.Sprintf("vmnet/vmnet%d/portforward/%s/%d", fwd.SlotNumber, fwd.Protocol, fwd.Port), bytes.NewBuffer(body)); err != nil {
					v.logger.Error("failed to create port forward", "portforward", fwd, "error", err)

					return err
				}
			}
		}

		v.logger.Info("port forward added", "portforward", fwd)

		ufwd := v.driverToUtilityFwd(fwd)

		// Ensure port forward is not already stored
		if err = v.Settings().NAT.Remove(ufwd); err != nil {
			v.logger.Trace("failure encountered attempting to remove port forward", "portforward", ufwd, "error", err)
		}

		if err = v.Settings().NAT.Add(ufwd); err != nil {
			v.logger.Trace("failed to store port forward in nat settings", "portforward", ufwd, "error", err)

			return errors.New("failed to persist port forward information")
		}

		if err = v.Settings().NAT.Save(); err != nil {
			v.logger.Error("failed to save port forward nat settings", "error", err)

			return errors.New("failed to store persistent port forward information")
		}
	}

	v.logger.Trace("all port forwards added", "portforwards", pfwds)

	return nil
}

func (v *VmrestDriver) DeletePortFwd(pfwds []*vagrant_driver.PortFwd) error {
	var err error

	v.logger.Trace("removing port forwards", "portforwards", pfwds)

	for _, fwd := range pfwds {
		v.logger.Trace("deleting port forward", "portforward", fwd)

		if v.InternalPortForwarding() {
			if err = v.DeleteInternalPortForward(fwd); err != nil {
				break
			}
		} else if _, err = v.Do("delete", fmt.Sprintf("vmnet/vmnet%d/portforward/%s/%d", fwd.SlotNumber, fwd.Protocol, fwd.Port), nil); err != nil {
			v.logger.Error("failed to delete port forward", "portforward", fwd, "error", err)

			return err
		} else {
			v.logger.Info("port forward removed", "portforward", fwd)

			ufwd := v.driverToUtilityFwd(fwd)

			if err = v.Settings().NAT.Remove(ufwd); err != nil {
				v.logger.Error("failed to remove port forward from nat settings", "portforward", ufwd, "error", err)

				err = errors.New("failed to persist port forward removal information")
				break
			} else if err = v.Settings().NAT.Save(); err != nil {
				v.logger.Error("failed to save port forward nat settings", "error", err)

				err = errors.New("failed to store persistent port forward information")
				break
			}
		}
	}

	v.logger.Trace("all port fowards removed", "portforwards", pfwds)

	return err
}

func (v *VmrestDriver) LookupDhcpAddress(device string, mac string) (string, error) {
	return v.fallback.LookupDhcpAddress(device, mac)
}

func (v *VmrestDriver) ReserveDhcpAddress(slot int, mac string, ip string) error {
	var err error
	var body []byte

	// Big Sur does not support dhcp address reservation
	if v.isBigSurMin {
		err = errors.New("DHCP reservations are not available on this platform")
	} else {

		v.logger.Trace("reserving dhcp address", "slot", slot, "mac", mac, "ip", ip)

		if body, err = json.Marshal(map[string]string{"IP": ip}); err != nil {
			v.logger.Error("failed to encode dhcp reservation request", "error", err)

			err = errors.New("failed to encode dhcp reservation request")
		} else if _, err = v.Do("put", fmt.Sprintf("vmnet/vmnet%d/mactoip/%s", slot, mac), bytes.NewBuffer(body)); err != nil {
			v.logger.Error("failed to create dhcp reservation", "error", err)

			err = errors.New("failed to create dhcp reservation")
		}
	}

	return err
}

// All of these we pass through to the fallback driver

func (v *VmrestDriver) LoadNetworkingFile() (utility.NetworkingFile, error) {
	return v.fallback.LoadNetworkingFile()
}

func (v *VmrestDriver) VerifyVmnet() error {
	return v.fallback.VerifyVmnet()
}

func (v *VmrestDriver) Request(method, path string, body io.Reader) (*http.Response, error) {
	v.logger.Info("starting remote request to vmware service")

	url := strings.Join([]string{v.vmrest.Active(), path}, "/")
	method = strings.ToUpper(method)

	if req, err := http.NewRequest(method, url, body); err != nil {
		return nil, err
	} else {
		req.SetBasicAuth(v.vmrest.Username(), v.vmrest.Password())
		req.Header.Add("Accept", VMREST_CONTENT_TYPE)
		req.Header.Add("User-Agent", v.vmrest.UserAgent())
		if body != nil {
			req.Header.Add("Content-Type", VMREST_CONTENT_TYPE)
		}

		v.logger.Debug("sending request", "method", method, "url", url)

		if resp, err := v.client.Do(req.WithContext(v.ctx)); err != nil {
			v.logger.Warn("request failed", "error", err)
			return nil, err
		} else {
			return resp, err
		}
	}
}

// Sends a request to the vmrest service
func (v *VmrestDriver) Do(method, path string, body io.Reader) ([]byte, error) {
	if resp, err := v.Request(method, path, body); err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()

		if r, err := io.ReadAll(resp.Body); err != nil {
			return nil, err
		} else {
			v.logger.Debug("received response", "code", resp.StatusCode, "status", resp.Status, "body", string(r), "error", err)

			if resp.StatusCode > 299 {
				result := map[string]interface{}{}
				if err = json.Unmarshal(r, &result); err != nil {
					return nil, errors.New("unknown error encountered with vmrest process")
				}

				if msg, ok := result["Message"].(string); !ok {
					return nil, errors.New("unknown error encountered with vmrest process")
				} else {
					return nil, errors.New("failure encountered: " + msg)
				}
			}

			return r, err
		}
	}
}

// Finds a free vmnet device. Currently very stupid and does not
// match on missing devices
func (v *VmrestDriver) setVmnetName(vnet *vagrant_driver.Vmnet) (err error) {
	vmns, err := v.Vmnets()
	names := []string{}
	for _, n := range vmns.Vmnets {
		names = append(names, n.Name)
	}
	slot := freeSlot(names, []string{VMWARE_NETDEV_PREFIX})
	vnet.Name = fmt.Sprintf("vmnet%d", slot)
	return
}

func freeSlot(list []string, prefixes []string) int {
	slots := []int{}

	for _, n := range list {
		for _, p := range prefixes {
			n = strings.TrimPrefix(n, p)
		}

		val, err := strconv.Atoi(n)

		if err != nil {
			continue
		}

		slots = append(slots, val)
	}

	sort.Ints(slots)

	for i := 1; i <= len(slots); i++ {
		if slots[i-1] != i {
			return i
		}
	}

	return len(slots) + 2
}

func (v *VmrestDriver) utilityToDriverFwd(f *utility.PortFwd) *vagrant_driver.PortFwd {
	slot, err := strconv.Atoi(string(f.Device[len(f.Device)-1]))

	if err != nil {
		slot = -1
	}

	return &vagrant_driver.PortFwd{
		Port:        f.HostPort,
		Protocol:    f.Protocol,
		Description: f.Description,
		SlotNumber:  slot,
		Guest: &vagrant_driver.PortFwdGuest{
			Ip:   f.GuestIp,
			Port: f.GuestPort,
		},
	}
}

func (v *VmrestDriver) driverToUtilityFwd(f *vagrant_driver.PortFwd) *utility.PortFwd {
	return &utility.PortFwd{
		HostPort:    f.Port,
		Protocol:    f.Protocol,
		Description: f.Description,
		GuestIp:     f.Guest.Ip,
		GuestPort:   f.Guest.Port,
		Device:      fmt.Sprintf("vmnet%d", f.SlotNumber)}
}

func (v *VmrestDriver) detectNAT(d Driver) (*vagrant_driver.Vmnet, error) {
	if devices, err := d.Vmnets(); err != nil {
		v.logger.Warn("failed to fetch vmnet list for nat detection", "error", err)
		return nil, err
	} else {
		var vnet *vagrant_driver.Vmnet

		for i := 0; i < len(devices.Vmnets); i++ {
			n := devices.Vmnets[i]

			v.logger.Trace("inspecting device for nat support", "vmnet", n)

			if n.Type == "nat" {
				v.logger.Debug("located nat device", "vmnet", n)
				vnet = n
				break
			}
		}

		if vnet == nil {
			err = errors.New("failed to locate NAT vmnet device")
		}

		return vnet, err
	}
}

// Check given path and determine if file system is case-(in)sensitive. If it
// is, return back the downcased version of the path. Otherwise, return the
// given path as valid if it exists.
func (v *VmrestDriver) matchVmPath(checkPath string) (string, error) {
	lowerPath := strings.ToLower(checkPath)
	checkStat, checkErr := os.Stat(checkPath)
	lowerStat, lowerErr := os.Stat(lowerPath)

	if checkErr == nil && lowerErr != nil {
		v.logger.Trace("exact vmx path match", "path", checkPath)

		return checkPath, nil
	} else if checkErr != nil && lowerErr == nil {
		v.logger.Trace("lower vmx path match only", "path", checkPath, "lower", lowerPath)

		return "", errors.New("failed to validate VMX path")
	} else if checkErr != nil && lowerErr != nil {
		return "", errors.New("failed to detect VMX path")
	} else if os.SameFile(checkStat, lowerStat) {
		v.logger.Trace("exact and lower valid match (case insensitive)", "path", checkPath, "lower", lowerPath, "used", lowerPath)

		return lowerPath, nil
	}

	v.logger.Trace("no vmx path match found", "path", checkPath)

	return "", errors.New("VMX path provided invalid")
}

// Validate the portforward description format and VMX path
func (v *VmrestDriver) validatePortFwdDescription(description string) (string, error) {
	if strings.HasPrefix(description, vagrant_driver.PORTFWD_PREFIX) {
		if match, err := v.matchVmPath(strings.Replace(description, vagrant_driver.PORTFWD_PREFIX, "", -1)); err != nil {
			return "", err
		} else {
			return vagrant_driver.PORTFWD_PREFIX + match, nil
		}
	}

	v.logger.Debug("port forward description prefix invalid", "description", description)

	return "", errors.New("invalid port forward description format")
}
