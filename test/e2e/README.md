# End-to-end tests

This directory runs the upstream Kubernetes [external storage CSI tests](https://github.com/kubernetes/kubernetes/tree/master/test/e2e/storage/external) against a real Thalassa Cloud Kubernetes cluster.

> **Warning**
> These tests create billable Thalassa Cloud resources (VPC, subnet, NAT gateway, Kubernetes cluster, block volumes, snapshots). Teardown runs automatically in CI, but failed runs may leave orphaned resources. Review your organisation if a job fails mid-run.

## What runs

1. Provision infrastructure with [`tcloud`](https://docs.thalassa.cloud/docs/cli/) (VPC, subnet, NAT gateway, Kubernetes cluster, node pool)
2. Build and push the CSI driver image to a container registry
3. Deploy the driver under the e2e driver name `csi.k8s.e2e.thalassa.cloud`
4. Run `e2e.test` with `test/e2e/test-driver.yaml` (full suite by default, or smoke subset via `E2E_TEST_PROFILE=smoke`)
5. Tear down the cluster and networking resources

## GitHub Actions

Workflow: [`.github/workflows/e2e.yml`](../../.github/workflows/e2e.yml)

| Trigger | Test profile | Behaviour |
|---------|--------------|-----------|
| Push to `main` | `full` | Full external storage suite (~2+ hours of tests) |
| Pull request (same repo) | `smoke` | Ext4 dynamic PV subset (~11 specs, ~20 minutes of tests) |
| Pull request (fork) | `smoke` | Waits for approval via the `e2e-fork-approval` environment |
| Manual | selectable | Use **Actions → e2e → Run workflow** and pick `smoke` or `full` |

### Repository secrets

| Secret | Required | Description |
|--------|----------|-------------|
| `THALASSA_CLIENT_ID` | Yes* | Thalassa Cloud OIDC client ID for `tcloud` |
| `THALASSA_CLIENT_SECRET` | Yes* | Thalassa Cloud OIDC client secret for `tcloud` |
| `THALASSA_TOKEN` | Alt. | Personal access token for `tcloud` |
| `THALASSA_ACCESS_TOKEN` | Alt. | Short-lived access token from OIDC token exchange |
| `THALASSA_SERVICE_ACCOUNT_ID` | Alt. | Service account for OIDC workload identity federation (CI) |
| `THALASSA_ORGANISATION_ID` | Alt. | Organisation ID for OIDC token exchange (falls back to `THALASSA_ORGANISATION_ID`) |
| `THALASSA_ORGANISATION` | Optional | Organisation slug/identity if not in your `tcloud` context |
| `THALASSA_CLIENT_ID` | Yes | API credentials used by the CSI controller |
| `THALASSA_CLIENT_SECRET` | Yes | API credentials used by the CSI controller |
| `THALASSA_ORGANISATION_ID` | Yes | Organisation identity for the CSI driver |
| `THALASSA_PROJECT_ID` | Optional | Project identity for the CSI driver |
| `E2E_REGISTRY_USERNAME` | Yes | Username for the Thalassa container registry |
| `E2E_REGISTRY_PASSWORD` | Yes | Password/token for the Thalassa container registry |

In GitHub Actions, images are pushed to **`registry.thalassacloud.nl/csi-thalassa-dev:e2e-<sha>`** by default. Override with repository variables if you use a different registry.

\* Provide one of: `THALASSA_ACCESS_TOKEN`, `THALASSA_TOKEN`, `THALASSA_CLIENT_ID`/`THALASSA_CLIENT_SECRET`, or OIDC workload identity (`THALASSA_SERVICE_ACCOUNT_ID` plus organisation ID — on GitHub Actions the workflow OIDC token is exchanged automatically).

### Repository variables (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `E2E_REGISTRY` | `registry.thalassacloud.nl` | Container registry host |
| `E2E_IMAGE_REPOSITORY` | `csi-thalassa-dev` | Image path inside the registry |
| `E2E_IMAGE_TAG` | `e2e-<git-sha>` | Container image tag (defaults to `e2e-` + commit) |
| `E2E_REGION` | `nl-01` | Thalassa Cloud region |
| `E2E_AVAILABILITY_ZONE` | `nl-01a` | Availability zone for worker nodes |
| `E2E_MACHINE_TYPE` | `pgp-medium` | Worker machine type |
| `E2E_NODE_COUNT` | `2` | Number of worker nodes |
| `E2E_POD_SECURITY_STANDARDS` | `privileged` | Cluster Pod Security Standards profile (`baseline`, `restricted`, or `privileged`) |
| `E2E_RETRY_ATTEMPTS` | `5` | Retries for transient API failures during bootstrap/teardown |
| `E2E_RETRY_DELAY_SECONDS` | `15` | Delay between retries |

### Fork approval

Create a GitHub environment named **`e2e-fork-approval`** and enable **Required reviewers**. Fork pull requests use this environment so e2e only runs after you approve the workflow.

## Running locally

### Prerequisites

- [`tcloud`](https://docs.thalassa.cloud/docs/cli/installation/) authenticated (`tcloud context create ...`)
- `kubectl`, `jq`, `curl`, `docker`, `go`, `envsubst` (gettext)
- Thalassa API credentials for the CSI driver
- Registry credentials for pushing to `E2E_REGISTRY`

### Full run

```bash
export E2E_REGISTRY=registry.thalassacloud.nl
export E2E_IMAGE_REPOSITORY=csi-thalassa-dev
export E2E_IMAGE_TAG=e2e-$(git rev-parse HEAD)
export E2E_REGISTRY_USERNAME=your-user
export E2E_REGISTRY_PASSWORD=your-token

export THALASSA_CLIENT_ID=...
export THALASSA_CLIENT_SECRET=...
export THALASSA_ORGANISATION_ID=...

bash test/e2e/scripts/run-e2e.sh
```

### Individual steps

```bash
# 1. Provision cluster and write test/e2e/.e2e-state + kubeconfig
make e2e-prepare

# 2. Build and push the CSI image
make e2e-image-push
# Or build only:
# make e2e-image-build

# 3. Deploy the driver to the cluster
make e2e-deploy

# 4. Run the Kubernetes external storage tests only
make e2e-test
# Or a faster smoke subset (~11 ext4 dynamic PV specs, typically ~20 minutes):
# make e2e-test-smoke

# 5. Delete cloud resources
make e2e-teardown
```

The smoke profile runs only `External.Storage` tests for ext4 dynamic PV provisioning. Override ginkgo filters when calling `e2e.sh` directly:

```bash
E2E_TEST_PROFILE=smoke make e2e-test-smoke
# or customize:
E2E_GINKGO_FOCUS='External.Storage.*csi.k8s.e2e.thalassa.cloud.*should store data' \
  E2E_TEST_PROFILE=full make e2e-test
```

Credentials can live in a gitignored `.env` file at the repo root; `make` loads it automatically for e2e targets.

Set `E2E_SKIP_TEARDOWN=true` to keep the cluster after a local run.

## Layout

```
test/e2e/
├── README.md
├── e2e.sh                 # Downloads e2e.test and runs external storage tests
├── test-driver.yaml         # CSI test driver capabilities
├── manifests/               # Kubernetes manifests (envsubst placeholders)
├── scripts/
│   ├── common.sh            # Shared env vars and helpers
│   ├── install-tcloud.sh    # Installs tcloud in CI/local environments
│   ├── bootstrap-cluster.sh # Creates VPC, subnet, Kubernetes cluster
│   ├── teardown-cluster.sh
│   ├── build-push-image.sh  # Builds and pushes to E2E_REGISTRY (see `make e2e-image-push`)
│   ├── deploy-driver.sh     # Installs CSI controller + node plugin
│   └── run-e2e.sh           # Orchestrates the full flow (used by CI)
├── .e2e-state               # Created locally/CI; tracks resource IDs
└── kubeconfig               # Created by bootstrap; gitignored
```

## Driver configuration

E2E uses driver name **`csi.k8s.e2e.thalassa.cloud`** (see `test-driver.yaml`) so tests do not collide with a production driver installation.

The deploy manifests set:

- Controller `--kube-config` for resolving node provider IDs
- `--vpc` and `--cluster` from the bootstrapped infrastructure
- `--validate-attachment=true` on node pods

## Troubleshooting

- **`tcloud` auth failures**: set `THALASSA_ACCESS_TOKEN`, `THALASSA_TOKEN`, `THALASSA_CLIENT_ID`/`THALASSA_CLIENT_SECRET`, or configure OIDC workload identity with `THALASSA_SERVICE_ACCOUNT_ID`.
- **Image pull errors**: verify `E2E_REGISTRY`, credentials, and that the cluster can reach the registry.
- **Leftover resources**: inspect `test/e2e/.e2e-state` and run `teardown-cluster.sh`, or delete resources manually in the Thalassa console.
- **SSH disruptive tests**: skipped by default via `KUBE_SSH_KEY_PATH=/fake/path/to/skip/ssh/tests`.
