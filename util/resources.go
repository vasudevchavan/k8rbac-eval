package util

import (
	"fmt"
	"strings"

	"k8s.io/client-go/discovery"
)

// ResolveResourceName finds the plural resource name for a given alias (singular, shortName, or plural).
func ResolveResourceName(discoveryClient discovery.DiscoveryInterface, input string) (string, error) {
	_, resources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// If we can't discover, we just return the input and hope for the best
		// But in a real CLI we might want to warn.
		// For now, return error if completely failed, or partial results?
		// ServerGroupsAndResources returns partial results and error.
		if len(resources) == 0 {
			return "", err
		}
	}

	lowerInput := strings.ToLower(input)

	for _, list := range resources {
		for _, r := range list.APIResources {
			if strings.ToLower(r.Name) == lowerInput {
				return r.Name, nil
			}
			if strings.ToLower(r.SingularName) == lowerInput {
				return r.Name, nil
			}
			for _, short := range r.ShortNames {
				if strings.ToLower(short) == lowerInput {
					return r.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("resource %q not found", input)
}
