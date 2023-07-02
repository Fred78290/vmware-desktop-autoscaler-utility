package service

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fred78290/vmrest-go-client/client"
	"github.com/Fred78290/vmrest-go-client/client/model"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/status"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	"github.com/hashicorp/go-hclog"
	vagrant_utility "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
	codes "google.golang.org/grpc/codes"
)

const (
	vmrunlistfailed = "vmrun list failed"
	vmrunstopfailed = "vmrun stop failed"
	toolsnotrunning = "not running"
	failedtofindvm  = "failed to find VM: %s, reason: %v"
)

var pcislotnumber = []string{"160", "192", "161", "193", "225"}

type EthernetCard struct {
	AddressType          string `json:"addressType,omitempty"`
	BsdName              string `json:"bsdName,omitempty"`
	ConnectionType       string `json:"connectionType,omitempty"`
	DisplayName          string `json:"displayName,omitempty"`
	MacAddress           string `json:"macaddress,omitempty"`
	MacAddressOffset     int    `json:"macaddressOffset,omitempty"`
	LinkStatePropagation bool   `json:"linkStatePropagation,omitempty"`
	PciSlotNumber        int    `json:"pciSlotNumber,omitempty"`
	Present              bool   `json:"present"`
	VirtualDev           string `json:"virtualDev,omitempty"`
	Vnet                 string `json:"vnet,omitempty"`
	IP4Address           string `json:"ip4address,omitempty"`
}

type VirtualMachineStatus struct {
	Powered       bool            `json:"powered"`
	EthernetCards []*EthernetCard `json:"ethernet,omitempty"`
}

type NetworkInterface struct {
	MacAddress     string `json:"macaddress,omitempty"`
	Vnet           string `json:"vnet,omitempty"`
	ConnectionType string `json:"type,omitempty"`
	Device         string `json:"device,omitempty"`
	BsdName        string `json:"bsdName,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
}

type NetworkDevice struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Type   string `json:"type,omitempty" yaml:"type,omitempty"`
	Dhcp   bool   `json:"dhcp,omitempty" yaml:"dhcp,omitempty"`
	Subnet string `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	Mask   string `json:"mask,omitempty" yaml:"mask,omitempty"`
}

type CreateVirtualMachine struct {
	Template     string              `json:"template,omitempty"`
	Name         string              `json:"name,omitempty"`
	Vcpus        int                 `json:"vcpus,omitempty"`
	Memory       int                 `json:"memory,omitempty"`
	DiskSizeInMb int                 `json:"diskSizeInMB,omitempty"`
	Networks     []*NetworkInterface `json:"networks,omitempty"`
	GuestInfos   map[string]string   `json:"guestInfos,omitempty"`
	Linked       bool                `json:"linked,omitempty"`
	Register     bool                `json:"register,omitempty"`
}

type networkInfo struct {
	index int
	mac   string
	ip    []string
}

type Vmrun interface {
	SetApiClient(*client.APIClient)
	RunningVms() ([]*VirtualMachine, error)
	Create(request *CreateVirtualMachine) (*VirtualMachine, error)
	Delete(vmuuid string) (bool, error)
	PowerOn(vmuuid string) (bool, error)
	PowerOff(vmuuid, mode string) (bool, error)
	PowerState(vmuuid string) (bool, error)
	ShutdownGuest(vmuuid string) (bool, error)
	Status(vmuuid string) (*VirtualMachineStatus, error)
	WaitForIP(vmuuid string, timeout time.Duration) (string, error)
	WaitForToolsRunning(vmuuid string, timeout time.Duration) (bool, error)
	SetAutoStart(vmuuid string, autostart bool) (bool, error)
	VirtualMachineByName(vmname string) (*VirtualMachine, error)
	VirtualMachineByUUID(vmuuid string) (*VirtualMachine, error)
	ListVirtualMachines() ([]*VirtualMachine, error)
	ListNetworks() ([]*NetworkDevice, error)
	AddNetworkInterface(vmuuid, vnet string) error
	ChangeNetworkInterface(vmuuid, vnet string, nic int) error
	StartAutostartVM() error
}

type VmrunExe struct {
	sync.Mutex
	exeVdiskManager string
	exePath         string
	logger          hclog.Logger
	timeout         time.Duration
	vmfolder        string
	clonevm         bool
	cachebyuuid     map[string]*VirtualMachine
	cachebyvmx      map[string]*VirtualMachine
	cachebyname     map[string]*VirtualMachine
	client          *client.APIClient
}

type VirtualMachine struct {
	Path        string `json:"path,omitempty"`
	Uuid        string `json:"uuid,omitempty"`
	Name        string `json:"name,omitempty"`
	Vcpus       int    `json:"vcpus,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Powered     bool   `json:"powered"`
	Address     string `json:"ip4address,omitempty"`
	ToolsStatus string `json:"toolsStatus,omitempty"`
}

func NewVmrun(c *settings.CommonConfig, exePath, exeVdiskManager string, logger hclog.Logger) (Vmrun, error) {
	if !vagrant_utility.RootOwned(exePath, true) {
		return nil, errors.New("failed to locate valid vmrun executable")
	}

	if !vagrant_utility.RootOwned(exeVdiskManager, true) {
		return nil, errors.New("failed to locate valid vmware-vdiskmanager executable")
	}

	logger = logger.Named("vmrun")

	return &VmrunExe{
		exeVdiskManager: exeVdiskManager,
		exePath:         exePath,
		logger:          logger,
		timeout:         c.Timeout,
		vmfolder:        c.VMFolder,
		cachebyuuid:     make(map[string]*VirtualMachine),
		cachebyvmx:      make(map[string]*VirtualMachine),
		cachebyname:     make(map[string]*VirtualMachine),
	}, nil
}

func (v *VmrunExe) SetApiClient(client *client.APIClient) {
	v.client = client
}

func (v *VmrunExe) cacheVM(vm *VirtualMachine) {
	v.cachebyuuid[vm.Uuid] = vm
	v.cachebyvmx[vm.Path] = vm
	v.cachebyname[vm.Name] = vm
}

func (v *VmrunExe) deleteCachedVM(vm *VirtualMachine) {
	delete(v.cachebyuuid, vm.Uuid)
	delete(v.cachebyvmx, vm.Path)
	delete(v.cachebyname, vm.Name)
}

func (v *VmrunExe) stillExists(vm *VirtualMachine) bool {
	if utils.FileExists(vm.Path) {
		if _, err := v.client.GetVM(vm.Uuid); err == nil {
			return true
		}
	}

	return false
}

func (v *VmrunExe) cachedVM(foundVM *VirtualMachine) (*VirtualMachine, error) {
	var err error

	if v.stillExists(foundVM) {
		if foundVM.Powered, err = v.isRunningVm(foundVM); err != nil {
			return foundVM, status.Errorf(codes.Unavailable, "failed to get power status for VM: %s, reason: %v", foundVM.Path, err)
		} else if foundVM.Powered {
			v.vmwareToolsStatus(foundVM)
		} else {
			foundVM.ToolsStatus = toolsnotrunning
		}
	} else {
		v.deleteCachedVM(foundVM)

		foundVM = nil
	}

	return foundVM, err
}

func (v *VmrunExe) fetchIPAddress(vmuuid string) (ip *model.InlineResponse200, err error) {
	if ip, err = v.client.GetIPAddress(vmuuid); err != nil {
		if ge, ok := err.(client.GenericSwaggerError); ok {
			if me, ok := ge.Model().(model.ErrorModel); ok {
				if me.Code == 106 {
					err = nil
				}
			}
		}
	}

	return ip, err
}

func (v *VmrunExe) fetchVM(vmuuid, vmx string) (vm *VirtualMachine, err error) {
	var info *model.VmInformation
	var name *model.ConfigVmParamsParameter
	var ip *model.InlineResponse200

	vm = &VirtualMachine{
		Path: vmx,
		Uuid: vmuuid,
	}

	if info, err = v.client.GetVM(vmuuid); err == nil {

		if name, err = v.client.GetVMParams(vmuuid, "vmname"); err == nil {
			if ip, err = v.fetchIPAddress(vmuuid); err != nil {
				return
			} else if ip != nil {
				vm.Address = ip.Ip
			}

			vm.Name = name.Value
			vm.Vcpus = info.Cpu.Processors
			vm.Memory = info.Memory
			vm.Powered, err = v.isRunningVm(vm)

			if vm.Powered {
				v.vmwareToolsStatus(vm)
			} else {
				vm.ToolsStatus = toolsnotrunning
			}
		}
	}

	return
}

func (v *VmrunExe) registeredVM() error {
	v.Lock()
	defer v.Unlock()

	if vms, err := v.client.GetAllVMs(); err != nil {
		return err
	} else {
		cachebyuuid := make(map[string]*VirtualMachine)
		cachebyvmx := make(map[string]*VirtualMachine)
		cachebyname := make(map[string]*VirtualMachine)

		for _, vm := range vms {
			if registered, err := v.fetchVM(vm.Id, vm.Path); err != nil {
				return err
			} else {
				cachebyuuid[vm.Id] = registered
				cachebyvmx[vm.Path] = registered
				cachebyvmx[registered.Name] = registered
			}
		}

		v.cachebyuuid = cachebyuuid
		v.cachebyvmx = cachebyvmx
		v.cachebyname = cachebyname

		return nil
	}
}

func (v *VmrunExe) RunningVms() ([]*VirtualMachine, error) {

	result := []*VirtualMachine{}

	if err := v.registeredVM(); err != nil {
		return result, err
	} else {
		cmd := exec.Command(v.exePath, "list")
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug(vmrunlistfailed, "exitcode", exitCode)
			v.logger.Trace(vmrunlistfailed, "output", out)

			return result, status.Errorf(codes.Internal, "failed to list running VMs")
		}

		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)

			if vagrant_utility.FileExists(line) {
				if vm, found := v.cachebyvmx[line]; found {
					result = append(result, vm)
				}
			}
		}

		return result, nil
	}
}

func (v *VmrunExe) isRunningVm(vm *VirtualMachine) (bool, error) {

	cmd := exec.Command(v.exePath, "list")
	exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

	if exitCode != 0 {
		v.logger.Debug(vmrunlistfailed, "exitcode", exitCode)
		v.logger.Trace(vmrunlistfailed, "output", out)

		return false, status.Errorf(codes.Internal, "failed to list running VMs")
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == vm.Path {
			return true, nil
		}
	}

	v.logger.Trace("vm not running", "path", vm.Path)

	return false, nil
}

func (v *VmrunExe) createVmPath(name string) (string, error) {
	vmpath := utility.DirectoryForVirtualMachine(v.vmfolder, name)

	if _, err := os.Stat(vmpath); err == nil {
		return vmpath, status.Errorf(codes.AlreadyExists, "VMX already exists: %s", vmpath)
	}

	return vmpath, nil
}

func (v *VmrunExe) expandDisk(vmxpath string, diskSizeInMb int, vmx *utils.VMXMap) error {

	if diskSizeInMb == 0 {
		return nil
	}

	for _, disk := range []string{"nvme0:0", "scsi0:0", "sata0:0"} {
		key := fmt.Sprintf("%s.present", disk)

		if utils.StrToBool(vmx.Get(key)) {
			key = fmt.Sprintf("%s.filename", disk)

			if vmx.Has(key) {
				vmdk := path.Join(path.Dir(vmxpath), vmx.Get(key))

				if _, err := os.Stat(vmdk); err != nil {
					return status.Errorf(codes.AlreadyExists, "VMDK: %s not found", vmdk)
				}

				cmd := exec.Command(v.exeVdiskManager, "-x", fmt.Sprintf("%dMB", diskSizeInMb), vmdk)
				exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

				if exitCode != 0 && !strings.Contains(out, "One of the parameters supplied is invalid") {
					v.logger.Debug("vmware-vdiskmanager failed", "exitcode", exitCode)
					v.logger.Trace("vmware-vdiskmanager failed", "output", out)

					return status.Errorf(codes.Internal, "failed to expand VMDK: %s to %dM, reason: %s", vmdk, diskSizeInMb, out)
				}

				return nil
			}
		}
	}

	return fmt.Errorf("no disk found for vmx: %s", vmxpath)
}

func (v *VmrunExe) clone(template *VirtualMachine, name string) (newpath string, err error) {
	if newpath, err = v.createVmPath(name); err != nil {
		return newpath, err
	} else {
		cmd := exec.Command(v.exePath, "clone", template.Path, newpath, "full", fmt.Sprintf("-cloneName=%s", name))
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug("vmrun clone failed", "exitcode", exitCode)
			v.logger.Trace("vmrun clone failed", "output", out)

			return newpath, status.Errorf(codes.Internal, "failed to clone VM: %s to %s, reason: %s", template.Path, newpath, out)
		}

		return newpath, nil
	}
}

func (v *VmrunExe) prepareEthernet(vmx *utils.VMXMap, inf *NetworkInterface, card int) {
	darwin := vagrant_utility.IsBigSurMin()

	ethernet := fmt.Sprintf("ethernet%d.", card)

	vmx.Set(ethernet+"present", "TRUE")
	vmx.Set(ethernet+"virtualDev", inf.Device)
	vmx.Set(ethernet+"connectionType", inf.ConnectionType)
	vmx.Set(ethernet+"linkStatePropagation.enable", "TRUE")
	vmx.Set(ethernet+"pciSlotNumber", pcislotnumber[card])

	if darwin {
		if inf.BsdName != "" {
			vmx.Set(ethernet+"bsdName", inf.BsdName)
		}

		if inf.DisplayName != "" {
			vmx.Set(ethernet+"displayName", inf.DisplayName)
		}
	}

	if inf.ConnectionType == "custom" {
		if darwin {
			vmx.Set(ethernet+"vnet", inf.Vnet)
		} else {
			vmx.Set(ethernet+"vnet", "/dev/"+inf.Vnet)
		}
	}

	vmx.Delete(ethernet + "generatedAddress")
	vmx.Delete(ethernet + "generatedAddressOffset")

	if inf.MacAddress != "generated" {
		vmx.Set(ethernet+"addressType", "static")
		vmx.Set(ethernet+"address", inf.MacAddress)
	} else {
		vmx.Set(ethernet+"addressType", inf.MacAddress)
	}
}

func (v *VmrunExe) prepareNetworkInterface(request *CreateVirtualMachine, vmx *utils.VMXMap) {
	numCards := len(request.Networks)

	if len(pcislotnumber) < numCards {
		numCards = len(pcislotnumber)
	}

	for card := 0; card < numCards; card++ {
		v.prepareEthernet(vmx, request.Networks[card], card)
	}
}

func (v *VmrunExe) prepareVMX(request *CreateVirtualMachine, vmxpath string, vmx *utils.VMXMap) (string, error) {
	vmx.Cleanup(len(request.Networks) > 0)

	vmx.Set("vmname", request.Name)
	vmx.Set("numvcpus", strconv.Itoa(request.Vcpus))
	vmx.Set("memsize", strconv.Itoa(request.Memory))

	// Set new guest infos
	if request.GuestInfos != nil {
		for k, v := range request.GuestInfos {
			vmx.Set("guestinfo."+k, v)
		}
	}

	v.prepareNetworkInterface(request, vmx)

	if err := vmx.Save(vmxpath); err != nil {
		return "", err
	}

	if request.Register {
		if result, err := v.client.RegisterVM(&model.VmRegisterParameter{Name: request.Name, Path: vmxpath}); err != nil {
			v.logger.Debug("failed to register vm", "name", request.Name, "path", vmxpath, "error", err)
			return "", err
		} else {
			return result.Id, nil
		}
	} else if vm, err := v.VirtualMachineByUUID(request.Template); err != nil {
		return "", err
	} else {

		return vm.Uuid, nil
	}
}

func (v *VmrunExe) prepareVM(request *CreateVirtualMachine, vm *VirtualMachine) (err error) {
	var vmx *utils.VMXMap

	if vmx, err = utils.LoadVMX(vm.Path); err != nil {
		return status.Errorf(codes.FailedPrecondition, "failed to load VMX: %s, reason: %v", vm.Path, err)
	}

	vmx.Cleanup(len(request.Networks) > 0)

	vmx.Set("vmname", request.Name)
	vmx.Set("numvcpus", strconv.Itoa(request.Vcpus))
	vmx.Set("memsize", strconv.Itoa(request.Memory))

	// Set new guest infos
	if request.GuestInfos != nil {
		for k, v := range request.GuestInfos {
			vmx.Set("guestinfo."+k, v)
		}
	}

	v.prepareNetworkInterface(request, vmx)

	if err = vmx.Save(vm.Path); err != nil {
		return status.Errorf(codes.FailedPrecondition, "failed to save VMX: %s, reason: %v", vm.Path, err)
	}

	if err = v.expandDisk(vm.Path, request.DiskSizeInMb, vmx); err != nil {
		return status.Errorf(codes.FailedPrecondition, "failed to expand disk: %s, reason: %v", vm.Path, err)
	}

	if request.Register {
		if _, err = v.client.RegisterVM(&model.VmRegisterParameter{Name: request.Name, Path: vm.Path}); err != nil {
			v.logger.Debug("failed to register vm", "name", request.Name, "path", vm.Path, "error", err)
			return err
		}
	}

	return
}

func (v *VmrunExe) Create(request *CreateVirtualMachine) (*VirtualMachine, error) {
	v.Lock()
	defer v.Unlock()

	if v.clonevm {
		return v.createWithVMRun(request)
	} else {
		return v.createWithVMRest(request)
	}
}

func (v *VmrunExe) createVM(template *VirtualMachine, name string) (vm *VirtualMachine, err error) {
	var infos *model.VmInformation

	if infos, err = v.client.CreateVM(&model.VmCloneParameter{ParentId: template.Uuid, Name: name}); err != nil {
		return
	}

	return v.VirtualMachineByUUID(infos.Id)
}

func (v *VmrunExe) createWithVMRest(request *CreateVirtualMachine) (*VirtualMachine, error) {
	if _, err := v.VirtualMachineByName(request.Name); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "VM named: %s, already exists", request.Name)
	} else if template, err := v.VirtualMachineByUUID(request.Template); err != nil {
		return nil, status.Errorf(codes.NotFound, "Template: %s, not found", request.Template)
	} else if vm, err := v.createVM(template, request.Name); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to create VM: %s, reason: %v", template.Path, err)
	} else if err := v.prepareVM(request, vm); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to prepare VM: %s, reason: %v", template.Path, err)
	} else {
		return vm, nil
	}
}

func (v *VmrunExe) createWithVMRun(request *CreateVirtualMachine) (*VirtualMachine, error) {
	if _, err := v.VirtualMachineByName(request.Name); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "VM named: %s, already exists", request.Name)
	} else if template, err := v.VirtualMachineByUUID(request.Template); err != nil {
		return nil, status.Errorf(codes.NotFound, "Template: %s, not found", request.Template)
	} else if vmxpath, err := v.clone(template, request.Name); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to clone VM: %s, reason: %v", template.Path, err)
	} else if vmx, err := utils.LoadVMX(vmxpath); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to load VMX: %s, reason: %v", vmxpath, err)
	} else if vmuuid, err := v.prepareVMX(request, vmxpath, vmx); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to prepare VMX: %s, reason: %v", template.Path, err)
	} else if err = v.expandDisk(vmxpath, request.DiskSizeInMb, vmx); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to expand disk of VM: %s, reason: %v", template.Path, err)
	} else {
		return v.VirtualMachineByUUID(vmuuid)
	}
}

func (v *VmrunExe) deleteWithVMRun(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else if found.Powered {
		return false, status.Errorf(codes.FailedPrecondition, "failed to delete VM: %s, reason: powered", vmuuid)
	} else {
		v.deleteCachedVM(found)

		cmd := exec.Command(v.exePath, "deleteVM", found.Path)
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug("vmrun deleteVM failed", "exitcode", exitCode)
			v.logger.Trace("vmrun deleteVM failed", "output", out)

			return false, status.Errorf(codes.Internal, "failed to delete VM: %s, reason: %s", vmuuid, out)
		}
	}

	return true, nil
}

func (v *VmrunExe) deleteWithVMRest(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else if found.Powered {
		return false, status.Errorf(codes.FailedPrecondition, "failed to delete VM: %s, reason: powered", vmuuid)
	} else if err = v.client.DeleteVM(vmuuid); err != nil {
		return false, status.Errorf(codes.Internal, "failed to delete VM: %s, reason: %v", vmuuid, err)
	} else {
		v.deleteCachedVM(found)
	}

	return true, nil
}

func (v *VmrunExe) Delete(vmuuid string) (bool, error) {
	v.Lock()
	defer v.Unlock()

	if v.clonevm {
		return v.deleteWithVMRun(vmuuid)
	} else {
		return v.deleteWithVMRest(vmuuid)
	}
}

func (v *VmrunExe) PowerState(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else {
		return found.Powered, nil
	}
}

func (v *VmrunExe) powerOnVM(vm *VirtualMachine) error {
	cmd := exec.Command(v.exePath, "start", vm.Path, "nogui")
	exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

	if exitCode != 0 {
		v.logger.Debug("vmrun start failed", "exitcode", exitCode)
		v.logger.Trace("vmrun start failed", "output", out)

		return status.Errorf(codes.Internal, "failed to power on VM: %s, reason: %s", vm.Uuid, out)
	}

	vm.Powered = true

	return nil
}

func (v *VmrunExe) PowerOn(vmuuid string) (bool, error) {
	v.Lock()
	defer v.Unlock()

	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else if found.Powered {
		return true, nil
	} else if err = v.powerOnVM(found); err != nil {
		return false, err
	}

	return true, nil
}

func (v *VmrunExe) PowerOff(vmuuid, mode string) (bool, error) {
	v.Lock()
	defer v.Unlock()

	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else if !found.Powered {
		return true, nil
	} else {
		cmd := exec.Command(v.exePath, "stop", found.Path, mode)
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug(vmrunstopfailed, "exitcode", exitCode)
			v.logger.Trace(vmrunstopfailed, "output", out)

			return false, status.Errorf(codes.Internal, "failed to power off VM: %s, reason: %s", vmuuid, out)
		}

		found.Powered = true
	}

	return true, nil
}

func (v *VmrunExe) ShutdownGuest(vmuuid string) (bool, error) {
	v.Lock()
	defer v.Unlock()

	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, failedtofindvm, vmuuid, err)
	} else if !found.Powered {
		return true, nil
	} else {
		cmd := exec.Command(v.exePath, "stop", found.Path, "soft")
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug(vmrunstopfailed, "exitcode", exitCode)
			v.logger.Trace(vmrunstopfailed, "output", out)

			return false, status.Errorf(codes.Internal, "failed to shutdown VM: %s, reason: %s", vmuuid, out)
		}
	}

	return true, nil
}

func (v *VmrunExe) getNicAddress(macaddress string, stack []networkInfo) string {

	for _, nic := range stack {
		if nic.mac == macaddress {
			for _, address := range nic.ip {
				if ip, err := netip.ParseAddr(strings.Split(address, "/")[0]); err == nil {
					if ip.Is4() {
						return ip.String()
					}
				}
			}
		}
	}

	return ""
}

func (v *VmrunExe) getNicInfoPowered(vm *VirtualMachine) (infos []networkInfo, err error) {
	var nics *model.NicIpStackAll

	if nics, err = v.client.GetNicInfo(vm.Uuid); err != nil {
		if ge, ok := err.(client.GenericSwaggerError); ok {
			if me, ok := ge.Model().(model.ErrorModel); ok {
				if me.Code == 106 {
					err = nil
				}
			}
		}
	}

	if nics != nil {
		infos = make([]networkInfo, 0, len(nics.Nics))

		for index, nic := range nics.Nics {
			infos = append(infos, networkInfo{
				index: index,
				mac:   nic.Mac,
				ip:    nic.Ip,
			})
		}
	}

	return
}

func (v *VmrunExe) getNicInfoNotPowered(vm *VirtualMachine) (infos []networkInfo, err error) {
	var nics *model.NicDevices

	if nics, err = v.client.GetAllNICDevices(vm.Uuid); nics != nil {
		infos = make([]networkInfo, 0, len(nics.Nics))

		for _, nic := range nics.Nics {
			infos = append(infos, networkInfo{
				index: nic.Index,
				mac:   nic.MacAddress,
				ip:    nil,
			})
		}
	}

	return
}

func (v *VmrunExe) getNicInfo(vm *VirtualMachine) (infos []networkInfo, err error) {

	if vm.Powered {
		return v.getNicInfoPowered(vm)
	} else {
		return v.getNicInfoNotPowered(vm)
	}
}

func (v *VmrunExe) Status(vmuuid string) (*VirtualMachineStatus, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return nil, err
	} else if vmx, err := utils.LoadVMX(vm.Path); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "can't load vmx for %s", vm.Path)
	} else if nics, err := v.getNicInfo(vm); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "can't get nics for vm %s, reason: %v", vm.Path, err)
	} else {
		card := 0

		vmstatus := &VirtualMachineStatus{
			Powered:       vm.Powered,
			EthernetCards: make([]*EthernetCard, 0, 5),
		}

		for {
			key := fmt.Sprintf("ethernet%d.present", card)

			if vmx.Has(key) {
				var macaddress string

				present := vmx.Get(fmt.Sprintf("ethernet%d.present", card))
				addressType := vmx.Get(fmt.Sprintf("ethernet%d.addresstype", card))

				if addressType == "generated" {
					macaddress = vmx.Get(fmt.Sprintf("ethernet%d.generatedaddress", card))
				} else {
					macaddress = vmx.Get(fmt.Sprintf("ethernet%d.address", card))
				}

				ethernet := &EthernetCard{
					Present:              utils.StrToBool(present),
					IP4Address:           v.getNicAddress(macaddress, nics),
					AddressType:          addressType,
					BsdName:              vmx.Get(fmt.Sprintf("ethernet%d.bsdname", card)),
					ConnectionType:       vmx.Get(fmt.Sprintf("ethernet%d.connectiontype", card)),
					DisplayName:          vmx.Get(fmt.Sprintf("ethernet%d.displayname", card)),
					MacAddress:           macaddress,
					MacAddressOffset:     utils.StrToInt(vmx.Get(fmt.Sprintf("ethernet%d.generatedaddressoffset", card))),
					LinkStatePropagation: utils.StrToBool(vmx.Get(fmt.Sprintf("ethernet%d.linkstatepropagation.enable", card))),
					PciSlotNumber:        utils.StrToInt(vmx.Get(fmt.Sprintf("ethernet%d.pcislotnumber", card))),
					VirtualDev:           vmx.Get(fmt.Sprintf("ethernet%d.virtualdev", card)),
					Vnet:                 vmx.Get(fmt.Sprintf("ethernet%d.vnet", card)),
				}

				card++

				vmstatus.EthernetCards = append(vmstatus.EthernetCards, ethernet)
			} else {
				break
			}
		}

		return vmstatus, nil
	}
}

func (v *VmrunExe) WaitForIP(vmuuid string, timeout time.Duration) (string, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return "", err
	} else if !vm.Powered {
		return "", status.Errorf(codes.FailedPrecondition, "failed to wait for IP, VM: %s is not powered", vmuuid)
	} else {
		address := ""

		err = utils.PollImmediate(5*time.Second, timeout, func() (done bool, err error) {
			if ipaddress, _ := v.client.GetIPAddress(vmuuid); ipaddress != nil && len(ipaddress.Ip) > 0 {
				address = ipaddress.Ip
				return true, nil
			} else {
				cmd := exec.Command(v.exePath, "getGuestIPAddress", vm.Path)
				exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

				if exitCode != 0 {
					// Got it on linux
					if strings.HasPrefix(out, "Error: Unable to get the IP address") || strings.HasPrefix(out, "Error: Cannot open VM:") || strings.HasPrefix(out, "Error: The VMware Tools are not running in the virtual machine") {
						return false, nil
					}

					v.logger.Debug("vmrun getGuestIPAddress failed", "exitcode", exitCode)
					v.logger.Trace("vmrun getGuestIPAddress failed", "output", out)

					return false, status.Errorf(codes.Internal, "failed to get ip VM: %s, reason: %s", vmuuid, out)
				}

				address = strings.Trim(out, "\n")

				return true, nil
			}
		})

		return address, err
	}
}

func (v *VmrunExe) vmwareToolsStatus(vm *VirtualMachine) error {
	// If we get IP address, assume vmware tools is running
	if ipaddress, err := v.client.GetIPAddress(vm.Uuid); err == nil && len(ipaddress.Ip) > 0 {
		vm.ToolsStatus = "running"
	} else {
		cmd := exec.Command(v.exePath, "checkToolsState", vm.Path)
		_, out := vagrant_utility.ExecuteWithOutput(cmd)
		// ignore exit code
		//		if exitCode != 0 {
		//			v.logger.Debug("vmrun checkToolsState failed", "exitcode", exitCode)
		//			v.logger.Trace("vmrun checkToolsState failed", "output", out)
		//
		//			return status.Errorf(codes.Internal, "failed to wait for tools running for VM: %s, reason: %s", vm.Uuid, out)
		//		}

		if strings.HasPrefix(out, "running") {
			vm.ToolsStatus = "running"
		} else if strings.HasPrefix(out, "installed") {
			vm.ToolsStatus = "installed"
		} else {
			vm.ToolsStatus = strings.Trim(out, "\n")
		}
	}

	return nil
}

func (v *VmrunExe) WaitForToolsRunning(vmuuid string, timeout time.Duration) (bool, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, err
	} else if !vm.Powered {
		return false, status.Errorf(codes.FailedPrecondition, "failed to wait for IP, VM: %s is not powered", vmuuid)
	} else {
		result := false

		err = utils.PollImmediate(time.Second, timeout, func() (done bool, err error) {
			if err := v.vmwareToolsStatus(vm); err != nil {
				return false, err
			} else {
				if vm.ToolsStatus == "running" {
					result = true

					return true, nil
				} else if vm.ToolsStatus == "installed" {
					return false, nil
				}

			}

			return false, status.Errorf(codes.Internal, "failed to wait for tools running for VM: %s, reason: %s", vmuuid, vm.ToolsStatus)
		})

		return result, err
	}
}

func (v *VmrunExe) StartAutostartVM() error {
	if vms, err := v.ListVirtualMachines(); err != nil {
		return err
	} else {
		for _, vm := range vms {
			if param, _ := v.client.GetVMParams(vm.Uuid, "autostart"); !vm.Powered && utils.StrToBool(param.Value) {
				if err = v.powerOnVM(vm); err != nil {
					v.logger.Error(fmt.Sprintf("unable to autostart VM: %s, %s", vm.Uuid, vm.Name))
				} else {
					v.logger.Info(fmt.Sprintf("Started VM: %s, %s", vm.Uuid, vm.Name))
				}
			}
		}
	}

	return nil
}

func (v *VmrunExe) SetAutoStart(vmuuid string, autostart bool) (bool, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, err
	} else if vmx, err := utils.LoadVMX(vm.Path); err != nil {
		return false, err
	} else {
		vmx.Set("autostart", utils.BoolToStr(autostart))

		if err = vmx.Save(vm.Path); err != nil {
			return false, err
		}
	}

	return autostart, nil
}

func (v *VmrunExe) findVM(vmname string) (*VirtualMachine, error) {
	if vms, err := v.client.GetAllVMs(); err != nil {
		return nil, err
	} else {
		for _, vm := range vms {
			if name, err := v.client.GetVMParams(vm.Id, "vmname"); err == nil {
				if name.Value == vmname {
					if foundVM, err := v.fetchVM(vm.Id, vm.Path); err == nil {
						v.cachebyuuid[vm.Id] = foundVM
						v.cachebyvmx[vm.Path] = foundVM
						v.cachebyvmx[foundVM.Name] = foundVM

						return foundVM, nil
					}
				}
			}
		}
	}

	return nil, status.Errorf(codes.NotFound, "vm with name: %s not found", vmname)
}

func (v *VmrunExe) VirtualMachineByName(vmname string) (foundVM *VirtualMachine, err error) {
	var found bool

	if foundVM, found = v.cachebyname[vmname]; !found {
		return v.findVM(vmname)
	} else if foundVM, err = v.cachedVM(foundVM); err != nil {
		return nil, err
	} else if foundVM == nil {
		return nil, status.Errorf(codes.NotFound, "vm with name: %s not found", vmname)
	}

	return foundVM, nil
}

func (v *VmrunExe) fetchAndCacheVM(vmuuid string) (foundVM *VirtualMachine, err error) {
	if vms, err := v.client.GetAllVMs(); err == nil {
		for _, vm := range vms {
			if vm.Id == vmuuid {
				if foundVM, err = v.fetchVM(vmuuid, vm.Path); err != nil {
					return nil, status.Errorf(codes.Internal, "error to fetch vm: %s, reason: %v", vmuuid, err)
				} else {
					v.cacheVM(foundVM)

					return foundVM, nil
				}
			}
		}
	}

	return nil, status.Errorf(codes.NotFound, "vm not found:%s", vmuuid)
}

func (v *VmrunExe) VirtualMachineByUUID(vmuuid string) (foundVM *VirtualMachine, err error) {
	var found bool

	if foundVM, found = v.cachebyuuid[vmuuid]; !found {

		if foundVM, err = v.fetchAndCacheVM(vmuuid); err != nil {
			return nil, err
		}

	} else if foundVM, err = v.cachedVM(foundVM); err != nil {
		return nil, err
	} else if foundVM == nil {
		return nil, status.Errorf(codes.NotFound, "vm with uuid: %s not found", vmuuid)
	}

	return foundVM, nil
}

func (v *VmrunExe) ListVirtualMachines() ([]*VirtualMachine, error) {
	if err := v.registeredVM(); err != nil {
		return nil, err
	} else {
		values := make([]*VirtualMachine, 0, len(v.cachebyuuid))

		for _, value := range v.cachebyuuid {
			values = append(values, value)
		}

		return values, nil
	}
}

func (v *VmrunExe) getNetworkInfos(vnet string) (*model.Network, error) {
	if networks, err := v.client.GetAllNetworks(); err != nil {
		return nil, err
	} else {
		for _, network := range networks.Vmnets {
			if network.Name == vnet {
				return &network, nil
			}
		}
	}

	return nil, fmt.Errorf("vmnet: %s, not found", vnet)
}

func (v *VmrunExe) setCustomInterface(vm *VirtualMachine, vmnet string, inetIndex int) (*utils.VMXMap, error) {
	if vmx, err := utils.LoadVMX(vm.Path); err != nil {
		return nil, err
	} else {
		inf := &NetworkInterface{
			MacAddress:     "generated",
			Vnet:           vmnet,
			ConnectionType: "custom",
			Device:         "vmxnet3",
		}

		v.prepareEthernet(vmx, inf, inetIndex)

		return vmx, nil
	}
}

func (v *VmrunExe) AddNetworkInterface(vmuuid, vmnet string) error {

	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return err
	} else if network, err := v.getNetworkInfos(vmnet); err != nil {
		return err
	} else if nics, err := v.client.GetAllNICDevices(vmuuid); err != nil {
		return err
	} else {
		var vmx *utils.VMXMap

		inetIndex := len(nics.Nics)
		if network.Type == "bridged" && vmnet != "vmnet0" {
			network.Type = "custom"
		}

		if network.Type == "custom" {
			if vmx, err = v.setCustomInterface(found, vmnet, inetIndex); err != nil {
				return err
			}
		} else if nic, err := v.client.CreateNICDevice(vmuuid, &model.NicDeviceParameter{Type: network.Type}); err != nil {
			return err
		} else if vmx, err = utils.LoadVMX(found.Path); err != nil {
			return err
		} else {
			vmx.Set(fmt.Sprintf("ethernet%d.virtualDev", nic.Index-1), "vmxnet3")
		}

		return vmx.Save(found.Path)
	}
}

func (v *VmrunExe) ChangeNetworkInterface(vmuuid, vmnet string, nic int) error {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return err
	} else if network, err := v.getNetworkInfos(vmnet); err != nil {
		return err
	} else {
		var vmx *utils.VMXMap

		inetIndex := nic - 1
		if network.Type == "bridged" && vmnet != "vmnet0" {
			network.Type = "custom"
		}

		if network.Type == "custom" {
			if vmx, err = v.setCustomInterface(found, vmnet, inetIndex); err != nil {
				return err
			}
		} else if _, err := v.client.UpdateNICDevice(vmuuid, nic, &model.NicDeviceParameter{Type: network.Type}); err != nil {
			return err
		} else if vmx, err = utils.LoadVMX(found.Path); err != nil {
			return err
		} else {
			vmx.Set(fmt.Sprintf("ethernet%d.virtualDev", inetIndex), "vmxnet3")
		}

		return vmx.Save(found.Path)
	}
}

func (v *VmrunExe) ListNetworks() ([]*NetworkDevice, error) {
	if networks, err := v.client.GetAllNetworks(); err != nil {
		return nil, err
	} else {
		result := make([]*NetworkDevice, 0, len(networks.Vmnets))

		for _, network := range networks.Vmnets {
			result = append(result, &NetworkDevice{
				Name:   network.Name,
				Type:   network.Type,
				Dhcp:   utils.StrToBool(network.Dhcp),
				Subnet: network.Subnet,
				Mask:   network.Mask,
			})
		}

		return result, nil
	}
}
