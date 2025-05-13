/*
Copyright 2025 Thalassa Cloud

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	iaas "github.com/thalassa-cloud/client-go/iaas"
	"github.com/thalassa-cloud/client-go/pkg/client"

	"github.com/thalassa-cloud/csi-thalassa/driver/defaults"
	"github.com/thalassa-cloud/csi-thalassa/driver/healthcheck"
	"github.com/thalassa-cloud/csi-thalassa/driver/version"
)

type Driver struct {
	name string
	// publishInfoVolumeName is used to pass the volume name from
	// `ControllerPublishVolume` to `NodeStageVolume or `NodePublishVolume`
	publishInfoVolumeName string

	endpoint               string
	debugAddr              string
	region                 string
	nodeID                 string
	defaultVolumesPageSize uint

	validateAttachment bool

	srv     *grpc.Server
	httpSrv *http.Server
	log     *slog.Logger
	mounter Mounter

	iaas *iaas.Client

	healthChecker *healthcheck.HealthChecker

	// ready defines whether the driver is ready to function. This value will
	// be used by the `Identity` service via the `Probe()` method.
	readyMu     sync.Mutex // protects ready
	ready       bool
	volumeLimit uint
	kubeConfig  string
}

// NewDriverParams defines the parameters that can be passed to NewDriver.
type NewDriverParams struct {
	CsiEndpoint string

	ThalassaToken        string
	ThalassaClientID     string
	ThalassaClientSecret string
	ThalassaURL          string
	ThalassaOrganisation string
	ThalassaInsecure     bool
	Region               string

	DriverName  string
	DebugAddr   string
	VolumeLimit uint
	NodeID      string
	KubeConfig  string
}

// NewDriver returns a CSI plugin that contains the necessary gRPC
// interfaces to interact with Kubernetes over unix domain sockets for
// managing DigitalOcean Block Storage
func NewDriver(p NewDriverParams) (*Driver, error) {
	driverName := p.DriverName
	if driverName == "" {
		driverName = defaults.DefaultDriverName
	}

	region := p.Region
	nodeId := p.NodeID

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log = log.With("region", region, "node_id", nodeId, "version", version.GetVersion())

	opts := []client.Option{
		client.WithBaseURL(p.ThalassaURL),
		client.WithOrganisation(p.ThalassaOrganisation),
	}

	if p.ThalassaInsecure {
		log.Warn("Insecure mode for API access enabled. Only use this in development environments.")
		opts = append(opts, client.WithInsecure())
	}

	if p.ThalassaClientID != "" && p.ThalassaClientSecret != "" {
		log.Info("Using OIDC for API access")
		if p.ThalassaInsecure {
			opts = append(opts, client.WithAuthOIDCInsecure(p.ThalassaClientID, p.ThalassaClientSecret, fmt.Sprintf("%s/oidc/token", p.ThalassaURL), p.ThalassaInsecure))
		} else {
			opts = append(opts, client.WithAuthOIDC(p.ThalassaClientID, p.ThalassaClientSecret, fmt.Sprintf("%s/oidc/token", p.ThalassaURL)))
		}
	} else if p.ThalassaToken != "" {
		log.Warn("Using personal token for API access. Only use this in development environments. Prefer using OIDC.")
		opts = append(opts, client.WithAuthPersonalToken(p.ThalassaToken))
	} else {
		log.Warn("No authentication method provided. This may only work in development environments")
	}

	tcClient, err := client.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Thalassa client: %s", err)
	}

	log.Info("Initializing Thalassa IaaS client")
	iaasClient, err := iaas.New(tcClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Thalassa IaaS client: %s", err)
	}

	healthChecker := healthcheck.NewHealthChecker(&tcHealthChecker{tcClient: tcClient})

	return &Driver{
		name:                  driverName,
		publishInfoVolumeName: driverName + "/volume-name",
		endpoint:              p.CsiEndpoint,
		debugAddr:             p.DebugAddr,
		volumeLimit:           p.VolumeLimit,
		nodeID:                nodeId,
		region:                region,
		log:                   log,
		iaas:                  iaasClient,
		healthChecker:         healthChecker,
	}, nil
}

// Run starts the CSI plugin by communication over the given endpoint
func (d *Driver) Run(ctx context.Context) error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	grpcAddr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		grpcAddr = filepath.FromSlash(u.Path)
	}

	// CSI plugins talk only over UNIX sockets currently
	if u.Scheme != "unix" {
		return fmt.Errorf("currently only unix domain sockets are supported, have: %s", u.Scheme)
	}
	// remove the socket if it's already there. This can happen if we
	// deploy a new version and the socket was created from the old running
	// plugin.
	d.log.Info("removing socket", "socket", grpcAddr)
	if err := os.Remove(grpcAddr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", grpcAddr, err)
	}

	grpcListener, err := net.Listen(u.Scheme, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// log response errors for better observability
	errHandler := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			d.log.Error("method failed", "method", info.FullMethod, "error", err)
		}
		return resp, err
	}

	if d.debugAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			err := d.healthChecker.Check(r.Context())
			if err != nil {
				d.log.Error("executing health check", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		d.httpSrv = &http.Server{
			Addr:    d.debugAddr,
			Handler: mux,
		}
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)

	d.ready = true
	d.log.Info("starting server", "grpc_addr", grpcAddr, "http_addr", d.debugAddr)

	var eg errgroup.Group
	if d.httpSrv != nil {
		eg.Go(func() error {
			<-ctx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return d.httpSrv.Shutdown(ctx)
		})
		eg.Go(func() error {
			err := d.httpSrv.ListenAndServe()
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		})
	}
	eg.Go(func() error {
		go func() {
			<-ctx.Done()
			d.log.Info("server stopped")
			d.readyMu.Lock()
			d.ready = false
			d.readyMu.Unlock()
			d.srv.GracefulStop()
		}()
		return d.srv.Serve(grpcListener)
	})
	return eg.Wait()
}
