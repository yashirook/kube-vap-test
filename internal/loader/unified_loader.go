package loader

import (
	"context"
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
)

// UnifiedResourceLoader is a unified resource loader interface
type UnifiedResourceLoader struct {
	localLoader   *LocalResourceLoader
	clusterLoader *ClusterResourceLoader
	isClusterMode bool
}

// NewUnifiedResourceLoader creates a unified resource loader
func NewUnifiedResourceLoader(kubeconfigPath string, clusterMode bool) (*UnifiedResourceLoader, error) {
	localLoader, err := NewLocalResourceLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize local resource loader: %w", err)
	}

	var clusterLoader *ClusterResourceLoader
	if clusterMode {
		clusterLoader, err = NewClusterResourceLoader(kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cluster resource loader: %w", err)
		}
	}

	return &UnifiedResourceLoader{
		localLoader:   localLoader,
		clusterLoader: clusterLoader,
		isClusterMode: clusterMode,
	}, nil
}

// GetActiveLoader returns the appropriate loader based on the current mode
func (u *UnifiedResourceLoader) GetActiveLoader() ResourceLoader {
	if u.isClusterMode && u.clusterLoader != nil {
		return u.clusterLoader
	}
	return u.localLoader
}

// LoadFromSource selects the appropriate loader based on source type and loads resources
func (u *UnifiedResourceLoader) LoadFromSource(source ResourceSource) (ResourceLoader, error) {
	switch source.Type {
	case SourceTypeLocal:
		return u.localLoader, nil
	case SourceTypeCluster:
		if u.clusterLoader == nil {
			// Create cluster loader if not initialized
			clusterLoader, err := NewClusterResourceLoader(source.KubeconfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize cluster resource loader: %w", err)
			}
			u.clusterLoader = clusterLoader
		}
		return u.clusterLoader, nil
	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}

// ResourceLoadOptions represents options for resource loading
type ResourceLoadOptions struct {
	// Progress callback function
	ProgressCallback func(current, total int, message string)
	// Error handling strategy
	ContinueOnError bool
	// Timeout setting
	TimeoutSeconds int
}

// LoadResourcesWithProgress loads resources with progress display
func (u *UnifiedResourceLoader) LoadResourcesWithProgress(
	ctx context.Context,
	source ResourceSource,
	resourceType string,
	options ResourceLoadOptions,
) ([]runtime.Object, error) {
	loader, err := u.LoadFromSource(source)
	if err != nil {
		return nil, err
	}

	// Call progress callback if available
	if options.ProgressCallback != nil {
		options.ProgressCallback(0, 1, fmt.Sprintf("Loading resources from %s...", source.Type))
	}

	resources, err := loader.GetResources(ctx, resourceType, source)
	if err != nil && !options.ContinueOnError {
		return nil, err
	}

	if options.ProgressCallback != nil {
		options.ProgressCallback(1, 1, fmt.Sprintf("Loaded %d resources", len(resources)))
	}

	return resources, nil
}

// ValidateSource validates resource source configuration
func ValidateSource(source ResourceSource) error {
	switch source.Type {
	case SourceTypeLocal:
		if len(source.Files) == 0 {
			return fmt.Errorf("local source requires file paths")
		}
		for _, file := range source.Files {
			if !filepath.IsAbs(file) {
				return fmt.Errorf("file path must be absolute: %s", file)
			}
		}
	case SourceTypeCluster:
		// For cluster source, no mandatory parameters (uses default kubeconfig)
	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}
	return nil
}

// CreateSourceFromFlags creates ResourceSource from CLI flags
func CreateSourceFromFlags(isCluster bool, files []string, namespace, kubeconfigPath string) ResourceSource {
	if isCluster {
		return ResourceSource{
			Type:           SourceTypeCluster,
			KubeconfigPath: kubeconfigPath,
			Namespace:      namespace,
		}
	}
	return ResourceSource{
		Type:  SourceTypeLocal,
		Files: files,
	}
}