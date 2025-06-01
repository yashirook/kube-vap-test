package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/yashirook/kube-vap-test/internal/engine"
	"github.com/yashirook/kube-vap-test/internal/loader"
	"github.com/yashirook/kube-vap-test/internal/reporter"
	vaptestv1 "github.com/yashirook/kube-vap-test/pkg/apis/admission/v1"
)

// RunOptions represents options for run command
type RunOptions struct {
	CommonOptions
	Cluster      bool
	SkipBindings bool
}

// NewRunCommand creates a new run command
func NewRunCommand(opts *RunOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [test-files...]",
		Short: "Run ValidatingAdmissionPolicy test definitions",
		Long:  `Run ValidatingAdmissionPolicy test definitions based on specified test files`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Do not show usage on errors
			cmd.SilenceUsage = true

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

			// Validate format
			format := reporter.OutputFormat(opts.OutputFormat)
			if format != reporter.OutputFormatTable &&
				format != reporter.OutputFormatJSON &&
				format != reporter.OutputFormatYAML {
				return fmt.Errorf("Invalid output format: %s (valid values: table, json, yaml)", opts.OutputFormat)
			}

			// Initialize reporter
			rep := reporter.NewReporter(format, opts.Verbose)

			// Initialize simulator
			simulator, err := engine.NewPolicySimulator()
			if err != nil {
				return fmt.Errorf("Failed to initialize policy simulator: %w", err)
			}


			// Process each test file
			var lastErr error
			for _, testFilePath := range args {
				if err := runTestFile(ctx, testFilePath, simulator, rep, opts); err != nil {
					lastErr = err
				}
			}

			return lastErr
		},
	}

	// Command-specific flags
	cmd.Flags().BoolVar(&opts.Cluster, "cluster", false, "Run tests in cluster mode")
	cmd.Flags().BoolVar(&opts.SkipBindings, "skip-bindings", false, "Skip policy bindings and test policy logic only")

	return cmd
}

func runTestFile(ctx context.Context, testFilePath string, simulator *engine.PolicySimulator, rep reporter.Reporter, opts *RunOptions) error {
	testFilePath = filepath.Clean(testFilePath)

	if !opts.Quiet {
		reporter.PrintInfo(fmt.Sprintf("Processing test file: %s", testFilePath))
	}

	// Initialize local resource loader to load test definition
	initialLoader, err := loader.NewLocalResourceLoader()
	if err != nil {
		reporter.PrintError(fmt.Errorf("Failed to initialize resource loader: %w", err))
		return err
	}

	// Load test definition
	test, err := initialLoader.LoadPolicyTest(testFilePath, true)
	if err != nil {
		reporter.PrintError(fmt.Errorf("Failed to load test definition: %w", err))
		return err
	}

	// Initialize resource loader based on source configuration
	var resourceLoader loader.ResourceLoader
	var resourceSource loader.ResourceSource

	// Determine source type
	sourceType := test.Spec.Source.Type
	if sourceType == "" {
		sourceType = vaptestv1.SourceTypeLocal // default to local
	}

	switch sourceType {
	case vaptestv1.SourceTypeLocal:
		// Use resources from local files
		resourceLoader, err = loader.NewLocalResourceLoader()
		if err != nil {
			reporter.PrintError(fmt.Errorf("Failed to initialize local resource loader: %w", err))
			return err
		}
		resourceSource = loader.ResourceSource{
			Type:  loader.SourceTypeLocal,
			Files: test.Spec.Source.Files,
		}
	case vaptestv1.SourceTypeCluster:
		// Use resources from cluster
		resourceLoader, err = loader.NewClusterResourceLoader(opts.Kubeconfig)
		if err != nil {
			reporter.PrintError(fmt.Errorf("Failed to initialize cluster resource loader: %w", err))
			return err
		}
		resourceSource = loader.ResourceSource{
			Type:           loader.SourceTypeCluster,
			KubeconfigPath: opts.Kubeconfig,
		}
	default:
		reporter.PrintError(fmt.Errorf("Invalid source type: %s", sourceType))
		return fmt.Errorf("Invalid source type: %s", sourceType)
	}

	// Load parameters (optional)
	var paramObj runtime.Object
	if test.Spec.IncludeParameters {
		// Load parameters from the same source
		paramObj, err = resourceLoader.LoadParameter(resourceSource)
		if err != nil {
			reporter.PrintWarning(fmt.Sprintf("Failed to load parameters: %s", err.Error()))
		}
	}

	// Load policies based on source
	policies, err := resourceLoader.LoadPolicies(resourceSource)
	if err != nil {
		reporter.PrintError(fmt.Errorf("Failed to load policies: %w", err))
		return err
	}

	// Check if policies exist
	if len(policies) == 0 {
		reporter.PrintError(fmt.Errorf("No policies were loaded"))
		return fmt.Errorf("No policies were loaded")
	}

	// Load policy bindings unless --skip-bindings is specified
	var bindings []*admissionregistrationv1.ValidatingAdmissionPolicyBinding
	if !opts.SkipBindings {
		bindings, err = resourceLoader.LoadPolicyBindings(resourceSource)
		if err != nil {
			// Warn but continue - bindings might be optional
			reporter.PrintWarning(fmt.Sprintf("Failed to load policy bindings: %s", err.Error()))
		}
		
		if !opts.Quiet && len(bindings) > 0 {
			reporter.PrintInfo(fmt.Sprintf("Loaded %d policy bindings", len(bindings)))
		}
	} else {
		if !opts.Quiet {
			reporter.PrintInfo("Skipping policy bindings (--skip-bindings flag)")
		}
	}

	// Execute tests based on whether we have bindings
	var status *vaptestv1.ValidatingAdmissionPolicyTestStatus
	if len(bindings) > 0 && !opts.SkipBindings {
		// Execute tests with policies and bindings
		status, err = simulator.RunPolicyTestsWithBindings(ctx, policies, bindings, paramObj, test.Spec.TestCases)
	} else {
		// Execute tests with policies only (backward compatible)
		status, err = simulator.RunPolicyTestsWithMultiPolicies(ctx, policies, paramObj, test.Spec.TestCases)
	}
	
	if err != nil {
		reporter.PrintError(fmt.Errorf("Failed to execute test: %w", err))
		return err
	}

	// Report test results
	if err := rep.Report(status); err != nil {
		reporter.PrintError(fmt.Errorf("Failed to report results: %w", err))
		return err
	}

	// Return error code if any tests failed
	if status.Summary.Failed > 0 {
		return fmt.Errorf("%d tests failed", status.Summary.Failed)
	}

	return nil
}