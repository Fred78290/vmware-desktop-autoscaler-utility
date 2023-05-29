package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"

	"github.com/Fred78290/kubernetes-desktop-autoscaler/api"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/driver"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/service"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/status"
	"github.com/Fred78290/vmware-desktop-autoscaler-utility/utility"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type Grpc struct {
	api.UnimplementedVMWareDesktopAutoscalerServiceServer

	server     *grpc.Server
	vmrun      service.Vmrun
	stopChan   chan bool
	reqTracker sync.WaitGroup
	actionSync sync.Mutex
	inflight   int
	Halted     bool
	transport  string
	address    string
	HaltedChan chan bool
	logger     hclog.Logger
	logDisplay bool
	UI         cli.Ui
	Driver     driver.Driver
}

func CreateGrpc(bindAddr string, driver driver.Driver, logDisplay bool, ui cli.Ui, logger hclog.Logger) (*Grpc, error) {
	if u, err := url.Parse(bindAddr); err != nil {
		return nil, err
	} else {
		var listen string

		logger = logger.Named("grpc")

		if u.Scheme == "unix" {
			listen = u.Path
		} else {
			listen = u.Host
		}

		srv := &Grpc{
			transport:  u.Scheme,
			address:    listen,
			inflight:   0,
			vmrun:      driver.GetVmrun(),
			Driver:     driver,
			Halted:     true,
			HaltedChan: make(chan bool),
			stopChan:   make(chan bool),
			logger:     logger,
			logDisplay: logDisplay,
			UI:         ui,
		}

		return srv, nil
	}
}

func (g *Grpc) Debugf(format string, args ...interface{}) {
	format = fmt.Sprintf(format, args...)

	g.logger.Debug(fmt.Sprintf(format, args...))
}

func (g *Grpc) Warnf(format string, args ...interface{}) {
	format = fmt.Sprintf(format, args...)

	if g.logDisplay {
		g.logger.Warn(fmt.Sprintf(format, args...))
	} else {
		g.UI.Warn(format)
	}
}

func (g *Grpc) Infof(format string, args ...interface{}) {
	format = fmt.Sprintf(format, args...)

	if g.logDisplay {
		g.logger.Info(fmt.Sprintf(format, args...))
	} else {
		g.UI.Info(format)
	}
}

func (g *Grpc) Errorf(format string, args ...interface{}) {
	format = fmt.Sprintf(format, args...)

	if g.logDisplay {
		g.logger.Error(fmt.Sprintf(format, args...))
	} else {
		g.UI.Error(format)
	}
}

func (g *Grpc) Start() error {
	g.Debugf("start grpc service requested")
	g.actionSync.Lock()

	defer g.actionSync.Unlock()

	g.Infof("gRPC service start transport: %s, listen: %s", g.transport, g.address)

	server, err := g.createServer()

	if err != nil {
		return err
	}

	g.server = server
	g.Halted = false

	go g.consume()

	g.Debugf("api ready for message consumption")

	return nil
}

func (g *Grpc) Stop() error {
	g.Debugf("stop grpc service requested")
	g.actionSync.Lock()
	defer g.actionSync.Unlock()

	if g.Halted {
		return errors.New("server process is currently halted")
	}

	g.Debugf("sending stop notification to consumer")

	g.stopChan <- true

	return nil
}

func (g *Grpc) createServer() (*grpc.Server, error) {
	var server *grpc.Server

	if paths, err := utility.GetCertificatePaths(); err != nil {
		return nil, err
	} else if paths.Certificate == "" || paths.PrivateKey == "" {
		server = grpc.NewServer()
	} else {
		certPool := x509.NewCertPool()

		if certificate, err := tls.LoadX509KeyPair(paths.Certificate, paths.PrivateKey); err != nil {
			return nil, fmt.Errorf("failed to read certificate files: %s", err)
		} else if bs, err := os.ReadFile(paths.Certificate); err != nil {
			return nil, fmt.Errorf("failed to read client ca cert: %s", err)
		} else if ok := certPool.AppendCertsFromPEM(bs); !ok {
			return nil, fmt.Errorf("failed to append client certs")
		} else {

			transportCreds := credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{certificate},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    certPool,
				RootCAs:      certPool,
				ServerName:   "localhost",
			})

			server = grpc.NewServer(grpc.Creds(transportCreds))
		}
	}

	api.RegisterVMWareDesktopAutoscalerServiceServer(server, g)

	return server, nil
}

func (g *Grpc) Inflight() int {
	return g.inflight
}

func (g *Grpc) consume() {

	defer func() {
		g.Halted = true
		g.Debugf("sending halt notification")
		g.HaltedChan <- true
	}()

	go func() {
		reflection.Register(g.server)

		if listener, err := net.Listen(g.transport, g.address); err != nil {
			g.Errorf("failed to listen: %v", err)
		} else if err = g.server.Serve(listener); err != nil {
			g.Errorf("failed to serve: %v", err)
		}
	}()

	//	go http.Serve(g.listener, http.HandlerFunc(g.RequestHandler))

	if <-g.stopChan {
		g.Debugf("stop notification received - closing")
		g.server.Stop()
		g.Debugf("wait for inflight requests to complete")
		g.reqTracker.Wait()
		g.Debugf("grpc consumer halted")
	}
}

func (g *Grpc) incrementInflight() {
	g.reqTracker.Add(1)
	g.inflight++
}

func (g *Grpc) decrementInflight() {
	g.inflight--
	g.reqTracker.Done()
}

func (g *Grpc) Create(ctx context.Context, req *api.CreateRequest) (*api.CreateResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	networks := make([]*service.NetworkInterface, 0, len(req.Networks))

	for _, network := range req.Networks {
		networks = append(networks, &service.NetworkInterface{
			MacAddress:     network.Macaddress,
			Vnet:           network.Vnet,
			ConnectionType: network.Type,
			Device:         network.Device,
			BsdName:        network.BsdName,
			DisplayName:    network.DisplayName,
		})
	}

	if result, err := g.vmrun.Create(&service.CreateVirtualMachine{Template: req.Template, Name: req.Name, Vcpus: int(req.Vcpus), Memory: int(req.Memory), DiskSizeInMb: int(req.DiskSizeInMb), Networks: networks, GuestInfos: req.GuestInfos, Linked: req.Linked}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.CreateResponse{
			Response: &api.CreateResponse_Error{
				Error: &api.ClientError{
					Code:   500,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.CreateResponse{Response: &api.CreateResponse_Result{
			Result: &api.CreateReply{
				Machine: &api.VirtualMachine{
					Uuid:        result.Uuid,
					Vmx:         result.Path,
					Vcpus:       req.Vcpus,
					Memory:      req.Memory,
					Powered:     result.Powered,
					Address:     result.Address,
					ToolsStatus: result.ToolsStatus,
				},
			},
		}}, nil
	}
}

func (g *Grpc) Delete(ctx context.Context, req *api.VirtualMachineRequest) (*api.DeleteResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.Delete(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.DeleteResponse{
			Response: &api.DeleteResponse_Error{
				Error: &api.ClientError{
					Code:   500,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.DeleteResponse{
			Response: &api.DeleteResponse_Result{
				Result: &api.DeleteReply{
					Done: result,
				},
			},
		}, nil
	}
}

func (g *Grpc) PowerOn(ctx context.Context, req *api.VirtualMachineRequest) (*api.PowerOnResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.PowerOn(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.PowerOnResponse{
			Response: &api.PowerOnResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.PowerOnResponse{
			Response: &api.PowerOnResponse_Result{
				Result: &api.PowerOnReply{
					Done: result,
				},
			},
		}, nil
	}
}

func (g *Grpc) PowerOff(ctx context.Context, req *api.VirtualMachineRequest) (*api.PowerOffResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.PowerOff(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.PowerOffResponse{
			Response: &api.PowerOffResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.PowerOffResponse{
			Response: &api.PowerOffResponse_Result{
				Result: &api.PowerOffReply{
					Done: result,
				},
			},
		}, nil
	}
}

func (g *Grpc) PowerState(ctx context.Context, req *api.VirtualMachineRequest) (*api.PowerStateResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.PowerState(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.PowerStateResponse{
			Response: &api.PowerStateResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.PowerStateResponse{
			Response: &api.PowerStateResponse_Powered{
				Powered: result,
			},
		}, nil
	}
}

func (g *Grpc) ShutdownGuest(ctx context.Context, req *api.VirtualMachineRequest) (*api.ShutdownGuestResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.ShutdownGuest(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.ShutdownGuestResponse{
			Response: &api.ShutdownGuestResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.ShutdownGuestResponse{
			Response: &api.ShutdownGuestResponse_Result{
				Result: &api.PowerOffReply{
					Done: result,
				},
			},
		}, nil
	}
}

func (g *Grpc) Status(ctx context.Context, req *api.VirtualMachineRequest) (*api.StatusResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.Status(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.StatusResponse{
			Response: &api.StatusResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		networks := make([]*api.Ethernet, 0, len(result.EthernetCards))

		for _, ether := range result.EthernetCards {
			networks = append(networks, &api.Ethernet{
				AddressType:            ether.AddressType,
				Address:                ether.IP4Address,
				BsdName:                ether.BsdName,
				ConnectionType:         ether.ConnectionType,
				DisplayName:            ether.DisplayName,
				GeneratedAddress:       ether.MacAddress,
				GeneratedAddressOffset: int32(ether.MacAddressOffset),
				LinkStatePropagation:   ether.LinkStatePropagation,
				PciSlotNumber:          int32(ether.PciSlotNumber),
				Present:                ether.Present,
				VirtualDev:             ether.VirtualDev,
				Vnet:                   ether.Vnet,
			})
		}

		return &api.StatusResponse{
			Response: &api.StatusResponse_Result{
				Result: &api.StatusReply{
					Powered:  result.Powered,
					Ethernet: networks,
				},
			},
		}, nil
	}
}

func (g *Grpc) WaitForIP(ctx context.Context, req *api.VirtualMachineRequest) (*api.WaitForIPResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if address, err := g.vmrun.WaitForIP(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.WaitForIPResponse{
			Response: &api.WaitForIPResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.WaitForIPResponse{
			Response: &api.WaitForIPResponse_Result{
				Result: &api.WaitForIPReply{
					Address: address,
				},
			},
		}, nil
	}
}

func (g *Grpc) WaitForToolsRunning(ctx context.Context, req *api.VirtualMachineRequest) (*api.WaitForToolsRunningResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()
	if running, err := g.vmrun.WaitForToolsRunning(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.WaitForToolsRunningResponse{
			Response: &api.WaitForToolsRunningResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.WaitForToolsRunningResponse{
			Response: &api.WaitForToolsRunningResponse_Result{
				Result: &api.WaitForToolsRunningReply{
					Running: running,
				},
			},
		}, nil
	}
}

func (g *Grpc) SetAutoStart(ctx context.Context, req *api.AutoStartRequest) (*api.AutoStartResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if result, err := g.vmrun.SetAutoStart(req.Uuid, req.Autostart); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.AutoStartResponse{
			Response: &api.AutoStartResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.AutoStartResponse{
			Response: &api.AutoStartResponse_Result{
				Result: &api.AutoStartReply{
					Done: result,
				},
			},
		}, nil
	}
}

func (g *Grpc) VirtualMachineByName(ctx context.Context, req *api.VirtualMachineRequest) (*api.VirtualMachineResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if vm, err := g.vmrun.VirtualMachineByName(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.VirtualMachineResponse{
			Response: &api.VirtualMachineResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.VirtualMachineResponse{
			Response: &api.VirtualMachineResponse_Result{
				Result: &api.VirtualMachine{
					Uuid:        vm.Uuid,
					Vmx:         vm.Path,
					Name:        vm.Name,
					Vcpus:       int32(vm.Vcpus),
					Memory:      int64(vm.Memory),
					Powered:     vm.Powered,
					Address:     vm.Address,
					ToolsStatus: vm.ToolsStatus,
				},
			},
		}, nil
	}
}

func (g *Grpc) VirtualMachineByUUID(ctx context.Context, req *api.VirtualMachineRequest) (*api.VirtualMachineResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if vm, err := g.vmrun.VirtualMachineByUUID(req.Identifier); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.VirtualMachineResponse{
			Response: &api.VirtualMachineResponse_Error{
				Error: &api.ClientError{
					Code:   404,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		return &api.VirtualMachineResponse{
			Response: &api.VirtualMachineResponse_Result{
				Result: &api.VirtualMachine{
					Uuid:        vm.Uuid,
					Vmx:         vm.Path,
					Name:        vm.Name,
					Vcpus:       int32(vm.Vcpus),
					Memory:      int64(vm.Memory),
					Powered:     vm.Powered,
					Address:     vm.Address,
					ToolsStatus: vm.ToolsStatus,
				},
			},
		}, nil
	}
}

func (g *Grpc) ListVirtualMachines(ctx context.Context, req *api.VirtualMachinesRequest) (*api.VirtualMachinesResponse, error) {
	g.incrementInflight()

	defer g.decrementInflight()

	if vms, err := g.vmrun.ListVirtualMachines(); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		return &api.VirtualMachinesResponse{
			Response: &api.VirtualMachinesResponse_Error{
				Error: &api.ClientError{
					Code:   500,
					Reason: err.Error(),
				},
			},
		}, nil
	} else {
		machines := make([]*api.VirtualMachine, 0, len(vms))

		for _, vm := range vms {
			machines = append(machines, &api.VirtualMachine{
				Uuid:        vm.Uuid,
				Name:        vm.Name,
				Vmx:         vm.Path,
				Vcpus:       int32(vm.Vcpus),
				Memory:      int64(vm.Memory),
				Powered:     vm.Powered,
				Address:     vm.Address,
				ToolsStatus: vm.ToolsStatus,
			})
		}

		return &api.VirtualMachinesResponse{
			Response: &api.VirtualMachinesResponse_Result{
				Result: &api.VirtualMachinesReply{
					Machines: machines,
				},
			},
		}, nil
	}
}
