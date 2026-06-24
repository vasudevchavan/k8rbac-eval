package access

import (
	"context"
	"fmt"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// targetVerbs is the fixed set of verbs evaluated for every resource.
var targetVerbs = []string{
	"get", "list", "watch", "create", "update", "patch", "delete",
}

// workerCount caps how many resources are checked concurrently in CheckAllResources.
const workerCount = 10

// Checker checks access level for a resource.
type Checker interface {
	Check(ctx context.Context, resource, namespace string) (map[string]bool, error)
}

// KubeChecker implements Checker using a Kubernetes client.
type KubeChecker struct {
	Client kubernetes.Interface
}

// NewKubeChecker creates a new KubeChecker.
func NewKubeChecker(client kubernetes.Interface) *KubeChecker {
	return &KubeChecker{Client: client}
}

// NewImpersonatedClient returns a client that impersonates the given user/groups.
func NewImpersonatedClient(restConfig *rest.Config, username string, groups []string) (kubernetes.Interface, error) {
	cfg := rest.CopyConfig(restConfig)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: username,
		Groups:   groups,
	}
	return kubernetes.NewForConfig(cfg)
}

// Check checks all targetVerbs for a single resource concurrently.
// All 7 SelfSubjectAccessReview calls run in parallel over the same HTTP/2 connection,
// making this ~7× faster than the previous sequential implementation.
func (k *KubeChecker) Check(ctx context.Context, resource, namespace string) (map[string]bool, error) {
	type result struct {
		verb    string
		allowed bool
		err     error
	}

	ch := make(chan result, len(targetVerbs))

	for _, verb := range targetVerbs {
		go func(v string) {
			sar := &authorizationv1.SelfSubjectAccessReview{
				Spec: authorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Verb:      v,
						Resource:  resource,
						Namespace: namespace,
					},
				},
			}
			resp, err := k.Client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
			if err != nil {
				ch <- result{verb: v, err: err}
				return
			}
			ch <- result{verb: v, allowed: resp.Status.Allowed}
		}(verb)
	}

	access := make(map[string]bool, len(targetVerbs))
	for range targetVerbs {
		r := <-ch
		if r.err != nil {
			return nil, fmt.Errorf("verb %s: %w", r.verb, r.err)
		}
		access[r.verb] = r.allowed
	}
	return access, nil
}

// CheckAllNamespaced uses SelfSubjectRulesReview to fetch all of the impersonated
// user's rules in a namespace with a single API call, then computes the access
// matrix for the given resource list locally — no per-resource API calls needed.
//
// Returns (accessMap, incomplete, error).
//   - incomplete=true: the API could not enumerate all rules (e.g. webhook
//     authorizer); caller should fall back to individual Check calls.
//   - accessMap keys are the resources passed in; denied verbs are explicitly false.
func (k *KubeChecker) CheckAllNamespaced(ctx context.Context, resources []string, namespace string) (map[string]map[string]bool, bool, error) {
	review := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{Namespace: namespace},
	}
	resp, err := k.Client.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, false, err
	}
	if resp.Status.Incomplete {
		return nil, true, nil
	}

	// Build a verb set per resource from the returned rules.
	// Wildcards: resource="*" applies to every resource; verb="*" grants every verb.
	allowed := make(map[string]map[string]struct{}) // resource → allowed verb set

	for _, rule := range resp.Status.ResourceRules {
		for _, res := range rule.Resources {
			// Strip subresource suffix (e.g. "pods/log" → "pods")
			base := strings.SplitN(res, "/", 2)[0]
			if _, ok := allowed[base]; !ok {
				allowed[base] = make(map[string]struct{})
			}
			for _, verb := range rule.Verbs {
				if verb == "*" {
					for _, tv := range targetVerbs {
						allowed[base][tv] = struct{}{}
					}
				} else {
					allowed[base][verb] = struct{}{}
				}
			}
		}
	}

	wildcardVerbs := allowed["*"] // rules with resource="*" apply to everything

	// Build the final access matrix for each requested resource.
	// Resources absent from all rules are all-denied (explicit false).
	accessMap := make(map[string]map[string]bool, len(resources))
	for _, res := range resources {
		verbMap := make(map[string]bool, len(targetVerbs))
		for _, verb := range targetVerbs {
			_, fromWildcard := wildcardVerbs[verb]
			_, fromDirect := allowed[res][verb]
			verbMap[verb] = fromWildcard || fromDirect
		}
		accessMap[res] = verbMap
	}

	return accessMap, false, nil
}

// CheckAllResources checks multiple resources concurrently using a worker pool
// (capped at workerCount goroutines). Each resource uses Check internally,
// which already parallelises the 7 verb calls.
//
// Errors for individual resources are logged by the caller; this function
// returns the first error encountered alongside any partial results.
func (k *KubeChecker) CheckAllResources(ctx context.Context, resources []string, namespace string) (map[string]map[string]bool, error) {
	type item struct {
		resource string
		access   map[string]bool
		err      error
	}

	sem := make(chan struct{}, workerCount)
	ch := make(chan item, len(resources))

	for _, res := range resources {
		sem <- struct{}{}
		go func(r string) {
			defer func() { <-sem }()
			accessMap, err := k.Check(ctx, r, namespace)
			ch <- item{resource: r, access: accessMap, err: err}
		}(res)
	}

	result := make(map[string]map[string]bool, len(resources))
	var firstErr error
	for range resources {
		it := <-ch
		if it.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("resource %s: %w", it.resource, it.err)
			}
			continue
		}
		result[it.resource] = it.access
	}
	return result, firstErr
}
