package discovery

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
)

type ResourceScopeResolver struct {
	mapper meta.RESTMapper
}

func NewResourceScopeResolver(
	dc discovery.DiscoveryInterface,
) (*ResourceScopeResolver, error) {
	resources, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, err
	}

	return &ResourceScopeResolver{
		mapper: restmapper.NewDiscoveryRESTMapper(resources),
	}, nil
}

func (r *ResourceScopeResolver) IsNamespaced(resource string) (bool, error) {
	gvr, err := r.ResourceFor(resource)
	if err != nil {
		return false, err
	}

	gvk, err := r.mapper.KindFor(gvr)
	if err != nil {
		return false, err
	}

	mapping, err := r.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return false, err
	}

	return mapping.Scope.Name() == meta.RESTScopeNameNamespace, nil
}

func (r *ResourceScopeResolver) ResourceFor(resource string) (schema.GroupVersionResource, error) {
	return r.mapper.ResourceFor(schema.GroupVersionResource{
		Resource: resource,
	})
}

// GetAllResources returns all unique top-level Kubernetes resources (no subresources)
// using the server's preferred API version for each resource group.
func GetAllResources(discoveryClient discovery.DiscoveryInterface) ([]string, error) {
	resourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var resources []string

	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range rl.APIResources {
			// Skip subresources (e.g. pods/exec, pods/log)
			if strings.Contains(r.Name, "/") {
				continue
			}

			key := gv.Group + "/" + r.Name
			if _, exists := seen[key]; exists {
				continue
			}

			seen[key] = struct{}{}
			resources = append(resources, r.Name)
		}
	}

	return resources, nil
}
