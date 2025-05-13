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
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func (d *Driver) getNodeMachineIdentity(ctx context.Context, nodeName string) (string, error) {
	config, err := clientcmd.BuildConfigFromFlags("", d.kubeConfig)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to build kube config: %s", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to build kubernetes client: %s", err)
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get node: %s", err)
	}

	if node.Spec.ProviderID == "" {
		return "", status.Errorf(codes.Internal, "node %q does not have a provider ID", nodeName)
	}

	// split the providerID to get the machine identity
	providerIDParts := strings.Split(node.Spec.ProviderID, "://")
	if len(providerIDParts) != 2 {
		return "", status.Errorf(codes.Internal, "invalid provider ID: %q", node.Spec.ProviderID)
	}
	return providerIDParts[1], nil
}
