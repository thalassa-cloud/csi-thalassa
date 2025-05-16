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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thalassa-cloud/csi-thalassa/driver"
	"github.com/thalassa-cloud/csi-thalassa/driver/defaults"
	"github.com/thalassa-cloud/csi-thalassa/driver/version"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Start the Thalassa CSI plugin",
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := viper.GetString("mode")
		log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		log.Info("Starting Thalassa CSI plugin", "version", version.GetVersion(), "commit", version.GetCommit(), "tree_state", version.GetTreeState(), "mode", mode)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-c
			cancel()
		}()

		switch mode {
		case "controller":
			controller, err := driver.NewDriver(driver.NewDriverParams{
				CsiEndpoint: viper.GetString("csi-endpoint"),

				ThalassaToken:        viper.GetString("thalassa-token"),
				ThalassaClientID:     viper.GetString("thalassa-client-id"),
				ThalassaClientSecret: viper.GetString("thalassa-client-secret"),
				ThalassaURL:          viper.GetString("thalassa-url"),
				ThalassaInsecure:     viper.GetBool("thalassa-insecure"),
				Region:               viper.GetString("thalassa-region"),

				DriverName:           viper.GetString("driver-name"),
				DebugAddr:            viper.GetString("debug-addr"),
				VolumeLimit:          viper.GetUint("volume-limit"),
				ThalassaOrganisation: viper.GetString("organisation"),
				KubeConfig:           viper.GetString("kube-config"),
				NodeID:               viper.GetString("node-id"),
				Cluster:              viper.GetString("cluster"),
				Vpc:                  viper.GetString("vpc"),
			})
			if err != nil {
				return fmt.Errorf("failed to create controller: %w", err)
			}

			if err := controller.Run(ctx); err != nil {
				return fmt.Errorf("failed to run driver: %w", err)
			}
		case "node":
			drv, err := driver.NewNodeDriver(driver.NewNodeDriverParams{
				CsiEndpoint:        viper.GetString("csi-endpoint"),
				DriverName:         viper.GetString("driver-name"),
				DebugAddr:          viper.GetString("debug-addr"),
				ValidateAttachment: viper.GetBool("validate-attachment"),
				VolumeLimit:        viper.GetUint("volume-limit"),
				NodeID:             viper.GetString("node-id"),
				Region:             viper.GetString("thalassa-region"),
				Cluster:            viper.GetString("cluster"),
				Vpc:                viper.GetString("vpc"),
			})
			if err != nil {
				return fmt.Errorf("failed to create node driver: %w", err)
			}
			if err := drv.RunNode(ctx); err != nil {
				return fmt.Errorf("failed to run driver: %w", err)
			}
		default:
			return fmt.Errorf("invalid mode: %s", viper.GetString("mode"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)

	// mode
	pluginCmd.Flags().String("mode", "node", "Mode to run the plugin in")

	// Add flags
	pluginCmd.Flags().String("csi-endpoint", "unix:///var/lib/kubelet/plugins/"+defaults.DefaultDriverName+"/csi.sock", "CSI endpoint")
	pluginCmd.Flags().String("driver-name", defaults.DefaultDriverName, "Name for the driver")
	pluginCmd.Flags().String("debug-addr", "", "Address to serve the HTTP debug server on")
	pluginCmd.Flags().Bool("validate-attachment", false, "Validate if the attachment has fully completed before formatting/mounting the device")

	pluginCmd.Flags().Uint("volume-limit", 20, "Volumes per node limit")
	pluginCmd.Flags().String("node-id", "", "Node ID")

	pluginCmd.Flags().String("thalassa-token", "", "Thalassa Cloud access token")
	pluginCmd.Flags().String("thalassa-client-id", "", "Thalassa Cloud client ID")
	pluginCmd.Flags().String("thalassa-client-secret", "", "Thalassa Cloud client secret")
	pluginCmd.Flags().Bool("thalassa-insecure", false, "Use insecure connection to Thalassa Cloud API")
	pluginCmd.Flags().String("thalassa-url", "https://api.thalassa.cloud/", "Thalassa Cloud API URL")
	pluginCmd.Flags().String("thalassa-region", "", "Thalassa Cloud region slug or identity")

	pluginCmd.Flags().String("organisation", "", "Thalassa Cloud organisation")
	pluginCmd.Flags().String("kube-config", "", "Path to kube config file")

	pluginCmd.Flags().String("cluster", "", "Cluster identity of the cluster. This is used to label volumes with the cluster identity")
	pluginCmd.Flags().String("vpc", "", "VPC identity in which the cluster is deployed. This is used for discovering virtual machines to attach volumes to")
	// Bind flags to viper

	viper.BindPFlag("mode", pluginCmd.Flags().Lookup("mode"))

	viper.BindPFlag("csi-endpoint", pluginCmd.Flags().Lookup("csi-endpoint"))
	viper.BindPFlag("driver-name", pluginCmd.Flags().Lookup("driver-name"))
	viper.BindPFlag("debug-addr", pluginCmd.Flags().Lookup("debug-addr"))
	viper.BindPFlag("validate-attachment", pluginCmd.Flags().Lookup("validate-attachment"))

	viper.BindPFlag("volume-limit", pluginCmd.Flags().Lookup("volume-limit"))
	viper.BindPFlag("node-id", pluginCmd.Flags().Lookup("node-id"))

	viper.BindPFlag("thalassa-token", pluginCmd.Flags().Lookup("thalassa-token"))
	viper.BindPFlag("thalassa-client-id", pluginCmd.Flags().Lookup("thalassa-client-id"))
	viper.BindPFlag("thalassa-client-secret", pluginCmd.Flags().Lookup("thalassa-client-secret"))
	viper.BindPFlag("thalassa-insecure", pluginCmd.Flags().Lookup("thalassa-insecure"))
	viper.BindPFlag("thalassa-url", pluginCmd.Flags().Lookup("thalassa-url"))
	viper.BindPFlag("thalassa-region", pluginCmd.Flags().Lookup("thalassa-region"))
	viper.BindPFlag("organisation", pluginCmd.Flags().Lookup("organisation"))
	viper.BindPFlag("kube-config", pluginCmd.Flags().Lookup("kube-config"))

	viper.BindPFlag("cluster", pluginCmd.Flags().Lookup("cluster"))
	viper.BindPFlag("vpc", pluginCmd.Flags().Lookup("vpc"))

}
