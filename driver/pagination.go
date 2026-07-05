package driver

import (
	"sort"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func paginateIdentities(identities []string, startingToken string, maxEntries int32) (sortedIdentities []string, start, end int, nextToken string, err error) {
	if maxEntries < 0 {
		return nil, 0, 0, "", status.Error(codes.InvalidArgument, "max entries cannot be negative")
	}

	sortedIdentities = append([]string(nil), identities...)
	sort.Strings(sortedIdentities)

	start = 0
	if startingToken != "" {
		found := false
		for i, id := range sortedIdentities {
			if id == startingToken {
				start = i
				found = true
				break
			}
		}
		if !found {
			return nil, 0, 0, "", status.Errorf(codes.Aborted, "invalid starting token %q", startingToken)
		}
	}

	end = len(sortedIdentities)
	if maxEntries > 0 && int(maxEntries) < end-start {
		end = start + int(maxEntries)
		nextToken = sortedIdentities[end]
	}

	return sortedIdentities, start, end, nextToken, nil
}
