package driver

import (
	iaas "github.com/thalassa-cloud/client-go/iaas"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func getVolumeTypeByFilters(volumeTypes []iaas.VolumeType, f ...func(volumeType iaas.VolumeType) bool) (string, error) {
	for _, filter := range f {
		volumeType, err := getVolumeTypeByFilter(volumeTypes, filter)
		if err != nil {
			continue
		}
		return volumeType, nil
	}
	return "", status.Errorf(codes.NotFound, "volume type not found")
}

func getVolumeTypeByFilter(volumeTypes []iaas.VolumeType, f func(volumeType iaas.VolumeType) bool) (string, error) {
	for _, volumeType := range volumeTypes {
		if f(volumeType) {
			return volumeType.Identity, nil
		}
	}
	return "", status.Errorf(codes.NotFound, "volume type not found")
}
