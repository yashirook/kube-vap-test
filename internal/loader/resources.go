package loader

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// SourceType defines where resources are loaded from
type SourceType string

const (
	// Load resources from local filesystem
	SourceTypeLocal SourceType = "local"
	// Load resources from Kubernetes cluster
	SourceTypeCluster SourceType = "cluster"
)

// ResourceSource represents the source of resources
type ResourceSource struct {
	// Type of source
	Type SourceType
	// File paths for local mode
	Files []string
	// Reference for cluster mode
	KubeconfigPath string
	// Namespace (for cluster mode)
	Namespace string
}

// ResourceLoader is an interface for loading resources
type ResourceLoader interface {
	// LoadPolicyTest loads test definitions
	// When runMode is true, returns an error for manifests containing multiple resources
	LoadPolicyTest(filePath string, runMode bool) (*kaptestv1.ValidatingAdmissionPolicyTest, error)

	// LoadPolicies loads policies
	// Loads from local files or cluster based on source configuration
	LoadPolicies(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error)

	// LoadPolicyBindings loads policy bindings
	// Loads from local files or cluster based on source configuration
	LoadPolicyBindings(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error)

	// LoadParameter loads parameters from the specified source
	LoadParameter(source ResourceSource) (runtime.Object, error)

	// GetResources retrieves resources
	// Loads from local files or cluster based on source configuration
	GetResources(ctx context.Context, resourceType string, source ResourceSource) ([]runtime.Object, error)
}

// LocalResourceLoader loads resources from local files
type LocalResourceLoader struct {
	scheme *runtime.Scheme
	codecs serializer.CodecFactory
}

// NewLocalResourceLoader creates a new LocalResourceLoader
func NewLocalResourceLoader() (*LocalResourceLoader, error) {
	scheme := runtime.NewScheme()
	if err := admissionregistrationv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add admissionregistrationv1 schema: %w", err)
	}

	return &LocalResourceLoader{
		scheme: scheme,
		codecs: serializer.NewCodecFactory(scheme),
	}, nil
}

// LoadPolicyTest loads test definitions
// When runMode is true, returns an error for manifests containing multiple resources
func (l *LocalResourceLoader) LoadPolicyTest(filePath string, runMode bool) (*kaptestv1.ValidatingAdmissionPolicyTest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	test := &kaptestv1.ValidatingAdmissionPolicyTest{}
	if err := yaml.Unmarshal(data, test); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test file: %w", err)
	}

	// Load test case objects from external files
	for i := range test.Spec.TestCases {
		testCase := &test.Spec.TestCases[i]

		// If object file is specified
		if testCase.ObjectFile != "" {
			// Resolve path relative to test file directory
			objectPath := filepath.Join(filepath.Dir(filePath), testCase.ObjectFile)
			objectData, err := os.ReadFile(objectPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read object file (%s): %w", objectPath, err)
			}

			// Check for multiple resources in run command
			if runMode {
				// Check if there are multiple YAML documents
				objectDocs := bytes.Split(objectData, []byte("---\n"))
				validDocs := 0
				for _, doc := range objectDocs {
					if len(bytes.TrimSpace(doc)) > 0 {
						validDocs++
					}
				}

				// Error if there are multiple valid documents
				if validDocs > 1 {
					return nil, fmt.Errorf("manifest file contains multiple resources. run command only supports single resource (%s)", objectPath)
				}
			}

			// Use the first valid YAML document
			var docToUse []byte
			if bytes.Contains(objectData, []byte("---\n")) {
				// May be split into multiple documents
				objectDocs := bytes.Split(objectData, []byte("---\n"))
				for _, doc := range objectDocs {
					trimmed := bytes.TrimSpace(doc)
					if len(trimmed) > 0 {
						docToUse = trimmed
						break
					}
				}
			} else {
				// Single document case
				docToUse = objectData
			}

			// Convert YAML to JSON
			jsonData, err := yaml.YAMLToJSON(docToUse)
			if err != nil {
				return nil, fmt.Errorf("failed to convert object file to JSON (%s): %w", objectPath, err)
			}

			testCase.Object = runtime.RawExtension{
				Raw: jsonData,
			}
		}

		// If old object file is specified
		if testCase.OldObjectFile != "" {
			// Resolve path relative to test file directory
			oldObjectPath := filepath.Join(filepath.Dir(filePath), testCase.OldObjectFile)
			oldObjectData, err := os.ReadFile(oldObjectPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read old object file (%s): %w", oldObjectPath, err)
			}

			// Convert YAML to JSON
			jsonData, err := yaml.YAMLToJSON(oldObjectData)
			if err != nil {
				return nil, fmt.Errorf("failed to convert old object file to JSON (%s): %w", oldObjectPath, err)
			}

			testCase.OldObject = &runtime.RawExtension{
				Raw: jsonData,
			}
		}
	}

	return test, nil
}

// LoadPolicies loads policies
func (l *LocalResourceLoader) LoadPolicies(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	if source.Type == SourceTypeLocal {
		// Load policies from local files
		return l.loadLocalPolicies(source.Files)
	} else if source.Type == SourceTypeCluster {
		// Local loader does not support loading from cluster
		return nil, fmt.Errorf("local resource loader cannot load cluster policies")
	}
	return nil, fmt.Errorf("unknown source type: %s", source.Type)
}

// loadLocalPolicies loads policies from a list of local file paths (internal method)
func (l *LocalResourceLoader) loadLocalPolicies(filePaths []string) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	policies := make([]*admissionregistrationv1.ValidatingAdmissionPolicy, 0, len(filePaths))

	for _, filePath := range filePaths {
		policy, err := l.loadPolicyFromFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy (%s): %w", filePath, err)
		}
		policies = append(policies, policy)
	}

	return policies, nil
}

// loadPolicyFromFile loads a policy directly from file path (internal method)
func (l *LocalResourceLoader) loadPolicyFromFile(filePath string) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	// Parse YAML
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	decoder := serializer.NewCodecFactory(l.scheme).UniversalDeserializer()
	if _, _, err := decoder.Decode(data, nil, policy); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return policy, nil
}

// LoadPolicyBindings loads policy bindings
func (l *LocalResourceLoader) LoadPolicyBindings(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	if source.Type == SourceTypeLocal {
		// Load policy bindings from local files
		return l.loadLocalPolicyBindings(source.Files)
	} else if source.Type == SourceTypeCluster {
		// Local loader does not support loading from cluster
		return nil, fmt.Errorf("local resource loader cannot load cluster policy bindings")
	}
	return nil, fmt.Errorf("unknown source type: %s", source.Type)
}

// loadLocalPolicyBindings loads policy bindings from a list of local file paths (internal method)
func (l *LocalResourceLoader) loadLocalPolicyBindings(filePaths []string) ([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	bindings := make([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, 0, len(filePaths))

	for _, path := range filePaths {
		binding, err := l.loadPolicyBindingFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy binding (%s): %w", path, err)
		}
		bindings = append(bindings, binding)
	}

	return bindings, nil
}

// loadPolicyBindingFromFile loads a policy binding directly from file path (internal method)
func (l *LocalResourceLoader) loadPolicyBindingFromFile(filePath string) (*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy binding file: %w", err)
	}

	// Parse YAML
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
	decoder := serializer.NewCodecFactory(l.scheme).UniversalDeserializer()
	if _, _, err := decoder.Decode(data, nil, binding); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return binding, nil
}

// LoadParameter loads parameters
func (l *LocalResourceLoader) LoadParameter(source ResourceSource) (runtime.Object, error) {
	// Load from source files
	if source.Type == SourceTypeLocal && len(source.Files) > 0 {
		// Load and merge multiple parameter files
		var result runtime.Object
		for i, file := range source.Files {
			param, err := l.loadParameterFromFile(file)
			if err != nil {
				// Skip if not a parameter file
				continue
			}

			if i == 0 {
				// Use the first file as base
				result = param
			} else {
				// TODO: Add implementation to merge multiple parameters in the future
				// Currently the last loaded file takes precedence
				result = param
			}
		}
		return result, nil
	}

	// No parameter source specified
	return nil, nil
}

// loadParameterFromFile loads parameters directly from file path (internal method)
func (l *LocalResourceLoader) loadParameterFromFile(filePath string) (runtime.Object, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read parameter file: %w", err)
	}

	obj, err := l.parseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parameter file: %w", err)
	}

	return obj, nil
}

// GetResources retrieves resources
func (l *LocalResourceLoader) GetResources(ctx context.Context, resourceType string, source ResourceSource) ([]runtime.Object, error) {
	return nil, fmt.Errorf("resource retrieval is not supported in local mode")
}

// parsePolicy parses ValidatingAdmissionPolicy from byte data
func (l *LocalResourceLoader) parsePolicy(data []byte) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	if err := yaml.Unmarshal(data, policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
	}
	return policy, nil
}

// parseYAML parses runtime object from byte data
func (l *LocalResourceLoader) parseYAML(data []byte) (runtime.Object, error) {
	obj := &runtime.Unknown{}
	if err := yaml.Unmarshal(data, obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}
	return obj, nil
}

// ClusterResourceLoader loads resources from cluster
type ClusterResourceLoader struct {
	clientset      *kubernetes.Clientset
	dynamicClient  dynamic.Interface
	scheme         *runtime.Scheme
	codecs         serializer.CodecFactory
	kubeconfigPath string
}

// GetResources retrieves resources of specified type from cluster
func (c *ClusterResourceLoader) GetResources(ctx context.Context, resourceType string, source ResourceSource) ([]runtime.Object, error) {
	// Get resources from dynamic client
	var gvr schema.GroupVersionResource

	// Set GVR based on resource type
	switch resourceType {
	case "pods":
		gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	case "deployments":
		gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	case "statefulsets":
		gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	case "daemonsets":
		gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	case "replicasets":
		gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	case "cronjobs":
		gvr = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	case "jobs":
		gvr = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	case "services":
		gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	case "configmaps":
		gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	case "secrets":
		gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	case "ingresses":
		gvr = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	// Target all namespaces if namespace is not specified
	var list *unstructured.UnstructuredList
	var err error

	listOptions := metav1.ListOptions{}

	if source.Namespace != "" {
		list, err = c.dynamicClient.Resource(gvr).Namespace(source.Namespace).List(ctx, listOptions)
	} else {
		list, err = c.dynamicClient.Resource(gvr).List(ctx, listOptions)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get resources: %w", err)
	}

	// Convert Unstructured list to slice of runtime.Object
	result := make([]runtime.Object, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}

	return result, nil
}

// NewClusterResourceLoader creates a new ClusterResourceLoader
func NewClusterResourceLoader(kubeconfigPath string) (*ClusterResourceLoader, error) {
	// Initialize Kubernetes client using kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Initialize schema
	scheme := runtime.NewScheme()
	if err := admissionregistrationv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add admissionregistrationv1 schema: %w", err)
	}

	return &ClusterResourceLoader{
		clientset:      clientset,
		dynamicClient:  dynamicClient,
		scheme:         scheme,
		codecs:         serializer.NewCodecFactory(scheme),
		kubeconfigPath: kubeconfigPath,
	}, nil
}

// LoadPolicyTest loads test definitions
func (c *ClusterResourceLoader) LoadPolicyTest(filePath string, runMode bool) (*kaptestv1.ValidatingAdmissionPolicyTest, error) {
	// Test definitions are always loaded from local files
	localLoader, err := NewLocalResourceLoader()
	if err != nil {
		return nil, err
	}
	return localLoader.LoadPolicyTest(filePath, runMode)
}

// LoadPolicies loads policies
func (c *ClusterResourceLoader) LoadPolicies(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	if source.Type == SourceTypeLocal {
		// Load policies from local files - 委譲パターン
		localLoader, err := NewLocalResourceLoader()
		if err != nil {
			return nil, err
		}
		return localLoader.LoadPolicies(source)
	} else if source.Type == SourceTypeCluster {
		// Get all ValidatingAdmissionPolicies from cluster
		policies, err := c.clientset.AdmissionregistrationV1().ValidatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list ValidatingAdmissionPolicies: %w", err)
		}

		result := make([]*admissionregistrationv1.ValidatingAdmissionPolicy, len(policies.Items))
		for i := range policies.Items {
			result[i] = &policies.Items[i]
		}

		return result, nil
	}
	return nil, fmt.Errorf("unknown source type: %s", source.Type)
}

// LoadPolicyBindings loads policy bindings
func (c *ClusterResourceLoader) LoadPolicyBindings(source ResourceSource) ([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	if source.Type == SourceTypeLocal {
		// Load policy bindings from local files - 委譲パターン
		localLoader, err := NewLocalResourceLoader()
		if err != nil {
			return nil, err
		}
		return localLoader.LoadPolicyBindings(source)
	} else if source.Type == SourceTypeCluster {
		// Get all policy bindings from cluster
		bindings, err := c.clientset.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get policy bindings from cluster: %w", err)
		}

		result := make([]*admissionregistrationv1.ValidatingAdmissionPolicyBinding, 0, len(bindings.Items))
		for i := range bindings.Items {
			result = append(result, &bindings.Items[i])
		}

		return result, nil
	}
	return nil, fmt.Errorf("unknown source type: %s", source.Type)
}

// LoadParameter loads parameters
func (c *ClusterResourceLoader) LoadParameter(source ResourceSource) (runtime.Object, error) {
	if source.Type == SourceTypeLocal {
		// Load parameters from local files - delegation pattern
		localLoader, err := NewLocalResourceLoader()
		if err != nil {
			return nil, err
		}
		return localLoader.LoadParameter(source)
	}

	// For cluster source, parameters loading is not yet implemented
	// In the future, we could load ConfigMaps or other parameter resources from cluster
	return nil, nil
}
