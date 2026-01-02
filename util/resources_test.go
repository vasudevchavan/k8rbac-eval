package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestResolveResourceName(t *testing.T) {
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &k8stesting.Fake{},
	}

	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "pods",
					SingularName: "pod",
					ShortNames:   []string{"po"},
					Kind:         "Pod",
				},
				{
					Name:         "services",
					SingularName: "service",
					ShortNames:   []string{"svc"},
					Kind:         "Service",
				},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "deployments",
					SingularName: "deployment",
					ShortNames:   []string{"deploy"},
					Kind:         "Deployment",
				},
			},
		},
	}

	tests := []struct {
		name          string
		input         string
		expectedName  string
		expectedError bool
	}{
		{
			name:         "resolve plural",
			input:        "pods",
			expectedName: "pods",
		},
		{
			name:         "resolve singular",
			input:        "pod",
			expectedName: "pods",
		},
		{
			name:         "resolve shortname",
			input:        "po",
			expectedName: "pods",
		},
		{
			name:         "resolve mixed case",
			input:        "PoDs",
			expectedName: "pods",
		},
		{
			name:         "resolve deployment shortname",
			input:        "deploy",
			expectedName: "deployments",
		},
		{
			name:          "not found",
			input:         "unknown",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveResourceName(fakeDiscovery, tt.input)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, got)
			}
		})
	}
}
