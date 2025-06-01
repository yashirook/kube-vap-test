package admission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMatchPolicy(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, MatchPolicy("Exact"), Exact)
	assert.Equal(t, MatchPolicy("Equivalent"), Equivalent)
}

func TestAdmissionTarget_PrepareObject(t *testing.T) {
	tests := []struct {
		name      string
		target    *AdmissionTarget
		wantObj   map[string]interface{}
		wantNil   bool
	}{
		{
			name: "empty target",
			target: &AdmissionTarget{},
			wantNil: true,
		},
		{
			name: "with namespace only",
			target: &AdmissionTarget{
				Namespace: "test-namespace",
			},
			wantObj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "test-namespace",
				},
			},
		},
		{
			name: "with labels only",
			target: &AdmissionTarget{
				Labels: map[string]string{
					"app":  "test",
					"tier": "backend",
				},
			},
			wantObj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app":  "test",
						"tier": "backend",
					},
				},
			},
		},
		{
			name: "with namespace and labels",
			target: &AdmissionTarget{
				Namespace: "production",
				Labels: map[string]string{
					"env": "prod",
					"version": "v1",
				},
			},
			wantObj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "production",
					"labels": map[string]interface{}{
						"env": "prod",
						"version": "v1",
					},
				},
			},
		},
		{
			name: "existing object gets metadata added",
			target: &AdmissionTarget{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
				Namespace: "default",
			},
			wantObj: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
		},
		{
			name: "existing metadata gets merged",
			target: &AdmissionTarget{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-pod",
					},
				},
				Namespace: "kube-system",
				Labels: map[string]string{
					"component": "api",
				},
			},
			wantObj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "kube-system",
					"labels": map[string]interface{}{
						"component": "api",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.target.PrepareObject()
			
			if tt.wantNil {
				assert.Nil(t, tt.target.Object)
			} else {
				assert.Equal(t, tt.wantObj, tt.target.Object)
			}
		})
	}
}

func TestNewAdmissionTarget(t *testing.T) {
	tests := []struct {
		name          string
		obj           *unstructured.Unstructured
		operation     string
		wantTarget    *AdmissionTarget
	}{
		{
			name:      "nil object",
			obj:       nil,
			operation: "CREATE",
			wantTarget: &AdmissionTarget{
				Operation: "CREATE",
			},
		},
		{
			name: "pod object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
			operation: "UPDATE",
			wantTarget: &AdmissionTarget{
				Operation:  "UPDATE",
				APIGroup:   "",
				APIVersion: "v1",
				Resource:   "Pod",
				Namespace:  "default",
				Labels: map[string]string{
					"app": "test",
				},
			},
		},
		{
			name: "deployment object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "production",
						"labels": map[string]interface{}{
							"app":     "api",
							"version": "v2",
						},
					},
				},
			},
			operation: "DELETE",
			wantTarget: &AdmissionTarget{
				Operation:  "DELETE",
				APIGroup:   "apps",
				APIVersion: "v1",
				Resource:   "Deployment",
				Namespace:  "production",
				Labels: map[string]string{
					"app":     "api",
					"version": "v2",
				},
			},
		},
		{
			name: "custom resource",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1alpha1",
					"kind":       "MyResource",
					"metadata": map[string]interface{}{
						"name": "test-resource",
					},
				},
			},
			operation: "CREATE",
			wantTarget: &AdmissionTarget{
				Operation:  "CREATE",
				APIGroup:   "example.com",
				APIVersion: "v1alpha1",
				Resource:   "MyResource",
				Namespace:  "",
				Labels:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewAdmissionTarget(tt.obj, tt.operation)

			assert.Equal(t, tt.wantTarget.Operation, target.Operation)
			assert.Equal(t, tt.wantTarget.APIGroup, target.APIGroup)
			assert.Equal(t, tt.wantTarget.APIVersion, target.APIVersion)
			assert.Equal(t, tt.wantTarget.Resource, target.Resource)
			assert.Equal(t, tt.wantTarget.Namespace, target.Namespace)
			assert.Equal(t, tt.wantTarget.Labels, target.Labels)

			if tt.obj != nil {
				assert.Equal(t, tt.obj.UnstructuredContent(), target.Object)
			}
		})
	}
}

func TestAdmissionTarget_CompleteFlow(t *testing.T) {
	// Test complete flow: create unstructured -> NewAdmissionTarget -> PrepareObject
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-service",
			},
		},
	}

	target := NewAdmissionTarget(obj, "CREATE")
	
	// Add additional metadata
	target.Namespace = "test-ns"
	target.Labels = map[string]string{"tier": "frontend"}
	
	// This should not affect the object yet
	assert.Equal(t, "", obj.GetNamespace())
	
	// Prepare object
	target.PrepareObject()
	
	// Check that metadata was added
	metadata, ok := target.Object["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test-ns", metadata["namespace"])
	
	labels, ok := metadata["labels"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "frontend", labels["tier"])
}