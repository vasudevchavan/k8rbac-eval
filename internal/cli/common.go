package cli

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/access"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/client"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/discovery"
)

func init() {
	// Direct CLI log output to stderr so it doesn't mix with piped/parsed results.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

// discCache is a process-wide discovery cache shared across CLI invocations in
// the same process (useful when the binary is embedded in the server).
var discCache = discovery.NewResourceCache(discovery.DefaultCacheTTL)

// orderedVerbs defines the fixed print order so output is always deterministic.
var orderedVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}

// AccessOptions holds validated flag values shared across show and generate commands.
type AccessOptions struct {
	UserNamespace string
	ClusterScope  bool
	Resource      string
	Kubeconfig    string
}

func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("namespace", "n", "default", "Namespace scope")
	cmd.Flags().String("resource", "", "Kubernetes resource (e.g. pods, deployments)")
	cmd.Flags().BoolP("clusterscope", "c", false, "Check cluster-wide access")
	cmd.Flags().String("kubeconfig", "", "Path to kubeconfig file (defaults to KUBECONFIG env or ~/.kube/config)")
}

func ValidateCommonFlags(cmd *cobra.Command, args []string) (AccessOptions, error) {
	opts := AccessOptions{}
	var err error

	opts.UserNamespace, err = cmd.Flags().GetString("namespace")
	if err != nil {
		return opts, err
	}
	opts.Resource, err = cmd.Flags().GetString("resource")
	if err != nil {
		return opts, err
	}
	opts.ClusterScope, err = cmd.Flags().GetBool("clusterscope")
	if err != nil {
		return opts, err
	}
	opts.Kubeconfig, err = cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return opts, err
	}

	// Build client using the explicit kubeconfig (or default resolution).
	clientset, err := client.GetClientsetWithKubeconfig(opts.Kubeconfig)
	if err != nil {
		return opts, err
	}

	// If a resource is specified, validate its scope and resolve aliases.
	if opts.Resource != "" {
		resolved, err := discovery.ResolveResourceName(clientset.Discovery(), opts.Resource)
		if err != nil {
			return opts, err
		}
		opts.Resource = resolved

		resolver, err := discovery.NewResourceScopeResolver(clientset.Discovery())
		if err != nil {
			return opts, err
		}

		namespaced, err := resolver.IsNamespaced(opts.Resource)
		if err != nil {
			return opts, err
		}

		nsFlag := cmd.Flags().Changed("namespace")
		csFlag := cmd.Flags().Changed("clusterscope")

		if !namespaced && nsFlag {
			return opts, fmt.Errorf("resource %q is cluster-scoped; --namespace is not allowed", opts.Resource)
		}
		if namespaced && csFlag && opts.ClusterScope {
			return opts, fmt.Errorf("resource %q is namespaced; --clusterscope is not allowed", opts.Resource)
		}
		if !namespaced {
			opts.ClusterScope = true
			opts.UserNamespace = ""
		}
		if namespaced && !nsFlag {
			opts.UserNamespace = "default"
		}
	}

	return opts, nil
}

func RunAccessCheck(cmd *cobra.Command, args []string, isServiceAccount bool, opts AccessOptions) error {
	username := args[0]
	displayUsername := username

	if isServiceAccount {
		saNamespace := opts.UserNamespace
		if saNamespace == "" {
			saNamespace = "default"
		}
		if !strings.HasPrefix(username, "system:serviceaccount:") {
			username = fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, username)
		}
		displayUsername = username
	}

	// Create client once; reuse for discovery and impersonation.
	clientset, err := client.GetClientsetWithKubeconfig(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("error creating clientset: %w", err)
	}

	resolver, err := discovery.NewResourceScopeResolver(clientset.Discovery())
	if err != nil {
		return fmt.Errorf("error creating scope resolver: %w", err)
	}

	restCfg, err := client.GetRestConfigWithKubeconfig(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("error loading rest config: %w", err)
	}

	groups := []string{"system:authenticated"}
	if isServiceAccount {
		groups = append(groups, "system:serviceaccounts")
		if opts.UserNamespace != "" {
			groups = append(groups, fmt.Sprintf("system:serviceaccounts:%s", opts.UserNamespace))
		}
	}

	impClient, err := access.NewImpersonatedClient(restCfg, username, groups)
	if err != nil {
		return fmt.Errorf("error creating impersonated client: %w", err)
	}

	checker := access.NewKubeChecker(impClient)

	// ── Single resource ───────────────────────────────────────────────────────
	// Check uses parallel verb calls internally (~7× faster than sequential).
	if opts.Resource != "" {
		ns := opts.UserNamespace
		if opts.ClusterScope {
			ns = ""
		}
		slog.Info("inspecting access", "subject", displayUsername, "resource", opts.Resource)
		accessMap, err := checker.Check(cmd.Context(), opts.Resource, ns)
		if err != nil {
			return fmt.Errorf("access check failed: %w", err)
		}
		printAccess(opts.Resource, accessMap)
		return nil
	}

	// ── All resources ─────────────────────────────────────────────────────────
	// Use the discovery cache to avoid re-hitting the API on repeat runs.
	allResources, err := discCache.Get(opts.Kubeconfig, clientset.Discovery())
	if err != nil {
		return fmt.Errorf("error fetching resources: %w", err)
	}

	// Filter to only the resources that match the requested scope.
	var filtered []string
	for _, res := range allResources {
		if strings.Contains(res, "/") {
			continue // skip subresources
		}
		namespaced, err := resolver.IsNamespaced(res)
		if err != nil {
			slog.Warn("skipping resource", "resource", res, "error", err)
			continue
		}
		if opts.ClusterScope && namespaced {
			continue
		}
		if !opts.ClusterScope && !namespaced {
			continue
		}
		filtered = append(filtered, res)
	}

	var allAccess map[string]map[string]bool

	if !opts.ClusterScope {
		// ── Fast path: SelfSubjectRulesReview (1 API call for everything) ──
		slog.Info("using SelfSubjectRulesReview", "subject", displayUsername, "namespace", opts.UserNamespace, "resources", len(filtered))
		result, incomplete, err := checker.CheckAllNamespaced(cmd.Context(), filtered, opts.UserNamespace)
		switch {
		case err != nil:
			slog.Warn("SelfSubjectRulesReview failed, falling back to individual checks", "error", err)
		case incomplete:
			slog.Warn("SelfSubjectRulesReview incomplete (webhook authorizer?), falling back to individual checks")
		default:
			allAccess = result
		}
	}

	if allAccess == nil {
		// ── Fallback: worker pool — workerCount resources checked concurrently,
		//    each using parallel verb checks. Good for cluster-scope and
		//    webhook-authorizer clusters where RulesReview is incomplete.
		ns := opts.UserNamespace
		if opts.ClusterScope {
			ns = ""
		}
		slog.Info("checking access via worker pool", "subject", displayUsername, "resources", len(filtered))
		allAccess, err = checker.CheckAllResources(cmd.Context(), filtered, ns)
		if err != nil {
			// Partial results are still printed; log the error and continue.
			slog.Warn("some resources could not be checked", "error", err)
		}
	}

	// Print in stable alphabetical order.
	sort.Strings(filtered)
	for _, res := range filtered {
		verbMap, ok := allAccess[res]
		if !ok {
			continue
		}
		printAccess(res, verbMap)
	}

	return nil
}

// printAccess prints the verb→allowed map for one resource in a fixed verb order.
func printAccess(resource string, accessMap map[string]bool) {
	fmt.Printf("resource: %s\n", resource)
	for _, verb := range orderedVerbs {
		fmt.Printf("  %-18s : %v\n", verb, accessMap[verb])
	}
}
