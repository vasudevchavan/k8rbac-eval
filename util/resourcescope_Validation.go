package util

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

func GetNamespacedResources(dc discovery.DiscoveryInterface) ([]string, error) {
	apiGroupResources, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return nil, err
	}

	var namespacedResources []string

	for _, group := range apiGroupResources {
		for _, resources := range group.VersionedResources {
			for _, r := range resources {
				if strings.Contains(r.Name, "/") {
					continue
				}
				if r.Namespaced {
					namespacedResources = append(namespacedResources, r.Name)
				}
			}
		}
	}

	return namespacedResources, nil
}

// GetAllResources returns all Kubernetes resources (namespaced and cluster-scoped)
// func GetAllResources(discoveryClient discovery.DiscoveryInterface) ([]string, error) {
// 	apiGroupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var resources []string

// 	for _, group := range apiGroupResources {
// 		for _, resList := range group.VersionedResources {
// 			for _, r := range resList {
// 				resources = append(resources, r.Name)
// 			}
// 		}
// 	}

// 	return resources, nil
// }

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
			// skip subresources like pods/exec
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
