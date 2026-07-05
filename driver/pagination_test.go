package driver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPaginateIdentities(t *testing.T) {
	identities := []string{"vol-a", "vol-b", "vol-c", "vol-d"}

	tests := []struct {
		name          string
		startingToken string
		maxEntries    int32
		wantStart     int
		wantEnd       int
		wantNextToken string
		wantCode      codes.Code
	}{
		{
			name:          "returns first page",
			maxEntries:    2,
			wantStart:     0,
			wantEnd:       2,
			wantNextToken: "vol-c",
		},
		{
			name:          "returns second page",
			startingToken: "vol-c",
			maxEntries:    2,
			wantStart:     2,
			wantEnd:       4,
		},
		{
			name:          "returns all entries when max entries is zero",
			maxEntries:    0,
			wantStart:     0,
			wantEnd:       4,
		},
		{
			name:          "rejects invalid starting token",
			startingToken: "missing",
			maxEntries:    2,
			wantCode:      codes.Aborted,
		},
		{
			name:       "rejects negative max entries",
			maxEntries: -1,
			wantCode:   codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortedIdentities, start, end, nextToken, err := paginateIdentities(identities, tt.startingToken, tt.maxEntries)
			if tt.wantCode != codes.OK {
				require.Error(t, err)
				require.Equal(t, tt.wantCode, status.Code(err))
				return
			}

			require.NoError(t, err)
			require.Equal(t, identities[tt.wantStart:tt.wantEnd], sortedIdentities[start:end])
			require.Equal(t, tt.wantNextToken, nextToken)
		})
	}
}
