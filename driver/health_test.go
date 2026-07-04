package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iaas"
)

type mockIaaSHealthClient struct {
	listRegions func(ctx context.Context, listRequest *iaas.ListRegionsRequest) ([]iaas.Region, error)
}

func (m *mockIaaSHealthClient) ListRegions(ctx context.Context, listRequest *iaas.ListRegionsRequest) ([]iaas.Region, error) {
	return m.listRegions(ctx, listRequest)
}

func TestTcHealthCheckerCheck(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		listErr   error
		wantError bool
	}{
		{
			name: "returns nil when API is reachable",
		},
		{
			name:      "returns error when API is unreachable",
			listErr:   errors.New("connection refused"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &tcHealthChecker{
				region: tt.region,
				iaas: &mockIaaSHealthClient{
					listRegions: func(ctx context.Context, listRequest *iaas.ListRegionsRequest) ([]iaas.Region, error) {
						if tt.listErr != nil {
							return nil, tt.listErr
						}
						return []iaas.Region{{Identity: "nl-01"}}, nil
					},
				},
			}

			err := checker.Check(context.Background())
			if tt.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "API health check failed")
				return
			}

			require.NoError(t, err)
		})
	}
}
