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
	"time"

	"github.com/Fred78290/vmrest-go-client/client"
	"github.com/Fred78290/vmrest-go-client/client/model"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	"github.com/hashicorp/go-hclog"
	vagrant_utility "github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

var pcislotnumber = []string{"160", "192", "161", "193", "225"}

type EthernetCard struct {
	AddressType          string
	BsdName              string
	ConnectionType       string
	DisplayName          string
	MacAddress           string
	MacAddressOffset     int
	LinkStatePropagation bool
	PciSlotNumber        int
	Present              bool
	VirtualDev           string
	Vnet                 string
	IP4Address           string
}

type VirtualMachineStatus struct {
	Powered       bool
	EthernetCards []*EthernetCard
}

type NetworkInterface struct {
	MacAddress     string
	Vnet           string
	ConnectionType string
	Device         string
	BsdName        string
	DisplayName    string
}

type CreateVirtualMachine struct {
	Template     string
	Name         string
	Vcpus        int
	Memory       int
	DiskSizeInMb int
	Networks     []*NetworkInterface
	GuestInfos   map[string]string
	Linked       bool
}

type Vmrun interface {
	SetApiClient(*client.APIClient)
	RunningVms() ([]*VirtualMachine, error)
	Create(request *CreateVirtualMachine) (*VirtualMachine, error)
	Delete(vmuuid string) (bool, error)
	PowerOn(vmuuid string) (bool, error)
	PowerOff(vmuuid string) (bool, error)
	PowerState(vmuuid string) (bool, error)
	ShutdownGuest(vmuuid string) (bool, error)
	Status(vmuuid string) (*VirtualMachineStatus, error)
	WaitForIP(vmuuid string) (string, error)
	WaitForToolsRunning(vmuuid string) (bool, error)
	SetAutoStart(vmuuid string, autostart bool) (bool, error)
	VirtualMachineByName(vmname string) (*VirtualMachine, error)
	VirtualMachineByUUID(vmuuid string) (*VirtualMachine, error)
	ListVirtualMachines() ([]*VirtualMachine, error)
}

type VmrunExe struct {
	exeVdiskManager string
	exePath         string
	logger          hclog.Logger
	timeout         time.Duration
	vmfolder        string
	cachebyuuid     map[string]*VirtualMachine
	cachebyvmx      map[string]*VirtualMachine
	cachebyname     map[string]*VirtualMachine
	client          *client.APIClient
}

type VirtualMachine struct {
	Path    string
	Uuid    string
	Name    string
	Vcpus   int
	Memory  int
	Powered bool
	Address string
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
	if _, err := v.client.GetVM(vm.Uuid); err == nil {
		return true
	}

	return false
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
		}
	}

	return
}

func (v *VmrunExe) registeredVM() error {
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
			v.logger.Debug("vmrun list failed", "exitcode", exitCode)
			v.logger.Trace("vmrun list failed", "output", out)

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
		v.logger.Debug("vmrun list failed", "exitcode", exitCode)
		v.logger.Trace("vmrun list failed", "output", out)

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

				if exitCode != 0 {
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

func (v *VmrunExe) prepareVMX(request *CreateVirtualMachine, vmxpath string, vmx *utils.VMXMap) (string, error) {
	vmx.Cleanup()

	vmx.Set("vmname", request.Name)
	vmx.Set("numvcpus", strconv.Itoa(request.Vcpus))
	vmx.Set("memsize", strconv.Itoa(request.Memory))

	// Set new guest infos
	if request.GuestInfos != nil {
		for k, v := range request.GuestInfos {
			vmx.Set("guestinfo."+k, v)
		}
	}

	numCards := len(request.Networks)

	if len(pcislotnumber) < numCards {
		numCards = len(pcislotnumber)
	}

	for card := 0; card < numCards; card++ {
		inf := request.Networks[card]
		ethernet := fmt.Sprintf("ethernet%d.", card)

		vmx.Set(ethernet+"present", "TRUE")
		vmx.Set(ethernet+"virtualDev", inf.Device)
		vmx.Set(ethernet+"connectionType", inf.ConnectionType)
		vmx.Set(ethernet+"linkStatePropagation.enable", "TRUE")
		vmx.Set(ethernet+"pciSlotNumber", pcislotnumber[card])

		if inf.BsdName != "" {
			vmx.Set(ethernet+"bsdName", inf.BsdName)
		}

		if inf.DisplayName != "" {
			vmx.Set(ethernet+"displayName", inf.DisplayName)
		}

		if inf.ConnectionType == "custom" {
			vmx.Set(ethernet+"vnet", inf.Vnet)
		}

		if inf.MacAddress != "generated" {
			vmx.Set(ethernet+"addressType", "static")
			vmx.Set(ethernet+"address", inf.MacAddress)
		} else {
			vmx.Set(ethernet+"addressType", inf.MacAddress)
		}
	}

	if err := vmx.Save(vmxpath); err != nil {
		return "", err
	}

	if result, err := v.client.RegisterVM(&model.VmRegisterParameter{Name: request.Name, Path: vmxpath}); err != nil {
		v.logger.Debug("failed to register vm", "name", request.Name, "path", vmxpath, "error", err)
		return "", err
	} else {
		return result.Id, nil
	}
}

func (v *VmrunExe) Create(request *CreateVirtualMachine) (*VirtualMachine, error) {
	if _, err := v.VirtualMachineByName(request.Name); err == nil {
		return nil, status.Errorf(codes.AlreadyExists, "VM named: %s, already exists", request.Name)
	} else if template, err := v.VirtualMachineByUUID(request.Template); err != nil {
		return nil, status.Errorf(codes.NotFound, "Template: %s, not found", request.Template)
	} else if vmxpath, err := v.clone(template, request.Name); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to clone VM: %s, reason: %v", template.Path, err)
	} else if vmx, err := utils.LoadVMX(vmxpath); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to load VMX: %s, reason: %v", vmxpath, err)
	} else if vmuuid, err := v.prepareVMX(request, vmxpath, vmx); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to prepare VM: %s, reason: %v", template.Path, err)
	} else if err = v.expandDisk(vmxpath, request.DiskSizeInMb, vmx); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to prepare VM: %s, reason: %v", template.Path, err)
	} else {
		return v.VirtualMachineByUUID(vmuuid)
	}
}

func (v *VmrunExe) Delete(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, "failed to find VM: %s, reason: %v", vmuuid, err)
	} else if found.Powered {
		return false, status.Errorf(codes.FailedPrecondition, "failed to delete VM: %s, reason: powered", vmuuid)
	} else {
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

func (v *VmrunExe) PowerState(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, "failed to find VM: %s, reason: %v", vmuuid, err)
	} else {
		return found.Powered, nil
	}
}

func (v *VmrunExe) PowerOn(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, "failed to find VM: %s, reason: %v", vmuuid, err)
	} else if found.Powered {
		return true, nil
	} else {
		cmd := exec.Command(v.exePath, "start", found.Path, "nogui")
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug("vmrun start failed", "exitcode", exitCode)
			v.logger.Trace("vmrun start failed", "output", out)

			return false, status.Errorf(codes.Internal, "failed to power on VM: %s, reason: %s", vmuuid, out)
		}

		found.Powered = true
	}

	return true, nil
}

func (v *VmrunExe) PowerOff(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, "failed to find VM: %s, reason: %v", vmuuid, err)
	} else if !found.Powered {
		return true, nil
	} else {
		cmd := exec.Command(v.exePath, "stop", found.Path, "hard")
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug("vmrun stop failed", "exitcode", exitCode)
			v.logger.Trace("vmrun stop failed", "output", out)

			return false, status.Errorf(codes.Internal, "failed to power off VM: %s, reason: %s", vmuuid, out)
		}

		found.Powered = true
	}

	return true, nil
}

func (v *VmrunExe) ShutdownGuest(vmuuid string) (bool, error) {
	if found, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, status.Errorf(codes.NotFound, "failed to find VM: %s, reason: %v", vmuuid, err)
	} else if !found.Powered {
		return true, nil
	} else {
		cmd := exec.Command(v.exePath, "stop", found.Path, "soft")
		exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

		if exitCode != 0 {
			v.logger.Debug("vmrun stop failed", "exitcode", exitCode)
			v.logger.Trace("vmrun stop failed", "output", out)

			return false, status.Errorf(codes.Internal, "failed to shutdown VM: %s, reason: %s", vmuuid, out)
		}
	}

	return true, nil
}

func (v *VmrunExe) getNicAddress(macaddress string, stack *model.NicIpStackAll) string {

	if stack != nil && stack.Nics != nil {
		for _, nic := range stack.Nics {
			if nic.Mac == macaddress {
				for _, address := range nic.Ip {
					if ip, err := netip.ParseAddr(strings.Split(address, "/")[0]); err == nil {
						if ip.Is4() {
							return ip.String()
						}
					}
				}
			}
		}
	}

	return ""
}

func (v *VmrunExe) GetNicInfo(vmuuid string) (nics *model.NicIpStackAll, err error) {

	if nics, err = v.client.GetNicInfo(vmuuid); err != nil {
		if ge, ok := err.(client.GenericSwaggerError); ok {
			if me, ok := ge.Model().(model.ErrorModel); ok {
				if me.Code == 106 {
					return nics, nil
				}
			}
		}
	}

	return
}

func (v *VmrunExe) Status(vmuuid string) (*VirtualMachineStatus, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return nil, err
	} else if vmx, err := utils.LoadVMX(vm.Path); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "can't load vmx for %s", vm.Path)
	} else if nics, err := v.GetNicInfo(vmuuid); err != nil {
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

func (v *VmrunExe) WaitForIP(vmuuid string) (string, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return "", err
	} else if !vm.Powered {
		return "", status.Errorf(codes.FailedPrecondition, "failed to wait for IP, VM: %s is not powered", vmuuid)
	} else {
		address := ""

		err = utils.PollImmediate(time.Second, v.timeout, func() (done bool, err error) {
			cmd := exec.Command(v.exePath, "getGuestIPAddress", vm.Path)
			exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

			if exitCode != 0 {
				v.logger.Debug("vmrun getGuestIPAddress failed", "exitcode", exitCode)
				v.logger.Trace("vmrun getGuestIPAddress failed", "output", out)

				return false, status.Errorf(codes.Internal, "failed to get ip VM: %s, reason: %s", vmuuid, out)
			}

			address = out

			return true, nil
		})

		return address, err
	}
}

func (v *VmrunExe) WaitForToolsRunning(vmuuid string) (bool, error) {
	if vm, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, err
	} else if !vm.Powered {
		return false, status.Errorf(codes.FailedPrecondition, "failed to wait for IP, VM: %s is not powered", vmuuid)
	} else {
		result := false

		err = utils.PollImmediate(time.Second, v.timeout, func() (done bool, err error) {
			cmd := exec.Command(v.exePath, "checkToolsState", vm.Path)
			exitCode, out := vagrant_utility.ExecuteWithOutput(cmd)

			if exitCode != 0 {
				v.logger.Debug("vmrun checkToolsState failed", "exitcode", exitCode)
				v.logger.Trace("vmrun checkToolsState failed", "output", out)

				return false, status.Errorf(codes.Internal, "failed to wait for tools running for VM: %s, reason: %s", vmuuid, out)
			}

			if strings.HasPrefix(out, "running") {
				result = true

				return true, nil
			} else if strings.HasPrefix(out, "installed") {
				return false, nil
			}

			return false, status.Errorf(codes.Internal, "failed to wait for tools running for VM: %s, reason: %s", vmuuid, out)
		})

		return result, err
	}
}

func (v *VmrunExe) SetAutoStart(vmuuid string, autostart bool) (bool, error) {
	if _, err := v.VirtualMachineByUUID(vmuuid); err != nil {
		return false, err
		//	} else if vm.Powered {
		//		return false, status.Errorf(codes.FailedPrecondition, "failed to set autostart for VM: %s is powered", vmuuid)
		//	} else {
		//		return false, status.Errorf(codes.Unimplemented, "method SetAutoStart not yet defined for %s", vm.Path)
	}

	return autostart, nil
}

func (v *VmrunExe) VirtualMachineByName(vmname string) (foundVM *VirtualMachine, err error) {
	var found bool

	if foundVM, found = v.cachebyname[vmname]; !found {

		if vms, err := v.client.GetAllVMs(); err == nil {
			for _, vm := range vms {
				if name, err := v.client.GetVMParams(vm.Id, "vmname"); err == nil {
					if name.Value == vmname {
						if foundVM, err = v.fetchVM(vm.Id, vm.Path); err == nil {
							v.cachebyuuid[vm.Id] = foundVM
							v.cachebyvmx[vm.Path] = foundVM
							v.cachebyvmx[foundVM.Name] = foundVM

							break
						}
					}
				}
			}
		}
	} else {
		if foundVM.Powered, err = v.isRunningVm(foundVM); err != nil {
			return foundVM, status.Errorf(codes.Unavailable, "failed to get power status for VM: %s, reason: %v", foundVM.Path, err)
		}
	}

	if foundVM == nil {
		return nil, status.Errorf(codes.NotFound, "vm with name: %s not found", vmname)
	}

	return foundVM, nil
}

func (v *VmrunExe) VirtualMachineByUUID(vmuuid string) (foundVM *VirtualMachine, err error) {
	var found bool

	if foundVM, found = v.cachebyuuid[vmuuid]; !found {

		if vms, err := v.client.GetAllVMs(); err == nil {
			for _, vm := range vms {
				if vm.Id == vmuuid {
					if foundVM, err = v.fetchVM(vmuuid, vm.Path); err != nil {
						return nil, status.Errorf(codes.Internal, "error to fetch vm: %s, reason: %v", vmuuid, err)
					} else {
						v.cacheVM(foundVM)
						break
					}
				}
			}
		}

	} else if v.stillExists(foundVM) {
		if foundVM.Powered, err = v.isRunningVm(foundVM); err != nil {
			return foundVM, status.Errorf(codes.Unavailable, "failed to get power status for VM: %s, reason: %v", vmuuid, err)
		}
	} else {
		v.deleteCachedVM(foundVM)

		foundVM = nil
	}

	if foundVM == nil {
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
