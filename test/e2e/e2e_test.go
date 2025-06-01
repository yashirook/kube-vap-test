package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Test target namespace
	testNamespace = "default"
	// Fixed context
	kindContext = "kind-kind"
)

// TestResult holds the command result structure
type TestResult struct {
	Results []ResourceResult `json:"results"`
	Summary SummaryResult    `json:"summary"`
}

// ResourceResult holds validation results for each resource
type ResourceResult struct {
	Name           string         `json:"name"`
	Success        bool           `json:"success"`
	Details        string         `json:"details,omitempty"`
	ActualResponse ResponseResult `json:"actualResponse"`
	PolicyResults  []PolicyResult `json:"policyResults"`
	Metadata       MetadataResult `json:"metadata"`
}

// ResponseResult holds the actual response result
type ResponseResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// PolicyResult holds evaluation results for each policy
type PolicyResult struct {
	PolicyName string `json:"policyName"`
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason,omitempty"`
	Message    string `json:"message,omitempty"`
}

// MetadataResult holds resource metadata
type MetadataResult struct {
	ResourceType string `json:"resourceType"`
}

// SummaryResult holds validation result summary
type SummaryResult struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

// E2ETestCase defines E2E test case
type E2ETestCase struct {
	Name               string            // Test name
	Description        string            // Test description
	Command            string            // Command to execute (test, check)
	PolicyFile         string            // Policy file to use
	TestFile           string            // Test definition file (for test command)
	DeploymentFiles    []string          // Resource files to deploy
	ManifestFiles      []string          // Manifest files to check (for check command)
	UseClusterMode     bool              // Whether to use cluster mode
	ExpectedSuccessful int               // Expected successful count
	ExpectedFailed     int               // Expected failed count
	ResourceChecks     map[string]bool   // Key: "resourceType/resourceName", Value: expected allowed state
	ReasonChecks       map[string]string // Key: "resourceType/resourceName", Value: expected denial reason
}

// TestNewCommandStructure runs E2E tests for the new command structure
func TestNewCommandStructure(t *testing.T) {
	// Define test cases
	testCases := []E2ETestCase{
		// Test for run command
		{
			Name:               "RunCommand_NoLatestTag",
			Description:        "Run test definitions for latest tag prohibition policy",
			Command:            "run",
			TestFile:           "../../examples/tests/no-latest-tag-test.yaml",
			ExpectedSuccessful: 4,
			ExpectedFailed:     0,
		},
		// Test for check command (local mode)
		{
			Name:           "CheckCommand_LocalMode_NoPrivileged",
			Description:    "Test privileged container prohibition policy with check command (local)",
			Command:        "check",
			PolicyFile:     "testdata/policies/no-privileged-policy.yaml",
			ManifestFiles:  []string{"testdata/privileged-pod.yaml", "testdata/non-privileged-pod.yaml"},
			UseClusterMode: false,
			ExpectedSuccessful: 1,
			ExpectedFailed:     1,
			ResourceChecks: map[string]bool{
				"pod.v1/non-privileged-pod": true,
				"pod.v1/privileged-pod":     false,
			},
			ReasonChecks: map[string]string{
				"pod.v1/privileged-pod": "PrivilegedContainerPolicy",
			},
		},
		// Test for check command (cluster mode)
		{
			Name:        "CheckCommand_ClusterMode_MultiResource",
			Description: "Test policies for multiple resources with check command (cluster)",
			Command:     "check",
			PolicyFile:  "testdata/policies/multi-resource-policy.yaml",
			DeploymentFiles: []string{
				"testdata/valid-deployment.yaml",
				"testdata/invalid-deployment.yaml",
				"testdata/valid-service.yaml",
				"testdata/invalid-service.yaml",
			},
			UseClusterMode:     true,
			ExpectedSuccessful: 4,
			ExpectedFailed:     2,
			ResourceChecks: map[string]bool{
				"deployments/valid-deployment":   true,
				"deployments/invalid-deployment": false,
				"services/valid-service":         true,
				"services/invalid-service":       false,
			},
			ReasonChecks: map[string]string{
				"deployments/invalid-deployment": "MissingAppLabel",
				"services/invalid-service":       "MissingAppLabel",
			},
		},
		// Test for validate command - commented out as validate command was removed
		// {
		// 	Name:        "ValidateCommand_TestFile",
		// 	Description: "Validate test definition file with validate command",
		// 	Command:     "validate",
		// 	TestFile:    "../../examples/tests/no-latest-tag-test.yaml",
		// 	// Validate command has different success/failure concept, so just ensure no errors
		// },
	}

	// Get project root directory
	rootDir, err := getRootDir()
	require.NoError(t, err, "Failed to get project root directory")

	// Build vaptest
	err = buildKadprobe(rootDir)
	require.NoError(t, err, "Failed to build vaptest")

	// Use kind-kind context for cluster mode tests
	var clientset *kubernetes.Clientset
	needsCluster := false
	for _, tc := range testCases {
		if tc.UseClusterMode {
			needsCluster = true
			break
		}
	}

	if needsCluster {
		err = switchContext(kindContext)
		require.NoError(t, err, "Failed to switch to kind-kind context")

		clientset, err = initializeClient(kindContext)
		require.NoError(t, err, "Failed to initialize Kubernetes client")
	}

	// Run each test case
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Running test case: %s - %s", tc.Name, tc.Description)

			// Deploy resources for cluster mode
			if tc.UseClusterMode && len(tc.DeploymentFiles) > 0 {
				// Clean up default namespace before test
				err = cleanupNamespace(clientset, testNamespace)
				require.NoError(t, err, "Failed to cleanup namespace")

				// Deploy test resources
				t.Logf("Deploying test resources for %s...", tc.Name)
				err = deployTestResources(rootDir, tc.DeploymentFiles)
				require.NoError(t, err, "Failed to deploy test resources")

				// Wait for deployed resources to be available
				t.Log("Waiting for resources to be ready...")
				err = waitForResources(clientset, testNamespace)
				require.NoError(t, err, "Failed to wait for resources")
			}

			// Execute command
			result, err := runCommand(rootDir, tc)
			
			// Validate command just ensures no errors
			if tc.Command == "validate" {
				require.NoError(t, err, "Validate command should not error on valid files")
				return
			}

			require.NoError(t, err, "Failed to run %s command", tc.Command)

			// Assert validation results
			if result != nil {
				validateResults(t, result, tc)
			}
		})
	}
}

// buildKadprobe builds vaptest binary
func buildKadprobe(rootDir string) error {
	vaptestPath := filepath.Join(rootDir, "vaptest")
	buildCmd := exec.Command("go", "build", "-o", vaptestPath, "./cmd/vaptest")
	buildCmd.Dir = rootDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build vaptest: %v\nOutput: %s", err, output)
	}
	return nil
}

// runCommand executes the specified command
func runCommand(rootDir string, tc E2ETestCase) (*TestResult, error) {
	vaptestPath := filepath.Join(rootDir, "vaptest")
	
	var cmd *exec.Cmd
	switch tc.Command {
	case "run":
		testFilePath := tc.TestFile
		if !filepath.IsAbs(testFilePath) {
			testFilePath = filepath.Join(rootDir, "test/e2e", testFilePath)
		}
		cmd = exec.Command(vaptestPath, tc.Command, testFilePath, "--output", "json")
		
	case "check":
		args := []string{"check"}
		
		if tc.UseClusterMode {
			args = append(args, "--cluster")
			args = append(args, "--namespace", testNamespace)
		} else {
			// Specify manifest files for local mode
			for _, manifest := range tc.ManifestFiles {
				manifestPath := filepath.Join(rootDir, "test/e2e", manifest)
				args = append(args, manifestPath)
			}
		}
		
		// Specify policy file
		policyPath := filepath.Join(rootDir, "test/e2e", tc.PolicyFile)
		args = append(args, "--policy", policyPath)
		args = append(args, "--output", "json")
		
		cmd = exec.Command(vaptestPath, args...)
		
	// Validate command was removed
	// case "validate":
	// 	testFilePath := tc.TestFile
	// 	if !filepath.IsAbs(testFilePath) {
	// 		testFilePath = filepath.Join(rootDir, "test/e2e", testFilePath)
	// 	}
	// 	cmd = exec.Command(vaptestPath, "validate", testFilePath)
	// 	// Validate command doesn't output JSON so no result is returned
	// 	cmd.Dir = rootDir
	// 	if output, err := cmd.CombinedOutput(); err != nil {
	// 		return nil, fmt.Errorf("validate command failed: %v\nOutput: %s", err, output)
	// 	}
	// 	return nil, nil
		
	default:
		return nil, fmt.Errorf("unknown command: %s", tc.Command)
	}

	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()

	// Display command and output for debugging
	fmt.Printf("Command: %s\n", cmd.String())
	fmt.Printf("Raw output length: %d bytes\n", len(output))
	
	if err != nil {
		return nil, fmt.Errorf("command failed: %v\nOutput: %s", err, output)
	}

	// Return appropriate error if output is empty
	if len(output) == 0 {
		return nil, fmt.Errorf("command produced no output")
	}

	// Extract JSON part from output
	jsonOutput := extractJSON(string(output))
	if jsonOutput == "" {
		return nil, fmt.Errorf("no JSON found in output: %s", string(output))
	}

	// Parse JSON result
	var result TestResult
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return nil, fmt.Errorf("failed to parse command output: %v\nOutput: %s", err, jsonOutput)
	}

	return &result, nil
}

// validateResults asserts validation results
func validateResults(t *testing.T, result *TestResult, tc E2ETestCase) {
	t.Log("Validating results...")

	// Exclude kubernetes service for check command in cluster mode
	var filteredResults []ResourceResult
	for _, r := range result.Results {
		if tc.Command == "check" && tc.UseClusterMode && r.Name == "services/kubernetes" {
			continue
		}
		filteredResults = append(filteredResults, r)
	}

	actualTotal := len(filteredResults)
	actualSuccessful := 0
	actualFailed := 0

	for _, r := range filteredResults {
		if tc.Command == "run" {
			// Check Success field for run command
			if r.Success {
				actualSuccessful++
			} else {
				actualFailed++
			}
		} else {
			// Check ActualResponse.Allowed for check command
			if r.ActualResponse.Allowed {
				actualSuccessful++
			} else {
				actualFailed++
			}
		}
	}

	// Assert summary
	assert.Equal(t, tc.ExpectedSuccessful+tc.ExpectedFailed, actualTotal,
		"Total resources should match expected")
	assert.Equal(t, tc.ExpectedSuccessful, actualSuccessful,
		"Successful resources should match expected")
	assert.Equal(t, tc.ExpectedFailed, actualFailed,
		"Failed resources should match expected")

	// Assert validation results for each resource (only for check command)
	if tc.Command == "check" && tc.ResourceChecks != nil {
		resourceChecked := make(map[string]bool)

		for _, resource := range filteredResults {
			for resourceKey, expectedAllowed := range tc.ResourceChecks {
				if strings.Contains(resource.Name, resourceKey) {
					resourceChecked[resourceKey] = true

					// Assert expected allowed/denied state
					assert.Equal(t, expectedAllowed, resource.ActualResponse.Allowed,
						"Resource %s allowed state should match expected", resourceKey)

					// Assert denial reason (for denied resources)
					if !expectedAllowed {
						expectedReason := tc.ReasonChecks[resourceKey]
						assert.Equal(t, expectedReason, resource.ActualResponse.Reason,
							"Resource %s denial reason should match expected", resourceKey)
					}
				}
			}
		}

		// Ensure all resources were checked
		for resourceKey := range tc.ResourceChecks {
			assert.True(t, resourceChecked[resourceKey],
				"Resource %s should have been checked", resourceKey)
		}
	}
}

// Below are existing helper functions (ported from verify_test.go)

func getRootDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func switchContext(context string) error {
	cmd := exec.Command("kubectl", "config", "use-context", context)
	return cmd.Run()
}

func initializeClient(context string) (*kubernetes.Clientset, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func cleanupNamespace(clientset *kubernetes.Clientset, namespace string) error {
	// Delete Services
	services, err := clientset.CoreV1().Services(namespace).List(
		context.TODO(),
		metav1.ListOptions{},
	)
	if err != nil {
		return err
	}

	for _, svc := range services.Items {
		if svc.Name == "kubernetes" {
			continue
		}
		err := clientset.CoreV1().Services(namespace).Delete(
			context.TODO(),
			svc.Name,
			metav1.DeleteOptions{},
		)
		if err != nil {
			return err
		}
	}

	// Delete Deployments
	err = clientset.AppsV1().Deployments(namespace).DeleteCollection(
		context.TODO(),
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)
	if err != nil {
		return err
	}

	// Delete Pods
	err = clientset.CoreV1().Pods(namespace).DeleteCollection(
		context.TODO(),
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)
	if err != nil {
		return err
	}

	return waitForNoResources(clientset, namespace)
}

func deployTestResources(rootDir string, resourceFiles []string) error {
	for _, resourceFile := range resourceFiles {
		cmd := exec.Command("kubectl", "apply", "-f", filepath.Join(rootDir, "test/e2e", resourceFile))
		cmd.Dir = rootDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to apply %s: %v\nOutput: %s", resourceFile, err, output)
		}
	}
	return nil
}

func waitForResources(clientset *kubernetes.Clientset, namespace string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for resources to be ready")
		case <-ticker.C:
			deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			allDeploymentsReady := true
			for _, deploy := range deployments.Items {
				if deploy.Status.ReadyReplicas != deploy.Status.Replicas {
					allDeploymentsReady = false
					break
				}
			}

			time.Sleep(5 * time.Second)

			if allDeploymentsReady {
				fmt.Println("All deployments are ready, proceeding with test...")
				return nil
			}
		}
	}
}

func waitForNoResources(clientset *kubernetes.Clientset, namespace string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for resources to be deleted")
		case <-ticker.C:
			deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			customPodCount := 0
			for _, pod := range pods.Items {
				if pod.Name == "privileged-pod" || pod.Name == "non-privileged-pod" ||
					pod.Name == "pod-with-limits" || pod.Name == "pod-without-limits" ||
					pod.Name == "hostpath-pod" || pod.Name == "no-hostpath-pod" {
					customPodCount++
				}
			}

			services, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			customServiceCount := 0
			for _, svc := range services.Items {
				if svc.Name != "kubernetes" {
					customServiceCount++
				}
			}

			if len(deployments.Items) == 0 && customPodCount == 0 && customServiceCount == 0 {
				return nil
			}
		}
	}
}

func extractJSON(output string) string {
	start := strings.Index(output, "{")
	if start == -1 {
		return ""
	}

	depth := 1
	for i := start + 1; i < len(output); i++ {
		if output[i] == '{' {
			depth++
		} else if output[i] == '}' {
			depth--
			if depth == 0 {
				return output[start : i+1]
			}
		}
	}

	return ""
}