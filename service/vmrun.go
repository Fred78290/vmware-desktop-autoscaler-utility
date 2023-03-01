package service

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

type Vmrun interface {
	RunningVms() ([]*VirtualMachine, error)
	Create(request *CreateVirtualMachine) (*VirtualMachine, error)
	Delete(vmuuid string) (bool, error)
	PowerOn(vmuuid string) (bool, error)
	PowerOff(vmuuid string) (bool, error)
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
	exePath string
	logger  hclog.Logger
}

type VirtualMachine struct {
	Path   string
	Uuid   string
	Name   string
	Vcpus  int
	Memory int
	vmrun  Vmrun
}

func NewVmrun(path string, logger hclog.Logger) (Vmrun, error) {
	if !utility.RootOwned(path, true) {
		return nil, errors.New("failed to locate valid vmrun executable")
	}

	logger = logger.Named("vmrun")

	return &VmrunExe{
		exePath: path,
		logger:  logger,
	}, nil
}

func (v *VmrunExe) RunningVms() ([]*VirtualMachine, error) {
	result := []*VirtualMachine{}
	cmd := exec.Command(v.exePath, "list")
	exitCode, out := utility.ExecuteWithOutput(cmd)

	if exitCode != 0 {
		v.logger.Debug("vmrun list failed", "exitcode", exitCode)
		v.logger.Trace("vmrun list failed", "output", out)

		return result, errors.New("failed to list running VMs")
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		v.logger.Trace("vmrun path check", "path", line)

		if utility.FileExists(line) {
			v.logger.Trace("vmrun path valid", "path", line)
			result = append(result, &VirtualMachine{Path: line, vmrun: v})
		}
	}

	return result, nil
}

type EthernetCard struct {
	AddressType            string
	BsdName                string
	ConnectionType         string
	DisplayName            string
	GeneratedAddress       string
	GeneratedAddressOffset int
	LinkStatePropagation   bool
	PciSlotNumber          int
	Present                bool
	VirtualDev             string
	Vnet                   string
	Address                string
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

func (v *VmrunExe) Create(request *CreateVirtualMachine) (*VirtualMachine, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}

func (v *VmrunExe) Delete(vmuuid string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}

func (v *VmrunExe) PowerOn(vmuuid string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method PowerOn not implemented")
}

func (v *VmrunExe) PowerOff(vmuuid string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method PowerOff not implemented")
}

func (v *VmrunExe) ShutdownGuest(vmuuid string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method ShutdownGuest not implemented")
}

func (v *VmrunExe) Status(vmuuid string) (*VirtualMachineStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}

func (v *VmrunExe) WaitForIP(vmuuid string) (string, error) {
	return "", status.Errorf(codes.Unimplemented, "method WaitForIP not implemented")
}

func (v *VmrunExe) WaitForToolsRunning(vmuuid string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method WaitForToolsRunning not implemented")
}

func (v *VmrunExe) SetAutoStart(vmuuid string, autostart bool) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "method SetAutoStart not implemented")
}

func (v *VmrunExe) VirtualMachineByName(vmname string) (*VirtualMachine, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VirtualMachineByName not implemented")
}

func (v *VmrunExe) VirtualMachineByUUID(vmuuid string) (*VirtualMachine, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VirtualMachineByUUID not implemented")
}

func (v *VmrunExe) ListVirtualMachines() ([]*VirtualMachine, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListVirtualMachines not implemented")
}
