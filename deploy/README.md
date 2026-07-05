# Deploy the Thalassa CSI driver

Kubernetes manifests for installing [csi-thalassa](https://github.com/thalassa-cloud/csi-thalassa) on a self-managed Thalassa Cloud Kubernetes cluster.

> Thalassa Cloud managed Kubernetes ships with this driver pre-installed. Use these manifests when you run your own cluster or need to install a specific version.

## Layout

```
deploy/
├── README.md
├── namespace.yaml
├── rbac.yaml
├── csidriver.yaml
├── controller.yaml
├── node.yaml
└── secret.yaml.example
```

## Prerequisites

- A Thalassa Cloud Kubernetes cluster with worker nodes in the target region
- `kubectl` configured for the cluster
- `envsubst` (GNU gettext)
- API credentials (client ID + secret) with permission to manage block volumes in the organisation/project
- The [Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter#usage) if you use volume snapshots

## Configuration

Export the variables below, then render and apply the manifests with `envsubst`.

| Variable | Required | Example | Description |
|----------|----------|---------|-------------|
| `CSI_NAMESPACE` | No | `thalassa-system` | Namespace for the driver |
| `CSI_IMAGE` | Yes | `ghcr.io/thalassa-cloud/csi-thalassa:v0.5.0` | Controller and node plugin image |
| `CSI_DRIVER_NAME` | No | `csi.k8s.thalassa.cloud` | CSIDriver name and kubelet plugin path |
| `THALASSA_API_URL` | No | `https://api.thalassa.cloud` | Thalassa Cloud API endpoint |
| `THALASSA_ORGANISATION_ID` | Yes | `o-…` | Organisation identity |
| `THALASSA_PROJECT_ID` | No | `p-…` | Project identity (optional) |
| `THALASSA_REGION` | Yes | `nl-01` | Region slug or identity |
| `THALASSA_CLUSTER_ID` | Yes | `k8s-…` | Kubernetes cluster identity |
| `THALASSA_VPC_ID` | Yes | `vpc-…` | VPC identity the cluster runs in |

Find cluster, VPC, and region identities with [`tcloud`](https://docs.thalassa.cloud/docs/cli/):

```bash
tcloud kubernetes list
tcloud networking vpcs list
```

## Install

### 1. Set variables

```bash
export CSI_NAMESPACE=thalassa-system
export CSI_IMAGE=ghcr.io/thalassa-cloud/csi-thalassa:v0.5.0
export CSI_DRIVER_NAME=csi.k8s.thalassa.cloud

export THALASSA_API_URL=https://api.thalassa.cloud
export THALASSA_ORGANISATION_ID=o-your-organisation
export THALASSA_PROJECT_ID=p-your-project        # optional
export THALASSA_REGION=nl-01
export THALASSA_CLUSTER_ID=k8s-your-cluster
export THALASSA_VPC_ID=vpc-your-vpc
```

### 2. Create the credentials secret

```bash
kubectl create namespace "${CSI_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${CSI_NAMESPACE}" create secret generic thalassa-cloud-credentials \
  --from-literal=client_id='your-client-id' \
  --from-literal=client_secret='your-client-secret' \
  --dry-run=client -o yaml | kubectl apply -f -
```

Or copy `secret.yaml.example`, fill in the values, and apply it.

### 3. Apply manifests

```bash
for manifest in namespace rbac csidriver controller node; do
  envsubst <"deploy/${manifest}.yaml" | kubectl apply -f -
done
```

### 4. Verify

```bash
kubectl -n "${CSI_NAMESPACE}" rollout status deployment/thalassa-csi-controller
kubectl -n "${CSI_NAMESPACE}" rollout status daemonset/thalassa-csi-node
kubectl -n "${CSI_NAMESPACE}" get pods -o wide
```

## Storage classes

Create a `StorageClass` that references `CSI_DRIVER_NAME` (default `csi.k8s.thalassa.cloud`). Thalassa Cloud Kubernetes clusters typically provide a default class such as `tc-block`.

See [`examples/`](../examples/) for sample PVCs and workloads.

## Uninstall

```bash
for manifest in node controller csidriver rbac namespace; do
  envsubst <"deploy/${manifest}.yaml" | kubectl delete -f - --ignore-not-found
done

kubectl -n "${CSI_NAMESPACE}" delete secret thalassa-cloud-credentials --ignore-not-found
```

## Notes

- The controller uses an in-pod kubeconfig (`ConfigMap/thalassa-csi-kubeconfig`) so it can resolve node provider IDs from the Kubernetes API.
- Node pods run privileged and use `hostNetwork` to register with the kubelet.
- Health checks are served by the plugin on port `10301` (`/health`).
