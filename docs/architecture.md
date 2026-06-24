# KubeAccess — Architecture

This document describes the internal design of KubeAccess for developers who want to understand, extend, or contribute to the project.

---

## High-level overview

```
┌─────────────────────────────────────────────────────────────────┐
│  User / CI                                                      │
│    kubeaccess CLI  ──────────────────────────────────────────┐  │
│                                                              │  │
│  Browser                                                     │  │
│    React UI (Vite :3000)                                     │  │
│        │  /api/*  (Vite proxy)                               │  │
│        ▼                                                     │  │
│    Go API server (:8080)  ─── exec ──►  kubeaccess binary ──┘  │
│        │                                                        │
│        ▼                                                        │
│    Kubernetes API server                                        │
└─────────────────────────────────────────────────────────────────┘
```

There are two independent entry points sharing the same library packages:

| Entry point | Path | Role |
|-------------|------|------|
| **CLI** | `main.go` → `internal/cli/` | Direct `kubectl`-style tool |
| **API server** | `cmd/server/main.go` | HTTP wrapper; shells out to the CLI binary |

The API server calls the compiled `kubeaccess` binary as a subprocess. This keeps the server simple and means every UI request goes through exactly the same code path as a direct CLI call.

---

## Package map

```
k8rbac-eval/
├── main.go                    CLI entry point (Cobra root)
├── cmd/
│   └── server/
│       └── main.go            Go HTTP API server
├── internal/
│   └── cli/
│       ├── root.go            Root Cobra command; sets global flags
│       ├── registercommands.go  Wires sub-commands
│       ├── useraccount.go     `kubeaccess show user` handler
│       ├── serviceaccount.go  `kubeaccess show sa` handler
│       ├── generate.go        `kubeaccess generate` handler
│       ├── common.go          RunAccessCheck — shared check logic, fast paths
│       └── version.go         `kubeaccess version` handler
├── pkg/
│   ├── access/
│   │   └── checker.go         KubeChecker: Check, CheckAllNamespaced, CheckAllResources
│   ├── client/
│   │   └── client.go          kubeconfig loading & clientset construction
│   ├── discovery/
│   │   ├── resources.go       GetAllResources via discovery API
│   │   ├── scope.go           IsNamespaced resolver
│   │   └── cache.go           TTL-based ResourceCache
│   ├── generator/
│   │   └── manifests.go       Role/ClusterRole YAML generation
│   └── platform/
│       └── detect.go          Cluster-type detection; cloud-identity helpers
└── ui/
    ├── src/
    │   ├── components/
    │   │   ├── CheckAccess.jsx     Access-check form + results table
    │   │   ├── GenerateRBAC.jsx    Manifest generation form + YAML output
    │   │   └── KubeconfigSelector.jsx  Kubeconfig picker + platform badge
    │   └── api/client.js       Axios API client (120 s timeout)
    ├── mock-api.cjs            Standalone Node mock server for UI dev
    └── vite.config.js          Vite config; proxies /api/* → :8080
```

---

## Request flow — Check Access

### CLI path

```
kubeaccess show user alice --resource pods -n default
    │
    ▼
internal/cli/useraccount.go  →  RunAccessCheck()
    │
    ├─ pkg/client  GetClientsetWithKubeconfig()      (admin clientset)
    ├─ pkg/client  GetRestConfigWithKubeconfig()     (rest.Config for impersonation)
    ├─ pkg/discovery  ResourceCache.Get()            (cached resource list)
    ├─ pkg/discovery  ResourceScopeResolver          (namespaced vs cluster)
    │
    ├─ pkg/access  NewImpersonatedClient()           (sets Impersonate header)
    ├─ pkg/access  KubeChecker.CheckAllNamespaced()  ◄── fast path (1 API call)
    │       or
    │   pkg/access  KubeChecker.CheckAllResources()  ◄── fallback (worker pool)
    │
    └─ stdout: "resource: pods\n  get : true\n  ..."
```

### UI/API-server path

```
Browser  →  POST /api/check
    │
    ▼
cmd/server/main.go  handleCheck()
    │
    ├─ getCachedClient()     (sync.Map keyed by kubeconfig path)
    ├─ detectPlatform()      (cached per-clientset)
    ├─ validateUser()        (RoleBinding scan or platform-specific check)
    │
    └─ exec.Command("kubeaccess", "show", ...)
           Stdout → APIResponse.Output
           Stderr → APIResponse.Warnings / APIResponse.Error
    │
    ▼
Browser  ←  JSON { output, warnings, error }
    │
    ▼
CheckAccess.jsx  parseOutput()   →  rows: [{resource, verb, allowed}]
```

---

## Performance design

### Why it matters

A "check all resources" scan touches every resource in the cluster. With 30+ resources × 7 verbs, a naive sequential approach makes 200+ serial HTTP calls — taking 10–30 seconds on a remote cluster.

### Three-layer strategy

#### 1. SelfSubjectRulesReview (fast path, namespaced checks)

`CheckAllNamespaced` sends **one** `SelfSubjectRulesReview` to the API server. The response lists every rule the user has in the namespace. KubeAccess computes the full access matrix locally by matching resource names and expanding `*` wildcards — no per-resource API calls.

```go
// One API call → full matrix for N resources
resp, _ := k.Client.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, review, ...)
// Walk resp.Status.ResourceRules, handle resource="*" and verb="*"
```

This reduces N×7 API calls to **1 API call** for namespace-scoped checks.

If the API returns `Status.Incomplete = true` (e.g. webhook authorizer that cannot enumerate all rules), KubeAccess falls back to the worker pool.

#### 2. Parallel verb checks (per-resource fallback)

`Check` fires all 7 `SelfSubjectAccessReview` calls concurrently over a single HTTP/2 connection:

```go
ch := make(chan result, len(targetVerbs))
for _, verb := range targetVerbs {
    go func(v string) { /* SSAR create */ ch <- result{...} }(verb)
}
// collect 7 results
```

This makes each resource check ~7× faster than sequential.

#### 3. Worker pool (all-resources fallback)

`CheckAllResources` runs up to `workerCount=10` resource checks concurrently, each using `Check` internally:

```go
sem := make(chan struct{}, workerCount)
for _, res := range resources {
    sem <- struct{}{}
    go func(r string) { defer func() { <-sem }(); Check(ctx, r, ns) }(res)
}
```

Combined with parallel verb calls, this gives up to 70× concurrency (10 resources × 7 verbs) for the worker-pool fallback path.

#### 4. Discovery cache

`pkg/discovery/ResourceCache` caches the list of API resources per kubeconfig with a 5-minute TTL. Resource lists rarely change; this eliminates repeated discovery round-trips across CLI invocations within the same server process.

#### 5. Client-side connection cache

`cmd/server/main.go` uses a `sync.Map` keyed by kubeconfig path to cache `*kubernetes.Clientset` instances. TLS handshakes are done once per kubeconfig; subsequent requests reuse the existing HTTP/2 connection pool.

---

## Platform detection

`pkg/platform.Detect()` is called once at server startup and cached. It probes the cluster in order:

| Platform | Probe |
|----------|-------|
| **OpenShift** | Any API group ending in `.openshift.io` |
| **EKS** | `aws-auth` ConfigMap in `kube-system`, OR node label `eks.amazonaws.com/nodegroup` |
| **AKS** | Node label `kubernetes.azure.com/cluster`; sets `AzureRBACMode` if `kubernetes.azure.com/azure-rbac` is also present |
| **Vanilla** | None of the above |

Detection is lenient — any probe failure is treated as "not that platform" rather than an error.

Platform type influences:
- **User validation** — OpenShift checks the `user.openshift.io` API; EKS checks `aws-auth`; others scan RoleBindings/ClusterRoleBindings.
- **Advisory warnings** — AKS Azure RBAC mode warns that Kubernetes RBAC may be bypassed. EKS/AKS service accounts with IRSA/Workload Identity annotations get cloud-permissions warnings.
- **UI resource chips** — OpenShift surfaces extra resources (`routes`, `buildconfigs`, `securitycontextconstraints`, etc.).

---

## RBAC manifest generation

`pkg/generator/manifests.go` builds Role/ClusterRole + Binding objects in memory using the `k8s.io/api` types and marshals them to YAML with `sigs.k8s.io/yaml`.

Key behaviours:
- `sanitizeName()` converts arbitrary usernames to DNS-label-safe strings for use in `metadata.name`.
- All `yaml.Marshal` errors propagate up — no silent drops.
- Namespace-scoped subjects produce a `Role` + `RoleBinding`; cluster-scoped produce a `ClusterRole` + `ClusterRoleBinding`.

---

## Impersonation

All access checks use Kubernetes impersonation rather than switching kubeconfig context. This means:

1. The **calling user** must have `impersonate` verb on `users`, `groups`, and `serviceaccounts` resources (typically via `system:masters` or a dedicated impersonation `ClusterRole`).
2. The impersonated identity's effective RBAC is evaluated by the API server — results match what that user would actually see.
3. No cluster credentials are ever stored or proxied through KubeAccess.

```go
cfg.Impersonate = rest.ImpersonationConfig{
    UserName: username,
    Groups:   []string{"system:authenticated", ...},
}
```

---

## API server — key design decisions

### No env mutation for kubeconfig

Early versions called `os.Setenv("KUBECONFIG", path)` before each request. Under concurrent requests this creates a race condition. The current design passes the kubeconfig path directly to `GetClientsetWithKubeconfig(path)`, which builds a kubeconfig from scratch without touching the environment.

### Separate stdout / stderr capture

`exec.Command` captures CLI stdout (structured results) and stderr (slog lines) into separate `strings.Builder` instances. Only stdout is returned to the UI as `output`; stderr is returned as `warnings` or `error`. This prevents slog noise from appearing in the parsed results table.

### CORS

The `CORS_ORIGIN` environment variable controls the `Access-Control-Allow-Origin` header (defaults to `*`). Set it to your UI's exact origin in production.

---

## UI architecture

The React frontend is built with **Carbon Design System** components and **Vite**.

### Component tree

```
App.jsx
├── KubeconfigSelector   — kubeconfig dropdown + /api/platform badge
├── CheckAccess          — form + parseOutput() + results table
└── GenerateRBAC         — form + YAML display + copy/download
```

### Output parsing

The CLI writes to stdout in a fixed format:

```
resource: pods
  get                : true
  list               : true
  ...
```

`parseOutput()` in `CheckAccess.jsx` converts this into `{id, resource, verb, allowed}` row objects for the Carbon `DataTable`. A multi-resource result (more than one unique resource) renders a grouped table with resource tags and a filter search box; a single-resource result renders a simpler verb/allowed table.

### Mock API

`ui/mock-api.cjs` is a minimal Node HTTP server that serves static responses matching the CLI output format. Use it for UI development without a cluster:

```bash
make ui-dev        # starts mock API + Vite
# or
node ui/mock-api.cjs &
cd ui && npx vite
```
