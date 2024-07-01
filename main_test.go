package main

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPrintObject(t *testing.T) {
	// Manually create the first mock Kubernetes resource
	now := metav1.Now()
	mockObj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "mock-pod",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "PodCompleted",
						"message":            "The pod has completed successfully.",
						"lastUpdateTime":     now.UTC().Format(time.RFC3339),
						"lastTransitionTime": now.UTC().Format(time.RFC3339),
						"observedGeneration": int64(1),
					},
					map[string]interface{}{
						"type":               "ContainersReady",
						"status":             "False",
						"reason":             "ContainersNotReady",
						"message":            "Some containers are not ready.",
						"lastUpdateTime":     now.UTC().Format(time.RFC3339),
						"lastTransitionTime": now.UTC().Format(time.RFC3339),
						"observedGeneration": int64(1),
					},
				},
			},
		},
	}

	// Manually create the second mock Kubernetes resource with a long URL in the message
	mockObj2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "mock-service",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Available",
						"status":             "True",
						"reason":             "ServiceAvailable",
						"message":            "The service is available at the following URL: https://example.com/this/is/a/very/long/url/that/should/be/wrapped/properly/by/the/printObject/function",
						"lastUpdateTime":     now.UTC().Format(time.RFC3339),
						"lastTransitionTime": now.UTC().Format(time.RFC3339),
						"observedGeneration": int64(2),
					},
				},
			},
		},
	}

	// Test the first mock object
	if err := printObject(mockObj1); err != nil {
		t.Errorf("printObject returned an error for mockObj1: %v", err)
	}

	// Test the second mock object
	if err := printObject(mockObj2); err != nil {
		t.Errorf("printObject returned an error for mockObj2: %v", err)
	}
}
