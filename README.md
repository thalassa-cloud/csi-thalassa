# csi-thalassa

A Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) Driver for [Thalassa Cloud Block Storage](https://docs.thalassa.cloud/docs/iaas/). The CSI plugin allows you to use Thalassa Cloud Block Storage with your preferred Container Orchestrator on Thalassa Cloud.
The Thalassa Cloud CSI plugin is only tested on Kubernetes.

## Releases

The Thalassa Cloud CSI plugin follows [semantic versioning](https://semver.org/).
The version will be bumped following the rules below:

- Bug fixes will be released as a `PATCH` update.
- New features (such as CSI spec bumps with no breaking changes) will be released as a `MINOR` update.
- Significant breaking changes makes a `MAJOR` update.

## Installing to Kubernetes

### Kubernetes Compatibility

The following table describes the required Thalassa Cloud CSI driver version per supported Kubernetes release.

| Kubernetes Release | Thalassa Cloud CSI Driver Version |
| ------------------ | --------------------------------- |
| 1.31               | v0.1.0+                           |
| 1.32               | v0.1.0+                           |
| 1.33               | v0.1.0+                           |

**Note:**

The [Thalassa Cloud Kubernetes](https://docs.thalassa.cloud/docs/platform/kubernetes/) service comes with the CSI driver pre-installed and no further steps are required.

## Credits

The Thalassa Cloud CSI project is loosely based on other CSI projects, such as DigitalOcean CSI, Scaleway CSI and others.

## Contributing

All contributions are welcome.
