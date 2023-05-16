package service_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Fred78290/kubernetes-desktop-autoscaler/desktop"
	apiclient "github.com/Fred78290/vmrest-go-client/client"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/settings"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utils"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vagrant-vmware-desktop/go_src/vagrant-vmware-utility/utility"
)

type ConfigTest struct {
	Name       string          `json:"name"`
	Vcpus      int             `json:"vcpus"`
	MemorySize int             `json:"memory"`
	DiskSize   int             `json:"disk-size"`
	Hostname   string          `json:"hostname"`
	Username   string          `json:"username"`
	Password   string          `json:"password"`
	Template   string          `json:"template"`
	AuthKey    string          `json:"ssh-key"`
	NodeIndex  int             `json:"node-index"`
	CloudInit  interface{}     `json:"cloud-init"`
	Network    desktop.Network `json:"network"`
}

func (c *ConfigTest) buildCloudInit() (desktop.GuestInfos, error) {
	return desktop.BuildCloudInit(c.Hostname, c.Username, c.AuthKey, c.CloudInit, &c.Network, c.NodeIndex, false)
}

func (c *ConfigTest) buildNetworkInterface() []*service.NetworkInterface {
	inf := desktop.BuildNetworkInterface(c.Network.Interfaces, c.NodeIndex)
	native := make([]*service.NetworkInterface, 0, len(inf))

	for _, net := range inf {
		native = append(native, &service.NetworkInterface{
			MacAddress:     net.Macaddress,
			Vnet:           net.Vnet,
			ConnectionType: net.Type,
			Device:         net.Device,
			BsdName:        net.BsdName,
			DisplayName:    net.DisplayName,
		})
	}

	return native
}

func getConfFile() string {
	if config := os.Getenv("TEST_CONFIG"); config != "" {
		return config
	}

	return "../test/config.json"
}

func loadConfig() (*ConfigTest, error) {
	var config ConfigTest

	if configStr, err := os.ReadFile(getConfFile()); err != nil {
		return nil, err
	} else {
		err = json.Unmarshal(configStr, &config)
		return &config, err
	}
}

func parseURL(urlStr string) (*url.URL, error) {
	if len(urlStr) == 0 {
		return nil, fmt.Errorf("empty url")
	}

	return url.Parse(urlStr)
}

func waitForPowerState(vmrun service.Vmrun, vmuuid string, wanted bool) error {
	return utils.PollImmediate(time.Second, 0, func() (bool, error) {
		if powered, err := vmrun.PowerState(vmuuid); err != nil {
			return false, err
		} else {
			return powered == wanted, nil
		}
	})
}

func TestCreateVM(t *testing.T) {

	if config, err := loadConfig(); err != nil {
		t.Errorf("unable to load config: %v", err)
	} else if u, err := parseURL(os.Getenv("VMREST_URL")); err != nil {
		t.Errorf("unable to parse VMREST_URL: %v", err)
	} else {

		logger := hclog.New(&hclog.LoggerOptions{
			Name:   "test",
			Output: os.Stdout,
			Level:  hclog.LevelFromString("debug"),
		})

		c := &settings.CommonConfig{
			VMRestURL: u.String(),
			VMFolder:  os.Getenv("VMFOLDER"),
		}

		configuration := &apiclient.Configuration{
			Endpoint:    u.String(),
			UserAgent:   "Test",
			UserName:    os.Getenv("VMREST_USERNAME"),
			Password:    os.Getenv("VMREST_PASSWORD"),
			Timeout:     c.Timeout / time.Second,
			UnsecureTLS: true,
		}

		if client, err := apiclient.NewAPIClient(configuration); err != nil {
			t.Errorf("vmrest api client failed: %v", err)
		} else if paths, err := utility.LoadVmwarePaths(logger); err != nil {
			t.Errorf("failed to load vmware path: %v", err)
		} else if vmrun, err := service.NewVmrun(c, paths.Vmrun, paths.Vdiskmanager, logger); err != nil {
			t.Errorf("failed to create vmrun: %v", err)
		} else if guestInfos, err := config.buildCloudInit(); err != nil {
			t.Errorf("failed to create guestInfos: %v", err)
		} else {
			failOnError := func(vm *service.VirtualMachine, message string, err error) {
				t.Errorf(message, err)

				if vm != nil {
					vmrun.PowerOff(vm.Uuid)
					vmrun.Delete(vm.Uuid)
				}
			}

			vmrun.SetApiClient(client)

			request := service.CreateVirtualMachine{
				Template:     config.Template,
				Name:         config.Name,
				Vcpus:        config.Vcpus,
				Memory:       config.MemorySize,
				DiskSizeInMb: config.DiskSize,
				GuestInfos:   guestInfos,
				Networks:     config.buildNetworkInterface(),
			}

			if vm, err := vmrun.Create(&request); err != nil {
				failOnError(vm, "failed to create vm: %v", err)
			} else if _, err := vmrun.Status(vm.Uuid); err != nil {
				failOnError(vm, "failed to get status vm: %v", err)
			} else if _, err := vmrun.PowerOn(vm.Uuid); err != nil {
				failOnError(vm, "failed to poweron vm: %v", err)
			} else if err = waitForPowerState(vmrun, vm.Uuid, true); err != nil {
				failOnError(vm, "failed to wait poweroff vm: %v", err)
			} else if _, err := vmrun.WaitForToolsRunning(vm.Uuid); err != nil {
				failOnError(vm, "failed to wait tools vm: %v", err)
			} else if _, err := vmrun.WaitForIP(vm.Uuid); err != nil {
				failOnError(vm, "failed to wait ip vm: %v", err)
			} else if _, err := vmrun.Status(vm.Uuid); err != nil {
				failOnError(vm, "failed to get status vm: %v", err)
			} else if _, err := vmrun.PowerOff(vm.Uuid); err != nil {
				failOnError(vm, "failed to poweroff vm: %v", err)
			} else if err = waitForPowerState(vmrun, vm.Uuid, false); err != nil {
				failOnError(vm, "failed to wait poweroff vm: %v", err)
			} else if _, err := vmrun.Delete(vm.Uuid); err != nil {
				failOnError(vm, "failed to delete vm: %v", err)
			}
		}
	}
}
