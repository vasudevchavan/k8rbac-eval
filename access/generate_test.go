package access

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateManifests(t *testing.T) {
	tests := []struct {
		name             string
		userName         string
		isServiceAccount bool
		resource         string
		group            string
		verbs            []string
		namespace        string
		namespaced       bool
		expectedRoleKind string
		expectedSubject  string
	}{
		{
			name:             "User Role",
			userName:         "alice",
			isServiceAccount: false,
			resource:         "pods",
			group:            "",
			verbs:            []string{"get", "list"},
			namespace:        "default",
			namespaced:       true,
			expectedRoleKind: "Role",
			expectedSubject:  "User",
		},
		{
			name:             "User ClusterRole",
			userName:         "bob",
			isServiceAccount: false,
			resource:         "nodes",
			group:            "",
			verbs:            []string{"watch"},
			namespace:        "",
			namespaced:       false,
			expectedRoleKind: "ClusterRole",
			expectedSubject:  "User",
		},
		{
			name:             "ServiceAccount Role",
			userName:         "my-sa",
			isServiceAccount: true,
			resource:         "deployments",
			group:            "apps",
			verbs:            []string{"create"},
			namespace:        "my-ns",
			namespaced:       true,
			expectedRoleKind: "Role",
			expectedSubject:  "ServiceAccount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roleBytes, bindingBytes, err := generateManifests(
				tt.userName,
				tt.isServiceAccount,
				tt.resource,
				tt.group,
				tt.verbs,
				tt.namespace,
				tt.namespaced,
			)

			assert.NoError(t, err)
			assert.NotEmpty(t, roleBytes)
			assert.NotEmpty(t, bindingBytes)

			roleStr := string(roleBytes)
			bindingStr := string(bindingBytes)

			assert.Contains(t, roleStr, "kind: "+tt.expectedRoleKind)
			assert.Contains(t, roleStr, tt.resource)
			if tt.group != "" {
				assert.Contains(t, roleStr, tt.group)
			}

			assert.Contains(t, bindingStr, "kind: "+tt.expectedRoleKind+"Binding")
			assert.Contains(t, bindingStr, "kind: "+tt.expectedSubject)
			assert.Contains(t, bindingStr, "name: "+tt.userName)
		})
	}
}
