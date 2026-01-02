# KubeAccess

`kubeaccess` is a CLI tool designed to inspect Kubernetes RBAC access levels for users and service accounts, and to generate RBAC manifests (Role/RoleBinding or ClusterRole/ClusterRoleBinding).

## Features

- **Check Access Level**: Verify if a specific user or service account has access to a resource in a namespace or cluster-wide.
- **Generate RBAC Manifests**: Automatically generate YAML manifests for Roles and Bindings based on desired access.
- **Impersonation Support**: Uses Kubernetes impersonation to accurately check effective permissions.

## Prerequisites

- Go 1.20+ (for building from source)
- A configured `~/.kube/config` file.
- **Permissions**: The user running this tool must have permissions to impersonate other users and groups (`system:masters` or similar privileges are often required for checking access of others).

## Installation

### From Source

Using Make:

```bash
git clone https://github.com/vasudevchavan/k8rbac-eval.git
cd k8rbac-eval
make build
# Binary will be in bin/kubeaccess

# Build for all platforms
make build-all
# Binaries will be in bin/ with suffixes (e.g., kubeaccess-linux-amd64)
```

Using Go directly:

```bash
git clone https://github.com/vasudevchavan/k8rbac-eval.git
cd k8rbac-eval
go install
```

## Usage

The CLI supports two main commands: `show` and `generate`.

### Show Access

Inspect the access level of a user or service account.

```bash
# Check user access for a resource in a namespace
kubeaccess show user <username> -n <namespace> --resource <resource>

# Check service account access
kubeaccess show sa <serviceaccount> -n <namespace> --resource <resource>
```

**Flags:**

- `-n, --namespace`: (Optional) Target namespace (default: "default").
- `--resource`: (Required) The Kubernetes resource to check (e.g., `pods`, `deployments`).
- `-c, --clusterscope`: (Optional) Check cluster-level access.

### Generate Manifests

Generate RBAC YAML manifests for a user or service account.

```bash
# Generate Role/Binding for a user
kubeaccess generate user <username> --resource <resource> --verb <verbs>

# Generate Role/Binding for a Service Account
kubeaccess generate sa <serviceaccount> --resource <resource> --verb <verbs>
```

**Flags:**

- `--verb`: (Optional) Verbs to include in the rule (default: `get`, `list`, `watch`). Can be repeated.
- `--resource`: (Required) Resource for the Role.
- `-n, --namespace`: (Optional) Namespace for the Role/Binding.

## Examples

### Checking Access

Check if user `alice` can access `pods` in `default` namespace:

```bash
kubeaccess show user alice -n default --resource pods
```

Check if service account `my-app` has `secrets` access:

```bash
kubeaccess show sa my-app -n default --resource secrets
```

### Generating Access

Generate a Role allowing `bob` to `create` and `delete` `deployments`:

```bash
kubeaccess generate user bob --resource deployments --verb create --verb delete
```

Generate a ClusterRole for a service account to view nodes:

# Generate a ClusterRole for a service account to view nodes

kubeaccess generate sa monitor-sa --resource nodes --verb get --verb list --clusterscope

# Generate a ClusterRole for a user 'deployer' to access nodes

kubeaccess generate user deployer --resource=nodes --verb=list,get,watch -c

````

### Checking Cluster-Wide Access

Check all cluster-scoped access for user `alice`:

```bash
kubeaccess show user alice -c
````
