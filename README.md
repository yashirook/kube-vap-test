# kube-vap-test - ValidatingAdmissionPolicy Test Tool

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.28+-blue.svg)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A comprehensive testing and validation tool for Kubernetes ValidatingAdmissionPolicy.

## Requirements

- **Kubernetes**: 1.28+ (ValidatingAdmissionPolicy GA)
- **Go**: 1.21+ (for building from source)

kube-vap-test supports all Kubernetes 1.30+ ValidatingAdmissionPolicy features including variables, messageExpression, matchConditions, and Kubernetes 1.31 CEL libraries (IP/CIDR, format functions).

## Overview

kube-vap-test simplifies the development, testing, and validation of Kubernetes ValidatingAdmissionPolicy. It enables you to test your policies locally before deploying them to a cluster, ensuring they work as expected.

## Features

- **Native Kubernetes Support**: Uses native ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding formats
- **Comprehensive Testing**: Define test cases using custom resource (ValidatingAdmissionPolicyTest)
- **Multiple Source Support**: Load policies from local files or directly from a Kubernetes cluster
- **Development Mode**: Use `--skip-bindings` to test policy logic without bindings
- **Advanced CEL Support**: 
  - Full CEL expression evaluation with Kubernetes 1.30+ features
  - Support for variables, messageExpression, and matchConditions
  - Kubernetes 1.31 CEL libraries (IP/CIDR, format functions)
- **Flexible Output**: Multiple output formats (table, JSON, YAML)
- **Parameter Support**: Full support for parameterized policies
- **Resource Scanning**: Scan multiple resources against policies
- **Validation**: Syntax validation for policies and test files

## Installation

### Homebrew (macOS/Linux) - Recommended

```bash
# Add tap and install
brew tap yashirook/tap
brew install kube-vap-test
```

### GitHub Releases (All platforms)

Download the latest release for your platform from [GitHub Releases](https://github.com/yashirook/kube-vap-test/releases):

```bash
# Linux (x86_64)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v1.31.0/kube-vap-test_1.31.0_Linux_x86_64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v1.31.0/kube-vap-test_1.31.0_Darwin_x86_64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v1.31.0/kube-vap-test_1.31.0_Darwin_arm64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/

# Windows (PowerShell)
# Download from https://github.com/yashirook/kube-vap-test/releases/download/v1.31.0/kube-vap-test_1.31.0_Windows_x86_64.zip
# Extract and add to PATH
```

### Go Install (requires Go 1.21+)

```bash
go install github.com/yashirook/kube-vap-test/cmd/kube-vap-test@latest
```

### As kubectl Plugin

```bash
# Install as kubectl plugin
git clone https://github.com/yashirook/kube-vap-test.git
cd kube-vap-test
make install-kubectl-plugin

# Now you can use it as:
kubectl kube-vap-test run examples/tests/no-latest-tag-test.yaml
kubectl kube-vap-test check --policy examples/policies/no-latest-tag-policy.yaml examples/manifests/allowed-pod-specific-tag.yaml
```

### Building from Source

```bash
git clone https://github.com/yashirook/kube-vap-test.git
cd kube-vap-test
make build
# Binary will be available at ./bin/kube-vap-test
```

## Usage

### Basic Usage

```bash
# Run test definitions
kube-vap-test run examples/tests/no-latest-tag-test.yaml

# Run tests with policy logic only (skip bindings)
kube-vap-test run examples/tests/no-latest-tag-test.yaml --skip-bindings

# Run tests with policies and bindings
kube-vap-test run examples/tests/no-hostpath-with-binding-test.yaml

# Specify output format
kube-vap-test run --output json examples/tests/no-latest-tag-test.yaml

# Check manifest files against policies
kube-vap-test check examples/manifests/allowed-pod-specific-tag.yaml --policy examples/policies/no-latest-tag-policy.yaml

# Check multiple manifest files
kube-vap-test check examples/manifests/*.yaml --policy examples/policies/no-latest-tag-policy.yaml

# Check against multiple policies
kube-vap-test check examples/manifests/test-pod.yaml \
  --policy examples/policies/no-latest-tag-policy.yaml \
  --policy examples/policies/resource-limits-policy.yaml

# Check resources in cluster
kube-vap-test check --cluster --namespace default --policy examples/policies/no-latest-tag-policy.yaml
```

### CLI Commands and Options

```
kube-vap-test [command] [options] [arguments...]

Commands:
  run         Run ValidatingAdmissionPolicy test definitions
  check       Check resources against policies
  version     Show version information
  help        Show help

Global Options:
  --output, -o     Output format (table, json, yaml)
  --quiet          Suppress progress information
  --kubeconfig     Path to kubeconfig file (default: ~/.kube/config)
  --verbose, -v    Show detailed output

Run Command Options:
  --skip-bindings  Skip policy bindings and test policy logic only

Check Command Options:
  --cluster, -c        Run in cluster mode (fetch resources from cluster)
  --namespace, -n      Namespace to check (cluster mode)
  --policy             Policy files to use (required, can specify multiple)
  --param              Parameter file for policies (optional)
  --operation          Operation to validate (CREATE, UPDATE, DELETE) (default: CREATE)
```

## Test Definition Files

Define tests using ValidatingAdmissionPolicyTest resources.

### Basic Test (Policy Only)

```yaml
apiVersion: admission.k8s.io/v1
kind: ValidatingAdmissionPolicyTest
metadata:
  name: no-latest-tag-test
spec:
  source:
    type: local  # default, can be omitted
    files:
      - "examples/policies/no-latest-tag-policy.yaml"
  testCases:
  - name: "allowed-pod-with-specific-tag"
    description: "Pod with specific tag should be allowed"
    object:
      apiVersion: v1
      kind: Pod
      metadata:
        name: allowed-pod
      spec:
        containers:
        - name: nginx
          image: nginx:1.21.0
    operation: CREATE
    expected:
      allowed: true
  - name: "denied-pod-with-latest-tag"
    description: "Pod with latest tag should be denied"
    object:
      apiVersion: v1
      kind: Pod
      metadata:
        name: denied-pod
      spec:
        containers:
        - name: nginx
          image: nginx:latest
    operation: CREATE
    expected:
      allowed: false
      reason: "ImageTagPolicy"
      messageContains: "Using the 'latest' tag is not allowed"
```

### Test with Policy and Binding

```yaml
apiVersion: admission.k8s.io/v1
kind: ValidatingAdmissionPolicyTest
metadata:
  name: no-hostpath-with-binding-test
spec:
  source:
    type: local
    files:
      - "examples/kube/no-hostpath-policy.yaml"
      - "examples/kube/no-hostpath-policy-binding.yaml"
  testCases:
  - name: "denied-pod-with-hostpath"
    description: "Pods with hostPath volumes are denied"
    object:
      apiVersion: v1
      kind: Pod
      metadata:
        name: test-pod
        namespace: default
      spec:
        containers:
        - name: nginx
          image: nginx:1.21.0
        volumes:
        - name: host-vol
          hostPath:
            path: /tmp
    operation: CREATE
    expected:
      allowed: false
      reason: "Forbidden"
```

### Test with Parameters

```yaml
apiVersion: admission.k8s.io/v1
kind: ValidatingAdmissionPolicyTest
metadata:
  name: parameterized-policy-test
spec:
  source:
    type: local
    files:
      - "examples/policies/parameterized-policy.yaml"
      - "examples/policies/parameterized-policy-binding.yaml"
      - "examples/parameters/allowed-registries.yaml"
  includeParameters: true  # Load parameters from the same source
  testCases:
  - name: "allowed-image-from-approved-registry"
    object:
      apiVersion: v1
      kind: Pod
      spec:
        containers:
        - name: app
          image: docker.io/nginx:1.21.0
    expected:
      allowed: true
```

## Cluster Mode

kube-vap-test supports both "local mode" (loading policies from local files) and "cluster mode" (using policies deployed to the cluster).

To use cluster mode, specify in the test definition file:

```yaml
spec:
  source:
    type: cluster  # Load all VAPs and VAPBs from cluster
```

In cluster mode, all ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding resources deployed to the cluster are automatically used. You can specify the kubeconfig file path with the `--kubeconfig` option. If not specified, the default `~/.kube/config` is used.

## Development Mode

When developing policies, you can use the `--skip-bindings` flag to test only the policy logic without evaluating bindings:

```bash
# Test only CEL expressions in the policy
kube-vap-test run examples/tests/policy-logic-only-test.yaml --skip-bindings
```

This is useful for:
- Rapid development and iteration on policy logic
- Testing CEL expressions without namespace or resource matching constraints
- Debugging policy validation rules in isolation

## Sample Policies

The project includes the following sample policies:

### Policies without Bindings (examples/policies/)

1. **No Latest Tag Policy** (`no-latest-tag-policy.yaml`)
   - Prohibits the use of `latest` tag in container images
   - Encourages the use of stable version tags

2. **Resource Limits Policy** (`resource-limits-policy.yaml`)
   - Enforces memory and CPU resource limits on all containers
   - Ensures predictability and safety of resource usage

3. **No Privileged Policy** (`no-privileged-policy.yaml`)
   - Prohibits running containers in privileged mode
   - Reduces security risks

### Policies with Bindings (examples/kube/)

These examples demonstrate full ValidatingAdmissionPolicy setup with bindings:

1. **No HostPath Policy** (`no-hostpath-policy.yaml` + `no-hostpath-policy-binding.yaml`)
   - Policy: Prohibits direct mounting of host filesystem
   - Binding: Excludes system namespaces (kube-system, kube-public, kube-node-lease)

2. **No Privileged Policy** (`no-privileged-policy.yaml` + `no-privileged-policy-binding.yaml`)
   - Policy: Prohibits running containers in privileged mode
   - Binding: Applies to specific namespaces with validation actions

3. **Resource Limits Policy** (`resource-limits-policy.yaml` + `resource-limits-policy-binding.yaml`)
   - Policy: Enforces resource limits on containers
   - Binding: Configures namespace selection and validation actions

### Advanced Examples

1. **Kubernetes 1.30 Features** (`k8s-1.30-advanced-policy.yaml`)
   - Demonstrates variables, messageExpression, and matchConditions
   - Shows complex policy composition

2. **Kubernetes 1.31 Features** (`k8s-1.31-ip-cidr-policy.yaml`)
   - IP/CIDR validation functions
   - Format validation functions

3. **Complex MatchConditions** (`complex-matchconditions-policy.yaml`)
   - Advanced filtering based on namespace, labels, and annotations
   - Multi-condition policy application

4. **Error Handling** (`error-handling-policy.yaml`)
   - Safe field access patterns
   - Type checking and null safety

## Supported Variables and Features

kube-vap-test supports the following CEL variables and features:

1. **Basic Variables**:
   - `object` - The resource object being created or updated
   - `oldObject` - The resource object before update (for UPDATE operations)
   - `operation` - Operation type (CREATE, UPDATE, DELETE)
   - `namespaceObject` - Namespace information of the object

2. **Custom Variable Definition**:
   ```yaml
   variables:
   - name: myVar
     expression: "object.metadata.name"
   ```

3. **CEL Functions and Features**:
   - Logical operators: `&&`, `||`, `!`
   - Comparison operators: `==`, `!=`, `>`, `<`, `>=`, `<=`
   - String operations: `startsWith()`, `endsWith()`, `contains()`
   - Collection operations: `size()`, `map()`, `filter()`, `all()`, `exists()`
   - Type checking: `has()`, `type()`, etc.

## Limitations

kube-vap-test currently has the following limitations:

1. **Unsupported CEL Variables**:
   - `request.requestResource` - Request resource information
   - `request.subResource` - Subresource information  
   - `request.requestSubResource` - Request subresource information
   - `request.options` - Request options
   - `userInfo` - User information (can be specified in `testCases[].userInfo` for testing)
   - `authorizer` - Authorization information
   - `authorizer.requestResource` - Not available in messageExpression

2. **Fixed Variables**:
   - `request.dryRun` - Always set to `true` as this is a testing tool

3. **Notes**:
   - Type checking for CRDs is limited to basic validation
   - Webhook timeout simulation is not supported
   - Failure policies are not simulated

Please consider these limitations when designing policies. For production use, always test policies in a real Kubernetes cluster.

## License

This project is released under the MIT License. See the [LICENSE](LICENSE) file for details.