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

		// user passed --namespace for cluster-scoped resource
		if !namespaced && nsFlag {
			return opts, fmt.Errorf("resource %q is cluster-scoped; --namespace is not allowed", opts.Resource)
		}

		// user passed --clusterscope for namespaced resource
		if namespaced && csFlag && opts.ClusterScope {
			return opts, fmt.Errorf("resource %q is namespaced; --clusterscope is not allowed", opts.Resource)
		}

		// auto-set clusterScope for cluster-scoped resources
		if !namespaced {
			opts.ClusterScope = true
			opts.UserNamespace = ""
		}

		// default namespace for namespaced resources
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
		// Construct the full service account name: system:serviceaccount:<ns>:<name>
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

	var resourcesToCheck []string
	if opts.Resource != "" {
		resourcesToCheck = []string{opts.Resource}
	} else {
		resourcesToCheck, err = discovery.GetAllResources(clientset.Discovery())
		if err != nil {
			return fmt.Errorf("error fetching resources: %w", err)
		}
	}

	for _, res := range resourcesToCheck {
		// Skip subresources (e.g. pods/exec)
		if strings.Contains(res, "/") {
			continue
		}

		namespaced, err := resolver.IsNamespaced(res)
		if err != nil {
			slog.Warn("skipping resource", "resource", res, "error", err)
			continue
		}

		// Respect --clusterscope: skip resources that don't match the requested scope.
		if opts.ClusterScope && namespaced {
			continue
		}
		if !opts.ClusterScope && !namespaced {
			continue
		}

		ns := ""
		if namespaced {
			ns = opts.UserNamespace
			slog.Info("inspecting access", "subject", displayUsername, "resource", res, "namespace", ns)
		} else {
			slog.Info("inspecting access", "subject", displayUsername, "resource", res, "scope", "cluster")
		}

		checker := access.NewKubeChecker(impClient)
		accessMap, err := checker.Check(cmd.Context(), res, ns)
		if err != nil {
			slog.Error("error checking access", "resource", res, "error", err)
			continue
		}

		// Sort verbs for deterministic output.
		verbs := make([]string, 0, len(accessMap))
		for v := range accessMap {
			verbs = append(verbs, v)
		}
		sort.Strings(verbs)

		fmt.Printf("resource: %s\n", res)
		for _, verb := range verbs {
			fmt.Printf("  %-18s : %v\n", verb, accessMap[verb])
		}
	}
	return nil
}
