package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/yashirook/kube-vap-test/internal/engine"
	"github.com/yashirook/kube-vap-test/internal/loader"
	"github.com/yashirook/kube-vap-test/internal/reporter"
	kaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// CheckOptions represents options for check command
type CheckOptions struct {
	CommonOptions
	
	// Command specific
	PolicyFiles []string
	ParamFile   string
	Operation   string
	Namespace   string
	Cluster     bool
}

// NewCheckCommand creates a new check command
func NewCheckCommand(opts *CheckOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [resources...]",
		Short: "Check resources against policies",
		Long:  `Check specified resources (local files or cluster resources) against specified policies.
With --cluster flag, resources are fetched directly from the cluster for validation.`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Do not show usage on errors
			cmd.SilenceUsage = true

			// Check required parameters
			if len(opts.PolicyFiles) == 0 {
				return fmt.Errorf("Please specify policy files with --policy flag")
			}

			if opts.Operation == "" {
				// Default to CREATE operation
				opts.Operation = "CREATE"
			}

			// Set up context (cancellable with Ctrl+C)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			// Set up signal handler
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				if !opts.Quiet {
					reporter.PrintWarning("Cancelling...")
				}
				cancel()
			}()

			// Validate common options
			if err := opts.ValidateCommonOptions(); err != nil {
				return err
			}

			// Initialize reporter
			rep := opts.GetReporter()

			// Initialize simulator
			simulator, err := engine.NewPolicySimulator()
			if err != nil {
				return fmt.Errorf("Failed to initialize policy simulator: %w", err)
			}

			// Branch processing for cluster mode and local mode
			if opts.Cluster {
				// Cluster mode
				return runClusterCheck(ctx, rep, simulator, args, opts)
			} else {
				// Local mode
				if len(args) == 0 {
					return fmt.Errorf("Please specify manifest files in local mode")
				}
				return runLocalCheck(ctx, rep, simulator, args, opts)
			}
		},
	}

	// Command-specific flags
	cmd.Flags().StringSliceVar(&opts.PolicyFiles, "policy", []string{}, "Policy files to use for validation (required, can specify multiple)")
	cmd.Flags().StringVar(&opts.ParamFile, "param", "", "Parameter file for policies (optional)")
	cmd.Flags().StringVar(&opts.Operation, "operation", "CREATE", "Operation to validate (CREATE, UPDATE, DELETE)")
	cmd.Flags().BoolVarP(&opts.Cluster, "cluster", "c", false, "Run in cluster mode (fetch resources from cluster)")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "Namespace to validate (cluster mode)")

	return cmd
}

// runLocalCheck executes check for local files
func runLocalCheck(ctx context.Context, rep reporter.Reporter, simulator *engine.PolicySimulator, manifestFiles []string, opts *CheckOptions) error {
	// Initialize resource loader
	resourceLoader, err := loader.NewLocalResourceLoader()
	if err != nil {
		return fmt.Errorf("Failed to initialize resource loader: %w", err)
	}

	// Source configuration for loading policy files
	resourceSource := loader.ResourceSource{
		Type:  loader.SourceTypeLocal,
		Files: opts.PolicyFiles,
	}

	// Load multiple policy files
	policies, err := resourceLoader.LoadPolicies(resourceSource)
	if err != nil {
		if !opts.Quiet {
			reporter.PrintError(fmt.Errorf("Failed to load policies: %w", err))
		}
		return fmt.Errorf("Failed to load policies: %w", err)
	}

	for _, policy := range policies {
		if !opts.Quiet {
			reporter.PrintInfo(fmt.Sprintf("Using policy '%s' for validation", policy.Name))
		}
	}

	// Load parameters (optional)
	var paramObj runtime.Object
	if opts.ParamFile != "" {
		paramSource := loader.ResourceSource{
			Type:  loader.SourceTypeLocal,
			Files: []string{opts.ParamFile},
		}
		paramObj, err = resourceLoader.LoadParameter(paramSource)
		if err != nil {
			if !opts.Quiet {
				reporter.PrintWarning(fmt.Sprintf("Failed to load parameters: %s", err.Error()))
			}
		}
	}

	// Store validation results by resource type
	var allResults []*kaptestv1.TestResult
	var totalCount, successCount, failedCount int

	// Process manifest files
	for _, manifestPath := range manifestFiles {
		manifestPath = filepath.Clean(manifestPath)

		if !opts.Quiet {
			reporter.PrintInfo(fmt.Sprintf("Processing manifest file: %s", manifestPath))
		}

		results, err := processManifestFile(ctx, manifestPath, policies, paramObj, simulator, opts)
		if err != nil {
			reporter.PrintError(err)
			continue
		}

		for _, result := range results {
			totalCount++
			if result.ActualResponse.Allowed {
				successCount++
			} else {
				failedCount++
			}
			allResults = append(allResults, result)
		}
	}

	// Create test result status
	resultsArray := make([]kaptestv1.TestResult, len(allResults))
	for i, result := range allResults {
		resultsArray[i] = *result
	}

	status := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: resultsArray,
		Summary: kaptestv1.TestSummary{
			Total:      totalCount,
			Successful: successCount,
			Failed:     failedCount,
		},
	}

	// Report test results
	if err := rep.Report(status); err != nil {
		reporter.PrintError(fmt.Errorf("Failed to report results: %w", err))
		return err
	}

	// check command only reports validation results and does not return error even if there are failures
	return nil
}

func processManifestFile(ctx context.Context, manifestPath string, policies []*admissionregistrationv1.ValidatingAdmissionPolicy, paramObj runtime.Object, simulator *engine.PolicySimulator, opts *CheckOptions) ([]*kaptestv1.TestResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read manifest file: %w", err)
	}

	var results []*kaptestv1.TestResult

	// Split YAML documents
	documents := bytes.Split(data, []byte("---\n"))
	for _, doc := range documents {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		// Convert to JSON
		jsonData, err := yaml.YAMLToJSON(doc)
		if err != nil {
			reporter.PrintError(fmt.Errorf("Failed to convert YAML to JSON: %w", err))
			continue
		}

		// Convert to Unstructured for dynamic client
		unstructuredObj := &unstructured.Unstructured{}
		if err := unstructuredObj.UnmarshalJSON(jsonData); err != nil {
			reporter.PrintError(fmt.Errorf("Failed to convert JSON to Unstructured: %w", err))
			continue
		}

		resourceType := fmt.Sprintf("%s.%s", strings.ToLower(unstructuredObj.GetKind()), unstructuredObj.GetAPIVersion())
		resourceName := unstructuredObj.GetName()

		if resourceName == "" {
			resourceName = fmt.Sprintf("unnamed-%s", uuid.NewString()[:8])
		}

		// Create test case
		testCase := kaptestv1.TestCase{
			Name:      fmt.Sprintf("%s/%s", resourceType, resourceName),
			Operation: opts.Operation,
			Object: runtime.RawExtension{
				Object: unstructuredObj,
			},
			Expected: kaptestv1.ExpectedResult{
				Allowed: true, // Expected value is not used, so any value is OK
			},
		}

		// Simulation using multiple policies
		result, err := simulator.SimulateTestCaseWithMultiPolicies(ctx, policies, paramObj, testCase)
		if err != nil {
			reporter.PrintError(fmt.Errorf("Failed to validate resource (%s/%s): %w", resourceType, resourceName, err))
			continue
		}

		// Add resource type information
		result.Metadata = map[string]string{
			"resourceType": resourceType,
		}

		results = append(results, result)
	}

	return results, nil
}

// runClusterCheck executes check for cluster resources
func runClusterCheck(ctx context.Context, rep reporter.Reporter, simulator *engine.PolicySimulator, resourceSpecs []string, opts *CheckOptions) error {
	// Initialize resource loader
	resourceLoader, err := loader.NewClusterResourceLoader(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster resource loader: %w", err)
	}

	// Source configuration for loading policy files
	policySource := loader.ResourceSource{
		Type:  loader.SourceTypeLocal,
		Files: opts.PolicyFiles,
	}

	// Load multiple policy files
	policies, err := resourceLoader.LoadPolicies(policySource)
	if err != nil {
		if !opts.Quiet {
			reporter.PrintError(fmt.Errorf("Failed to load policies: %w", err))
		}
		return fmt.Errorf("Failed to load policies: %w", err)
	}

	for _, policy := range policies {
		if !opts.Quiet {
			reporter.PrintInfo(fmt.Sprintf("Using policy '%s' for validation", policy.Name))
		}
	}

	// Load parameters (optional)
	var paramObj runtime.Object
	if opts.ParamFile != "" {
		paramSource := loader.ResourceSource{
			Type:  loader.SourceTypeLocal,
			Files: []string{opts.ParamFile},
		}
		paramObj, err = resourceLoader.LoadParameter(paramSource)
		if err != nil {
			if !opts.Quiet {
				reporter.PrintWarning(fmt.Sprintf("Failed to load parameters: %s", err.Error()))
			}
		}
	}

	// Determine resource types to validate
	var resourceTypes []string
	if len(resourceSpecs) > 0 {
		// When specified in arguments
		resourceTypes = resourceSpecs
	} else {
		// Extract target resources from policies
		resourceTypes, err = extractResourceTypesFromPolicies(policies)
		if err != nil {
			return fmt.Errorf("Failed to extract resource types from policies: %w", err)
		}
	}

	if !opts.Quiet {
		reporter.PrintInfo(fmt.Sprintf("Resources to validate: %s", strings.Join(resourceTypes, ", ")))
	}

	// When no resources are specified
	if len(resourceTypes) == 0 {
		return fmt.Errorf("No resources specified for validation")
	}

	if !opts.Quiet {
		reporter.PrintInfo(fmt.Sprintf("Fetching resources from cluster..."))
	}

	// Store validation results by resource type
	var allResults []*kaptestv1.TestResult
	var totalCount, successCount, failedCount int

	// Configuration for fetching resources from cluster
	resourceSource := loader.ResourceSource{
		Type:      loader.SourceTypeCluster,
		Namespace: opts.Namespace,
	}

	// Process each resource type
	for _, resourceType := range resourceTypes {
		// Fetch resources from cluster
		resources, err := resourceLoader.GetResources(ctx, resourceType, resourceSource)
		if err != nil {
			reporter.PrintError(fmt.Errorf("Failed to fetch resources (%s): %w", resourceType, err))
			continue
		}

		if !opts.Quiet {
			reporter.PrintInfo(fmt.Sprintf("Fetched resources: %s (%d items)", resourceType, len(resources)))
		}

		// Validate each resource with policies
		for _, resource := range resources {
			// Get resource name
			unstructuredObj, ok := resource.(*unstructured.Unstructured)
			if !ok {
				reporter.PrintError(fmt.Errorf("Failed to convert resource type: %T", resource))
				continue
			}

			resourceName := unstructuredObj.GetName()

			// Create test case
			testCase := kaptestv1.TestCase{
				Name:      fmt.Sprintf("%s/%s", resourceType, resourceName),
				Operation: "CREATE", // Assume CREATE operation for validation
				Object: runtime.RawExtension{
					Object: resource,
				},
				Expected: kaptestv1.ExpectedResult{
					Allowed: true, // Expected value is not used, so any value is OK
				},
			}

			// Simulation using multiple policies
			result, err := simulator.SimulateTestCaseWithMultiPolicies(ctx, policies, paramObj, testCase)
			if err != nil {
				reporter.PrintError(fmt.Errorf("Failed to validate resource (%s/%s): %w", resourceType, resourceName, err))
				continue
			}

			// Save results
			totalCount++
			if result.ActualResponse.Allowed {
				successCount++
			} else {
				failedCount++
			}

			// Add resource type information
			result.Metadata = map[string]string{
				"resourceType": resourceType,
			}

			allResults = append(allResults, result)
		}
	}

	// Create test result status
	resultsArray := make([]kaptestv1.TestResult, len(allResults))
	for i, result := range allResults {
		resultsArray[i] = *result
	}

	status := &kaptestv1.ValidatingAdmissionPolicyTestStatus{
		Results: resultsArray,
		Summary: kaptestv1.TestSummary{
			Total:      totalCount,
			Successful: successCount,
			Failed:     failedCount,
		},
	}

	// Report test results
	if err := rep.Report(status); err != nil {
		reporter.PrintError(fmt.Errorf("Failed to report results: %w", err))
		return err
	}

	// check command only reports validation results and does not return error even if there are failures
	return nil
}

// extractResourceTypesFromPolicies extracts target resource types from policies
func extractResourceTypesFromPolicies(policies []*admissionregistrationv1.ValidatingAdmissionPolicy) ([]string, error) {
	resourceMap := make(map[string]bool)

	for _, policy := range policies {
		if policy.Spec.MatchConstraints == nil || len(policy.Spec.MatchConstraints.ResourceRules) == 0 {
			// Skip if no match constraints
			continue
		}

		for _, rule := range policy.Spec.MatchConstraints.ResourceRules {
			for _, resource := range rule.Resources {
				// "*" represents all resources but is not practical, so ignore it
				if resource != "*" {
					resourceMap[resource] = true
				}
			}
		}
	}

	// Convert map to slice
	resources := make([]string, 0, len(resourceMap))
	for resource := range resourceMap {
		resources = append(resources, resource)
	}

	return resources, nil
}