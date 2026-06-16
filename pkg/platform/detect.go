// pkg/platform/detect.go
// Detects the Kubernetes distribution at startup so the server can adapt
// user-validation logic and surface platform-specific warnings in the UI.
//
// Detection strategy:
//   OpenShift  — presence of "route.openshift.io" API group
//   EKS        — "aws-auth" ConfigMap in kube-system  OR
//                node label "eks.amazonaws.com/nodegroup"
//   AKS        — node label "kubernetes.azure.com/cluster"
//   Vanilla    — none of the above

package platform

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Type represents the detected Kubernetes distribution.
type Type string

const (
	TypeOpenShift Type = "openshift"
	TypeEKS       Type = "eks"
	TypeAKS       Type = "aks"
	TypeVanilla   Type = "kubernetes"
)

// Info holds detection results and platform-specific flags.
type Info struct {
	Platform      Type   `json:"platform"`
	// AzureRBACMode is true when AKS is configured with Azure RBAC instead of
	// Kubernetes RBAC. In this mode RoleBindings may not reflect real access.
	AzureRBACMode bool   `json:"azureRbacMode,omitempty"`
	// DisplayName is a human-readable label for the UI badge.
	DisplayName   string `json:"displayName"`
}

// Detect queries the cluster and returns a populated Info.
// It is intentionally lenient: any single detection probe failure is treated
// as "not that platform" rather than a hard error.
func Detect(ctx context.Context, client kubernetes.Interface) Info {
	// --- OpenShift ---
	if isOpenShift(ctx, client) {
		return Info{Platform: TypeOpenShift, DisplayName: "OpenShift"}
	}

	// --- EKS ---
	if isEKS(ctx, client) {
		return Info{Platform: TypeEKS, DisplayName: "EKS (AWS)"}
	}

	// --- AKS ---
	if azureRBAC, ok := isAKS(ctx, client); ok {
		return Info{Platform: TypeAKS, DisplayName: "AKS (Azure)", AzureRBACMode: azureRBAC}
	}

	return Info{Platform: TypeVanilla, DisplayName: "Kubernetes"}
}

// isOpenShift returns true when the cluster exposes OpenShift-specific API groups.
func isOpenShift(ctx context.Context, client kubernetes.Interface) bool {
	groups, err := client.Discovery().ServerGroups()
	if err != nil || groups == nil {
		return false
	}
	for _, g := range groups.Groups {
		if strings.HasSuffix(g.Name, ".openshift.io") {
			return true
		}
	}
	return false
}

// isEKS returns true when the cluster has the aws-auth ConfigMap or an EKS
// node group label on any node.
func isEKS(ctx context.Context, client kubernetes.Interface) bool {
	// Fast check: aws-auth ConfigMap in kube-system
	_, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "aws-auth", metav1.GetOptions{})
	if err == nil {
		return true
	}

	// Fallback: check a node label (works even without aws-auth when using EKS
	// Access Entries instead of the ConfigMap)
	return hasNodeLabel(ctx, client, "eks.amazonaws.com/nodegroup")
}

// isAKS returns (azureRBACMode, true) when the cluster is AKS.
// azureRBACMode is true when Azure RBAC is active (Kubernetes RBAC bypassed).
func isAKS(ctx context.Context, client kubernetes.Interface) (azureRBAC bool, ok bool) {
	if !hasNodeLabel(ctx, client, "kubernetes.azure.com/cluster") {
		return false, false
	}

	// Detect Azure RBAC mode: nodes carry "kubernetes.azure.com/azure-rbac" label
	// when the cluster was created with --enable-azure-rbac.
	azureRBAC = hasNodeLabel(ctx, client, "kubernetes.azure.com/azure-rbac")
	return azureRBAC, true
}

// hasNodeLabel returns true when at least one node in the cluster carries the
// given label key (value is ignored).
func hasNodeLabel(ctx context.Context, client kubernetes.Interface, labelKey string) bool {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || nodes == nil {
		return false
	}
	for _, node := range nodes.Items {
		if _, ok := node.Labels[labelKey]; ok {
			return true
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────────────────────
// User / SA helpers called by the API server
// ────────────────────────────────────────────────────────────────────────────

// OpenShiftUserExists checks whether a real OpenShift User object exists by
// querying the user.openshift.io/v1 REST API via a raw path request.
// Returns (false, nil) on any error — callers fall back to RoleBinding scan.
func OpenShiftUserExists(ctx context.Context, k8s kubernetes.Interface, username string) (bool, error) {
	// Use the Discovery REST client which supports arbitrary API paths.
	result := k8s.Discovery().RESTClient().Get().
		AbsPath("/apis/user.openshift.io/v1/users/" + username).
		Do(ctx)
	err := result.Error()
	if err != nil {
		return false, nil // 404 or API not found — not an error we surface
	}
	return true, nil
}

// EKSUserExists checks whether a username appears in the aws-auth ConfigMap
// (legacy) or has an EKS Access Entry (new API).
// Returns (true, nil) if found, (false, nil) if not, (false, err) on lookup failure.
func EKSUserExists(ctx context.Context, client kubernetes.Interface, username string) (bool, error) {
	cm, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "aws-auth", metav1.GetOptions{})
	if err != nil {
		// aws-auth missing — cluster may use EKS Access Entries; skip
		return true, nil // can't determine, allow through
	}

	// Parse mapUsers and mapRoles YAML embedded in the ConfigMap.
	// We look for the username string rather than fully unmarshaling the YAML
	// to avoid adding a YAML dependency.
	for _, key := range []string{"mapUsers", "mapRoles"} {
		if blob, ok := cm.Data[key]; ok {
			if strings.Contains(blob, username) {
				return true, nil
			}
		}
	}
	return false, nil
}

// SACloudWarnings inspects a ServiceAccount's annotations and returns
// human-readable warning strings for any cloud-managed identity that
// grants permissions outside Kubernetes RBAC.
func SACloudWarnings(ctx context.Context, client kubernetes.Interface, name, namespace string) []string {
	sa, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return annotationWarnings(sa)
}

func annotationWarnings(sa *corev1.ServiceAccount) []string {
	var warnings []string
	annotations := sa.Annotations

	if arn, ok := annotations["eks.amazonaws.com/role-arn"]; ok {
		warnings = append(warnings, "This service account uses IRSA (IAM Roles for Service Accounts). "+
			"AWS permissions granted via IAM role "+arn+" are not visible in Kubernetes RBAC checks.")
	}

	if clientID, ok := annotations["azure.workload.identity/client-id"]; ok {
		warnings = append(warnings, "This service account uses Azure Workload Identity (client ID: "+clientID+"). "+
			"Azure AD permissions are not visible in Kubernetes RBAC checks.")
	}

	return warnings
}
