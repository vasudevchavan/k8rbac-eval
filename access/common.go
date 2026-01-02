package access

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vasudevchavan/k8s-get-access-level/util"
)

var (
	userNamespace string
	clusterScope  bool
	resource      string
)

func ValidateCommonFlags(cmd *cobra.Command, args []string) error {
	clientset, err := util.GetClientset()
	if err != nil {
		return err
	}

	// If a resource is specified, validate cluster vs namespace
	if resource != "" {
		resolved, err := util.ResolveResourceName(clientset.Discovery(), resource)
		if err != nil {
			return err
		}
		resource = resolved

		resolver, err := util.NewResourceScopeResolver(clientset.Discovery())
		if err != nil {
			return err
		}

		namespaced, err := resolver.IsNamespaced(resource)
		if err != nil {
			return err
		}

		nsFlag := cmd.Flags().Changed("namespace")
		csFlag := cmd.Flags().Changed("clusterscope")

		//  user passed --namespace for cluster-scoped resource
		if !namespaced && nsFlag {
			return fmt.Errorf("resource %q is cluster-scoped; --namespace is not allowed", resource)
		}

		//  user passed --clusterscope for namespaced resource
		if namespaced && csFlag && clusterScope {
			return fmt.Errorf("resource %q is namespaced; --clusterscope is not allowed", resource)
		}

		//  set clusterScope automatically for cluster resources
		if !namespaced {
			clusterScope = true
			userNamespace = ""
		}

		//  set default namespace for namespaced resources if not specified
		if namespaced && !nsFlag {
			userNamespace = "default"
		}
	}

	return nil
}

func RunAccessCheck(cmd *cobra.Command, args []string, isServiceAccount bool) error {
	username := args[0]
	displayUsername := username
	if isServiceAccount {
		// If namespace is not provided for SA, assume default or use the one set in flags
		saNamespace := userNamespace
		if saNamespace == "" {
			saNamespace = "default"
		}
		// Construct the full service account name: system:serviceaccount:<ns>:<name>
		// Only strictly necessary if the user provided a short name.
		// If the user provided "system:serviceaccount:...", we might double prefix if not careful.
		// For now, assuming simple name input as per typical CLI usage.
		if !strings.HasPrefix(username, "system:serviceaccount:") {
			username = fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, username)
		}
		displayUsername = username
	}

	clientset, err := util.GetClientset()
	if err != nil {
		return fmt.Errorf("error creating clientset: %v", err)
	}

	resolver, err := util.NewResourceScopeResolver(clientset.Discovery())
	if err != nil {
		return fmt.Errorf("error creating resolver: %v", err)
	}

	// Load rest config once
	restCfg, err := util.GetRestConfig()
	if err != nil {
		return fmt.Errorf("error loading rest config: %v", err)
	}

	// Create impersonated client once
	// We only include system:authenticated by default.
	// system:masters would give full access, defeating the purpose of this tool.
	groups := []string{"system:authenticated"}
	if isServiceAccount {
		groups = append(groups, "system:serviceaccounts")
		if userNamespace != "" {
			groups = append(groups, fmt.Sprintf("system:serviceaccounts:%s", userNamespace))
		}
	}

	impClient, err := NewImpersonatedClient(restCfg, username, groups)
	if err != nil {
		return fmt.Errorf("error creating impersonated client: %v", err)
	}

	var resourcesToCheck []string
	if resource != "" {
		resourcesToCheck = []string{resource}
	} else {
		resourcesToCheck, err = util.GetAllResources(clientset.Discovery())
		if err != nil {
			return fmt.Errorf("error fetching resources: %v", err)
		}
	}

	for _, res := range resourcesToCheck {
		// Skip subresources
		if strings.Contains(res, "/") {
			continue
		}

		namespaced, err := resolver.IsNamespaced(res)
		if err != nil {
			fmt.Printf("Skipping %s: %v\n", res, err)
			continue
		}

		// Respect --clusterscope
		if clusterScope && namespaced {
			continue
		}
		if !clusterScope && !namespaced {
			continue
		}

		ns := ""
		if namespaced {
			ns = userNamespace
			if isServiceAccount {
				fmt.Printf("\nInspecting access for service account %s on %s (namespace: %s)\n",
					displayUsername, res, ns)
			} else {
				fmt.Printf("\nInspecting access for user %s on %s (namespace: %s)\n",
					displayUsername, res, ns)
			}
		} else {
			if isServiceAccount {
				fmt.Printf("\nInspecting access for service account %s on cluster-scoped %s\n",
					displayUsername, res)
			} else {
				fmt.Printf("\nInspecting access for user %s on cluster-scoped %s\n",
					displayUsername, res)
			}
		}

		accessMap, err := GetUserAccessLevel(
			impClient,
			res,
			ns,
		)
		if err != nil {
			fmt.Printf("  Error checking access: %v\n", err)
			continue
		}

		for verb, allowed := range accessMap {
			fmt.Printf("  %-6s : %v\n", verb, allowed)
		}
	}
	return nil
}
