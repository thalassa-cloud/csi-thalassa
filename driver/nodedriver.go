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
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/thalassa-cloud/csi-thalassa/driver/defaults"
)

// NewDriverParams defines the parameters that can be passed to NewDriver.
type NewNodeDriverParams struct {
	CsiEndpoint        string
	DriverName         string
	DebugAddr          string
	ValidateAttachment bool
	VolumeLimit        uint
	NodeID             string
	Region             string
	Vpc                string
	Cluster            string
}

// NewDriver returns a CSI plugin that contains the necessary gRPC
// interfaces to interact with Kubernetes over unix domain sockets for
// managing DigitalOcean Block Storage
func NewNodeDriver(p NewNodeDriverParams) (*Driver, error) {
	driverName := p.DriverName
	if driverName == "" {
		driverName = defaults.DefaultDriverName
	}

	nodeId := p.NodeID

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	return &Driver{
		debugAddr:             p.DebugAddr,
		endpoint:              p.CsiEndpoint,
		log:                   log,
		mounter:               NewMounter(log),
		name:                  driverName,
		nodeID:                nodeId,
		publishInfoVolumeName: driverName + "/volume-name",
		region:                p.Region,
		volumeLimit:           p.VolumeLimit,
		vpc:                   p.Vpc,
		clusterIdentity:       p.Cluster,
	}, nil
}

// Run starts the CSI plugin by communication over the given endpoint
func (d *Driver) RunNode(ctx context.Context) error {
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

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterNodeServer(d.srv, d)
	csi.RegisterIdentityServer(d.srv, d)

	d.ready = true // we're now ready to go!
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
