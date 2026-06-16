// cmd/server/main.go
// HTTP API server that wraps the kubeaccess CLI for the Carbon/React UI.
//
// Endpoints:
//   GET  /api/platform    – detected cluster platform + flags
//   POST /api/check       – run `kubeaccess show {user|sa} <name> [flags]`
//   POST /api/generate    – run `kubeaccess generate {user|sa} <name> [flags]`
//   GET  /api/kubeconfigs – list available kubeconfig files
//   GET  /api/health      – liveness probe

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vasudevchavan/k8s-get-access-level/pkg/client"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/platform"
)

// ────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ────────────────────────────────────────────────────────────────────────────

type CheckRequest struct {
	SubjectType  string `json:"subjectType"` // "user" | "sa"
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Resource     string `json:"resource"`
	ClusterScope bool   `json:"clusterScope"`
	Kubeconfig   string `json:"kubeconfig"`
}

type GenerateRequest struct {
	SubjectType  string   `json:"subjectType"` // "user" | "sa"
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Resource     string   `json:"resource"`
	Verbs        []string `json:"verbs"`
	ClusterScope bool     `json:"clusterScope"`
	Kubeconfig   string   `json:"kubeconfig"`
}

type KubeconfigListResponse struct {
	Files   []string `json:"files"`
	Default string   `json:"default"`
}

type APIResponse struct {
	Output   string   `json:"output"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"` // non-fatal advisories (IRSA, Workload Identity, etc.)
}

// ────────────────────────────────────────────────────────────────────────────
// Platform cache — detect once per unique kubeconfig path
// ────────────────────────────────────────────────────────────────────────────

type platformCache struct {
	mu    sync.RWMutex
	cache map[string]platform.Info
}

var pCache = &platformCache{cache: make(map[string]platform.Info)}

func (c *platformCache) get(key string) (platform.Info, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cache[key]
	return v, ok
}

func (c *platformCache) set(key string, info platform.Info) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = info
}

// detectPlatform returns (cached) platform info for a kubeconfig path.
func detectPlatform(kubeconfig string) platform.Info {
	key := kubeconfig
	if key == "" {
		key = "__default__"
	}
	if info, ok := pCache.get(key); ok {
		return info
	}

	if kubeconfig != "" {
		_ = os.Setenv("KUBECONFIG", kubeconfig)
	}
	k8sClient, err := client.GetClientset()
	if err != nil {
		slog.Warn("platform detection: could not build client", "error", err)
		info := platform.Info{Platform: platform.TypeVanilla, DisplayName: "Kubernetes"}
		pCache.set(key, info)
		return info
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info := platform.Detect(ctx, k8sClient)
	slog.Info("detected cluster platform",
		"platform", info.Platform,
		"displayName", info.DisplayName,
		"azureRBACMode", info.AzureRBACMode,
	)
	pCache.set(key, info)
	return info
}

// ────────────────────────────────────────────────────────────────────────────
// Binary resolution
// ────────────────────────────────────────────────────────────────────────────

func findBinary() (string, error) {
	if env := os.Getenv("KUBEACCESS_BIN"); env != "" {
		return env, nil
	}
	if p, err := exec.LookPath("kubeaccess"); err == nil {
		return p, nil
	}
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "kubeaccess")
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	cwd, _ := os.Getwd()
	for _, dir := range []string{
		filepath.Join(cwd, "bin"),
		filepath.Join(cwd, "..", "bin"),
		filepath.Join(cwd, "..", "..", "bin"),
	} {
		arch := fmt.Sprintf("kubeaccess-%s-%s", runtime.GOOS, runtime.GOARCH)
		for _, name := range []string{"kubeaccess", arch} {
			p := filepath.Join(dir, name)
			if runtime.GOOS == "windows" {
				p += ".exe"
			}
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("kubeaccess binary not found; set KUBEACCESS_BIN or add it to $PATH")
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// buildEnv returns os.Environ() with KUBECONFIG overridden when kubeconfig != "".
func buildEnv(kubeconfig string) []string {
	env := os.Environ()
	if kubeconfig != "" {
		env = append(env, "KUBECONFIG="+kubeconfig)
	}
	return env
}

// roleBindingUserExists returns true when username appears as a subject in any
// RoleBinding or ClusterRoleBinding across all namespaces.
func roleBindingUserExists(kubeconfig, username string) bool {
	cmd := exec.Command("kubectl",
		"get", "rolebindings,clusterrolebindings",
		"--all-namespaces",
		"-o", "jsonpath={.items[*].subjects[*].name}",
	)
	cmd.Env = buildEnv(kubeconfig)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	for _, s := range strings.Fields(string(out)) {
		if s == username {
			return true
		}
	}
	return false
}

// validateUser checks whether a user exists on the cluster.
// Returns (found, skip): skip=true means we cannot determine existence
// and the request should proceed without blocking.
func validateUser(ctx context.Context, pInfo platform.Info, kubeconfig, username string) (found, skip bool) {
	if kubeconfig != "" {
		_ = os.Setenv("KUBECONFIG", kubeconfig)
	}
	typedClient, err := client.GetClientset()
	if err != nil {
		return false, true // can't validate, let through
	}

	switch pInfo.Platform {

	case platform.TypeOpenShift:
		// Check real OpenShift User object first
		if exists, _ := platform.OpenShiftUserExists(ctx, typedClient, username); exists {
			return true, false
		}
		// Fall back to RoleBinding scan for system users / service users
		return roleBindingUserExists(kubeconfig, username), false

	case platform.TypeEKS:
		// Check aws-auth ConfigMap; skip if absent (EKS Access Entries clusters)
		if exists, err := platform.EKSUserExists(ctx, typedClient, username); err == nil {
			if exists {
				return true, false
			}
		} else {
			return false, true // can't determine
		}
		// Also check RoleBindings for users with direct k8s bindings
		return roleBindingUserExists(kubeconfig, username), false

	case platform.TypeAKS:
		if pInfo.AzureRBACMode {
			// Azure RBAC mode: users never appear in Kubernetes RoleBindings
			return false, true
		}
		return roleBindingUserExists(kubeconfig, username), false

	default:
		return roleBindingUserExists(kubeconfig, username), false
	}
}

func buildUserNotFoundMsg(pInfo platform.Info, username string) string {
	switch pInfo.Platform {
	case platform.TypeOpenShift:
		return fmt.Sprintf(
			"user %q not found on this OpenShift cluster — "+
				"no User object exists and no RoleBinding references this username", username)
	case platform.TypeEKS:
		return fmt.Sprintf(
			"user %q not found — "+
				"not in aws-auth ConfigMap and has no RoleBinding on this EKS cluster", username)
	default:
		return fmt.Sprintf(
			"user %q has no RoleBinding or ClusterRoleBinding in this cluster — "+
				"they may not exist or have never been granted access", username)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Handlers
// ────────────────────────────────────────────────────────────────────────────

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePlatform detects and returns the cluster platform + flags.
// Query param: ?kubeconfig=<path>  (optional)
func handlePlatform(w http.ResponseWriter, r *http.Request) {
	kubeconfig := r.URL.Query().Get("kubeconfig")
	info := detectPlatform(kubeconfig)
	writeJSON(w, http.StatusOK, info)
}

func handleKubeconfigs(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	var files []string

	addFile := func(p string) {
		if p == "" || seen[p] {
			return
		}
		if _, err := os.Stat(p); err == nil {
			seen[p] = true
			files = append(files, p)
		}
	}

	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	for _, p := range strings.Split(os.Getenv("KUBECONFIG"), sep) {
		addFile(strings.TrimSpace(p))
	}

	home, _ := os.UserHomeDir()
	defaultCfg := filepath.Join(home, ".kube", "config")
	addFile(defaultCfg)

	kubeDir := filepath.Join(home, ".kube")
	if entries, err := os.ReadDir(kubeDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || name == "config" {
				addFile(filepath.Join(kubeDir, name))
			}
		}
	}

	resp := KubeconfigListResponse{Files: files, Default: defaultCfg}
	if len(files) > 0 && !seen[defaultCfg] {
		resp.Default = files[0]
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleCheck(binary string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Error: "method not allowed"})
			return
		}

		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid JSON: " + err.Error()})
			return
		}
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "name is required"})
			return
		}
		subjectType := strings.ToLower(req.SubjectType)
		if subjectType != "user" && subjectType != "sa" {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "subjectType must be 'user' or 'sa'"})
			return
		}

		ctx := r.Context()
		pInfo := detectPlatform(req.Kubeconfig)

		var warnings []string

		if subjectType == "sa" {
			ns := req.Namespace
			if ns == "" {
				ns = "default"
			}

			// Validate SA existence via kubectl
			saCmd := exec.Command("kubectl", "get", "sa", req.Name, "-n", ns)
			saCmd.Env = buildEnv(req.Kubeconfig)
			if out, err := saCmd.CombinedOutput(); err != nil {
				msg := strings.TrimSpace(string(out))
				if msg == "" {
					msg = err.Error()
				}
				writeJSON(w, http.StatusBadRequest, APIResponse{
					Error: fmt.Sprintf("service account %q not found in namespace %q: %s", req.Name, ns, msg),
				})
				return
			}

			// Cloud identity annotation warnings (IRSA / Workload Identity)
			if kubeconfig := req.Kubeconfig; kubeconfig != "" {
				_ = os.Setenv("KUBECONFIG", kubeconfig)
			}
			if k8sClient, err := client.GetClientset(); err == nil {
				warnings = platform.SACloudWarnings(ctx, k8sClient, req.Name, ns)
			}

		} else {
			// Platform-aware user validation
			found, skip := validateUser(ctx, pInfo, req.Kubeconfig, req.Name)
			if !skip && !found {
				writeJSON(w, http.StatusBadRequest, APIResponse{
					Error: buildUserNotFoundMsg(pInfo, req.Name),
				})
				return
			}
			if pInfo.AzureRBACMode {
				warnings = append(warnings, "This AKS cluster uses Azure RBAC. "+
					"Access shown here reflects Kubernetes RBAC only. "+
					"Azure role assignments are not visible via Kubernetes APIs.")
			}
		}

		// Build and run kubeaccess check
		args := []string{"show", subjectType, req.Name}
		if req.Resource != "" {
			args = append(args, "--resource", req.Resource)
		}
		if req.ClusterScope {
			args = append(args, "--clusterscope")
		} else if req.Namespace != "" {
			args = append(args, "--namespace", req.Namespace)
		}

		slog.Info("running check", "args", args, "platform", pInfo.Platform)
		checkCmd := exec.Command(binary, args...)
		checkCmd.Env = buildEnv(req.Kubeconfig)
		out, err := checkCmd.CombinedOutput()
		resp := APIResponse{Output: string(out), Warnings: warnings}
		if err != nil {
			if msg := strings.TrimSpace(string(out)); msg != "" {
				resp.Error = msg
			} else {
				resp.Error = err.Error()
			}
			slog.Error("kubeaccess check failed", "error", err, "output", string(out))
			writeJSON(w, http.StatusInternalServerError, resp)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGenerate(binary string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, APIResponse{Error: "method not allowed"})
			return
		}

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid JSON: " + err.Error()})
			return
		}
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "name is required"})
			return
		}
		if req.Resource == "" {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "resource is required"})
			return
		}
		subjectType := strings.ToLower(req.SubjectType)
		if subjectType != "user" && subjectType != "sa" {
			writeJSON(w, http.StatusBadRequest, APIResponse{Error: "subjectType must be 'user' or 'sa'"})
			return
		}

		args := []string{"generate", subjectType, req.Name, "--resource", req.Resource}
		verbs := req.Verbs
		if len(verbs) == 0 {
			verbs = []string{"get", "list", "watch"}
		}
		for _, v := range verbs {
			args = append(args, "--verb", v)
		}
		if req.ClusterScope {
			args = append(args, "--clusterscope")
		} else if req.Namespace != "" {
			args = append(args, "--namespace", req.Namespace)
		}

		pInfo := detectPlatform(req.Kubeconfig)
		slog.Info("running generate", "args", args, "platform", pInfo.Platform)

		genCmd := exec.Command(binary, args...)
		genCmd.Env = buildEnv(req.Kubeconfig)
		out, err := genCmd.CombinedOutput()
		resp := APIResponse{Output: string(out)}
		if err != nil {
			if msg := strings.TrimSpace(string(out)); msg != "" {
				resp.Error = msg
			} else {
				resp.Error = err.Error()
			}
			slog.Error("kubeaccess generate failed", "error", err, "output", string(out))
			writeJSON(w, http.StatusInternalServerError, resp)
			return
		}

		// On OpenShift, surface an advisory about User subject kinds
		if pInfo.Platform == platform.TypeOpenShift && subjectType == "user" {
			resp.Warnings = []string{
				"On OpenShift, subjects of Kind 'User' refer to OpenShift User objects " +
					"(via user.openshift.io). The generated manifest uses standard " +
					"rbac.authorization.k8s.io subjects which work correctly on OpenShift.",
			}
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// CORS middleware
// ────────────────────────────────────────────────────────────────────────────

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ────────────────────────────────────────────────────────────────────────────
// main
// ────────────────────────────────────────────────────────────────────────────

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	binary, err := findBinary()
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
	slog.Info("using kubeaccess binary", "path", binary)

	// Eagerly detect platform at startup using the default kubeconfig
	go func() { detectPlatform("") }()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/platform", handlePlatform)
	mux.HandleFunc("/api/kubeconfigs", handleKubeconfigs)
	mux.HandleFunc("/api/check", handleCheck(binary))
	mux.HandleFunc("/api/generate", handleGenerate(binary))

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	slog.Info("KubeAccess API server starting", "addr", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
