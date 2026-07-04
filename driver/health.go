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

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iaas"
)

type iaasHealthClient interface {
	ListRegions(ctx context.Context, listRequest *iaas.ListRegionsRequest) ([]iaas.Region, error)
}

type tcHealthChecker struct {
	iaas   iaasHealthClient
	region string
}

func (c *tcHealthChecker) Name() string {
	return "thalassa"
}

func (c *tcHealthChecker) Check(ctx context.Context) error {
	request := &iaas.ListRegionsRequest{}
	if c.region != "" {
		request.Filters = []filters.Filter{
			&filters.FilterKeyValue{
				Key:   filters.FilterRegion,
				Value: c.region,
			},
		}
	}

	if _, err := c.iaas.ListRegions(ctx, request); err != nil {
		return fmt.Errorf("thalassa API health check failed: %w", err)
	}

	return nil
}
