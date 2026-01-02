package access

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewImpersonatedClient(
	restConfig *rest.Config,
	username string,
	groups []string,
) (kubernetes.Interface, error) {

	cfg := rest.CopyConfig(restConfig)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: username,
		Groups:   groups,
	}

	return kubernetes.NewForConfig(cfg)
}

func GetUserAccessLevel(
	client kubernetes.Interface, // impersonated client
	resource string,
	namespace string, // "" for cluster-scoped
) (map[string]bool, error) {

	verbs := []string{
		"get", "list", "watch",
		"create", "update", "patch", "delete",
	}

	access := make(map[string]bool)

	for _, verb := range verbs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Verb:      verb,
					Resource:  resource,
					Namespace: namespace,
				},
			},
		}

		resp, err := client.AuthorizationV1().
			SelfSubjectAccessReviews().
			Create(context.Background(), sar, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}

		access[verb] = resp.Status.Allowed
	}

	return access, nil
}
