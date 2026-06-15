// cmd/server/main.go
// HTTP API server that wraps the kubeaccess CLI for the Carbon/React UI.
//
// Endpoints:
//   POST /api/check    – run `kubeaccess show {user|sa} <name> [flags]`
//   POST /api/generate – run `kubeaccess generate {user|sa} <name> [flags]`
//   GET  /api/health   – liveness probe
//
// The server looks for the kubeaccess binary in $PATH, then alongside itself,
// then in the project's bin/ directory.

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ────────────────────────────────────────────────────────────────────────────

type CheckRequest struct {
	SubjectType  string `json:"subjectType"`  // "user" | "sa"
	Name         string `json:"name"`         // username or service account name
	Namespace    string `json:"namespace"`    // optional, default "default"
	Resource     string `json:"resource"`     // optional; empty = all resources
	ClusterScope bool   `json:"clusterScope"` // -c flag
	Kubeconfig   string `json:"kubeconfig"`   // optional path to kubeconfig file
}

type GenerateRequest struct {
	SubjectType  string   `json:"subjectType"` // "user" | "sa"
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Resource     string   `json:"resource"`
	Verbs        []string `json:"verbs"` // e.g. ["get","list","watch"]
	ClusterScope bool     `json:"clusterScope"`
	Kubeconfig   string   `json:"kubeconfig"` // optional path to kubeconfig file
}

// KubeconfigListResponse lists available kubeconfig files
type KubeconfigListResponse struct {
	Files   []string `json:"files"`
	Default string   `json:"default"`
}

type APIResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// Binary resolution
// ────────────────────────────────────────────────────────────────────────────

func findBinary() (string, error) {
	// 1. $KUBEACCESS_BIN env override
	if env := os.Getenv("KUBEACCESS_BIN"); env != "" {
		return env, nil
	}

	// 2. $PATH
	if p, err := exec.LookPath("kubeaccess"); err == nil {
		return p, nil
	}

	// 3. Alongside this server binary
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

	// 4. Project bin/ relative to CWD
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
// Handlers
// ────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleKubeconfigs lists kubeconfig files found in ~/.kube/ and any paths
// in the KUBECONFIG env var.
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

	// KUBECONFIG env (colon-separated on Unix, semicolon on Windows)
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	for _, p := range strings.Split(os.Getenv("KUBECONFIG"), sep) {
		addFile(strings.TrimSpace(p))
	}

	// ~/.kube/config (default)
	home, _ := os.UserHomeDir()
	defaultCfg := filepath.Join(home, ".kube", "config")
	addFile(defaultCfg)

	// Any other *.yaml / *.yml files in ~/.kube/
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

		// Validate subject existence before running the access check.
		// Kubernetes impersonation works for any subject (even non-existent ones),
		// so without this check we'd silently return all-denied results for a typo'd
		// SA or a user that has never been granted any RBAC.
		if subjectType == "sa" {
			// Service accounts are namespaced resources — check directly.
			ns := req.Namespace
			if ns == "" {
				ns = "default"
			}
			kubectlArgs := []string{"get", "sa", req.Name, "-n", ns}
			kubectlCmd := exec.Command("kubectl", kubectlArgs...)
			kubectlCmd.Env = os.Environ()
			if req.Kubeconfig != "" {
				kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+req.Kubeconfig)
			}
			if out, err := kubectlCmd.CombinedOutput(); err != nil {
				msg := strings.TrimSpace(string(out))
				if msg == "" {
					msg = err.Error()
				}
				writeJSON(w, http.StatusBadRequest, APIResponse{
					Error: fmt.Sprintf("service account %q not found in namespace %q: %s", req.Name, ns, msg),
				})
				return
			}
		} else {
			// Users are not stored as Kubernetes objects, but we can check whether
			// the username appears as a subject in any RoleBinding or ClusterRoleBinding.
			// If not, the user has no RBAC configured and the check result would be
			// misleading (all-denied looks the same as "user doesn't exist").
			jsonArgs := []string{
				"get", "rolebindings,clusterrolebindings",
				"--all-namespaces",
				"-o", "jsonpath={.items[*].subjects[*].name}",
			}
			rbCmd := exec.Command("kubectl", jsonArgs...)
			rbCmd.Env = os.Environ()
			if req.Kubeconfig != "" {
				rbCmd.Env = append(rbCmd.Env, "KUBECONFIG="+req.Kubeconfig)
			}
			if out, err := rbCmd.CombinedOutput(); err == nil {
				subjects := strings.Fields(string(out))
				found := false
				for _, s := range subjects {
					if s == req.Name {
						found = true
						break
					}
				}
				if !found {
					writeJSON(w, http.StatusBadRequest, APIResponse{
						Error: fmt.Sprintf("user %q has no RoleBinding or ClusterRoleBinding in this cluster — they may not exist or have never been granted access", req.Name),
					})
					return
				}
			}
			// If kubectl itself fails (permissions, connectivity), skip the check and
			// let kubeaccess proceed — it will surface any real errors.
		}

		args := []string{"show", subjectType, req.Name}

		if req.Resource != "" {
			args = append(args, "--resource", req.Resource)
		}
		if req.ClusterScope {
			args = append(args, "--clusterscope")
		} else if req.Namespace != "" {
			args = append(args, "--namespace", req.Namespace)
		}

		slog.Info("running check", "args", args)
		checkCmd := exec.Command(binary, args...)
		// Propagate env; override KUBECONFIG if the caller specified one.
		checkCmd.Env = os.Environ()
		if req.Kubeconfig != "" {
			checkCmd.Env = append(checkCmd.Env, "KUBECONFIG="+req.Kubeconfig)
		}
		out, err := checkCmd.CombinedOutput()
		resp := APIResponse{Output: string(out)}
		if err != nil {
			// Prefer the CLI's own output over the bare "exit status N" string —
			// it contains the actual error message (e.g. permission denied, API unreachable).
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

		slog.Info("running generate", "args", args)
		genCmd := exec.Command(binary, args...)
		genCmd.Env = os.Environ()
		if req.Kubeconfig != "" {
			genCmd.Env = append(genCmd.Env, "KUBECONFIG="+req.Kubeconfig)
		}
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
		writeJSON(w, http.StatusOK, resp)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// CORS middleware (for Vite dev proxy passthrough)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealth)
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
