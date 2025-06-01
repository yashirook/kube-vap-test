#!/bin/bash

# Stop on error
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEST_DIR="${REPO_ROOT}/examples/tests"

# Color output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Check and setup kind-kind cluster
ensure_kind_cluster() {
  echo -e "${YELLOW}Checking for kind-kind cluster...${NC}"
  
  # Get cluster list
  if ! kubectl config get-contexts kind-kind > /dev/null 2>&1; then
    echo -e "${RED}Error: kind-kind cluster not found. Please create a kind cluster.${NC}"
    echo -e "${YELLOW}Example: kind create cluster --name kind${NC}"
    exit 1
  fi
  
  # Set context
  echo -e "${YELLOW}Switching context to kind-kind...${NC}"
  kubectl config use-context kind-kind
  
  # Check cluster connection
  if ! kubectl cluster-info > /dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to kind-kind cluster. Please check if the cluster is running.${NC}"
    exit 1
  fi
  
  echo -e "${GREEN}Connected to kind-kind cluster.${NC}"
}

# Check and setup kind-kind cluster
ensure_kind_cluster

echo -e "${YELLOW}Starting E2E tests...${NC}"

# Check build status
if [ ! -f "${REPO_ROOT}/kube-vap-test" ]; then
  echo "Building kube-vap-test..."
  (cd "${REPO_ROOT}" && go build -o kube-vap-test ./cmd/kube-vap-test)
fi

# Function for test execution
run_test() {
  local test_file="$1"
  local test_name="$(basename "$test_file" .yaml)"
  
  echo -e "${YELLOW}Running test: ${test_name}${NC}"
  
  # Execute the test using kube-vap-test run command
  local cmd="${REPO_ROOT}/kube-vap-test run $test_file"
  
  echo -e "${YELLOW}Command: $cmd${NC}"
  cmd_output=$(eval "$cmd" 2>&1)
  cmd_status=$?
  echo -e "${YELLOW}Output: $cmd_output${NC}"
  echo -e "${YELLOW}Exit code: $cmd_status${NC}"
  
  if [ $cmd_status -eq 0 ]; then
    echo -e "${GREEN}✓ Test passed: ${test_name}${NC}"
    return 0
  else
    echo -e "${RED}✗ Test failed: ${test_name}${NC}"
    return 1
  fi
}

# Policy manifest validation
check_policy_manifest() {
  local policy_file="$1"
  local manifest_file="$2"
  local expected_result="$3" # "allowed" or "denied"
  local policy_name="$(basename "$policy_file" -policy.yaml)"
  local manifest_name="$(basename "$manifest_file")"
  
  echo -e "${YELLOW}Validating manifest: ${manifest_name} (${expected_result})${NC}"
  
  # Read manifest file contents
  manifest_content=$(cat "$manifest_file")
  
  # Create temporary test file
  temp_test_file=$(mktemp)
  cat > "$temp_test_file" <<EOL
apiVersion: admission.k8s.io/v1
kind: ValidatingAdmissionPolicyTest
metadata:
  name: single-test
spec:
  source:
    type: local
    files:
      - "$policy_file"
  testCases:
    - name: "test-case"
      operation: CREATE
      expected:
        allowed: $([ "$expected_result" == "allowed" ] && echo "true" || echo "false")
      object:
EOL
  
  # Add manifest content with proper indentation
  echo "$manifest_content" | sed 's/^/        /' >> "$temp_test_file"
  
  echo -e "${YELLOW}Command: ${REPO_ROOT}/kube-vap-test run \"$temp_test_file\"${NC}"
  cmd_output=$("${REPO_ROOT}/kube-vap-test" run "$temp_test_file" 2>&1)
  cmd_status=$?
  echo -e "${YELLOW}Output: $cmd_output${NC}"
  echo -e "${YELLOW}Exit code: $cmd_status${NC}"
  
  # Determine if rejected from output
  # Check output content as well as exit code
  if [[ $cmd_output == *"ImageTagPolicy"* || $cmd_output == *"ResourceLimitsPolicy"* || $cmd_output == *"PrivilegedContainerPolicy"* || $cmd_output == *"HostPathVolumePolicy"* ]]; then
    # Policy violation detected
    if [ "$expected_result" == "denied" ]; then
      echo -e "${GREEN}✓ Validation passed: ${manifest_name} (denied)${NC}"
      return 0
    else
      echo -e "${RED}✗ Validation failed: ${manifest_name} (should have been allowed but was denied)${NC}"
      return 1
    fi
  else
    # No policy violation detected
    if [ "$expected_result" == "allowed" ]; then
      echo -e "${GREEN}✓ Validation passed: ${manifest_name} (allowed)${NC}"
      return 0
    else
      echo -e "${RED}✗ Validation failed: ${manifest_name} (should have been denied but was allowed)${NC}"
      return 1
    fi
  fi
}

# Cluster resource verification
verify_cluster() {
  local policy_file="$1"
  local namespace="$2"
  local expected_status="$3" # "success" or "failure"
  local policy_name="$(basename "$policy_file" -policy.yaml)"
  
  echo -e "${YELLOW}Verifying cluster resources: ${policy_name} (namespace: ${namespace})${NC}"
  
  # Execute check command (formerly verify) - changed to display output
  local cmd="${REPO_ROOT}/kube-vap-test check --cluster --policy \"$policy_file\" --namespace \"$namespace\" --output json"
  echo -e "${YELLOW}Command: $cmd${NC}"
  cmd_output=$(eval "$cmd" 2>&1)
  cmd_status=$?
  echo -e "${YELLOW}Output: $cmd_output${NC}"
  echo -e "${YELLOW}Exit code: $cmd_status${NC}"
  
  # Parse results from JSON output (skip Info lines)
  local json_output=$(echo "$cmd_output" | grep -v "^Info:" | jq . 2>/dev/null)
  if [ $? -eq 0 ] && [ -n "$json_output" ]; then
    # Valid JSON, check summary
    local failed_count=$(echo "$cmd_output" | grep -v "^Info:" | jq -r '.summary.failed // 0')
    local total_count=$(echo "$cmd_output" | grep -v "^Info:" | jq -r '.summary.total // 0')
    
    
    if [[ "$expected_status" == "success" && "$failed_count" == "0" && "$total_count" != "0" ]]; then
      echo -e "${GREEN}✓ Verification passed: ${policy_name} (all resources passed validation)${NC}"
      return 0
    elif [[ "$expected_status" == "failure" && "$failed_count" != "0" ]]; then
      echo -e "${GREEN}✓ Verification passed: ${policy_name} (some resources failed validation - as expected)${NC}"
      return 0
    else
      echo -e "${RED}✗ Verification failed: ${policy_name} (unexpected result: failed=$failed_count, expected=$expected_status)${NC}"
      return 1
    fi
  else
    # Invalid JSON, use exit code as before
    if [[ "$expected_status" == "success" && $cmd_status -eq 0 ]]; then
      echo -e "${GREEN}✓ Verification passed: ${policy_name} (all resources passed validation)${NC}"
      return 0
    elif [[ "$expected_status" == "failure" && $cmd_status -ne 0 ]]; then
      echo -e "${GREEN}✓ Verification passed: ${policy_name} (some resources failed validation - as expected)${NC}"
      return 0
    else
      echo -e "${RED}✗ Verification failed: ${policy_name} (unexpected result)${NC}"
      return 1
    fi
  fi
}

# Function to setup cluster test resources
setup_cluster_resources() {
  echo -e "${YELLOW}Setting up test cluster resources...${NC}"
  
  # Create test pods
  echo -e "${YELLOW}Creating test pods...${NC}"
  kubectl apply -f "${REPO_ROOT}/examples/manifests/allowed-pod-no-hostpath.yaml" -n default 2>&1 || echo "Failed to create pod"
  kubectl apply -f "${REPO_ROOT}/examples/manifests/allowed-pod-with-resource-limits.yaml" -n default 2>&1 || echo "Failed to create pod"
  
  # Create test services (including service without app:label)
  echo -e "${YELLOW}Creating test services...${NC}"
  
  # Create temporary file for service without app:label
  temp_service_file=$(mktemp)
  cat > "$temp_service_file" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: test-service-no-label
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: non-existent
EOF
  kubectl apply -f "$temp_service_file" -n default 2>&1 || echo "Failed to create service"
  rm "$temp_service_file"
  
  # Create temporary file for service with app:label
  temp_service_file=$(mktemp)
  cat > "$temp_service_file" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: test-service-with-label
  labels:
    app: test-app
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: test-app
EOF
  kubectl apply -f "$temp_service_file" -n default 2>&1 || echo "Failed to create service"
  rm "$temp_service_file"
  
  # Wait for all resources to be created
  echo -e "${YELLOW}Waiting for resources to be created...${NC}"
  sleep 5
  
  # Verify created resources
  echo -e "${YELLOW}Created resources:${NC}"
  kubectl get pods,svc -n default
}

# Function to cleanup cluster test resources
cleanup_cluster_resources() {
  echo -e "${YELLOW}Cleaning up test cluster resources...${NC}"
  
  # Delete test pods
  kubectl delete pod nginx-no-hostpath -n default --ignore-not-found=true 2>&1 || echo "Failed to delete pod"
  kubectl delete pod nginx-with-limits -n default --ignore-not-found=true 2>&1 || echo "Failed to delete pod"
  
  # Delete other existing test pods
  kubectl delete pod hostpath-pod -n default --ignore-not-found=true 2>&1 || echo "Failed to delete pod"
  kubectl delete pod no-hostpath-pod -n default --ignore-not-found=true 2>&1 || echo "Failed to delete pod"
  
  # Delete test services
  kubectl delete service test-service-no-label -n default --ignore-not-found=true 2>&1 || echo "Failed to delete service"
  kubectl delete service test-service-with-label -n default --ignore-not-found=true 2>&1 || echo "Failed to delete service"
  
  echo -e "${YELLOW}Waiting for resource deletion...${NC}"
  sleep 3
  
  # Verify resources after deletion
  echo -e "${YELLOW}Resources after deletion:${NC}"
  kubectl get pods,svc -n default
}

# Function to validate test file dependencies
validate_test_dependencies() {
  local test_file="$1"
  
  # Extract policy files referenced in test file (considering YAML structure)
  local policy_files=$(grep -A 20 "files:" "$test_file" | grep -E "^\s+- [\"']?\./")
  
  # Skip if file not found
  while IFS= read -r line; do
    if [[ -n "$line" ]]; then
      # Extract path (remove quotes and hyphen)
      local policy_file=$(echo "$line" | sed 's/^[[:space:]]*-[[:space:]]*//' | sed 's/^[\"'"'"']//' | sed 's/[\"'"'"'].*$//')
      local abs_policy_path="${REPO_ROOT}/${policy_file#./}"
      if [[ ! -f "$abs_policy_path" ]]; then
        echo -e "${YELLOW}Skipping: $(basename "$test_file") (dependency not found: $policy_file)${NC}"
        return 1
      fi
    fi
  done <<< "$policy_files"
  
  return 0
}

# Execute all test cases
FAILED=0
for test_file in "${TEST_DIR}"/*.yaml; do
  # Skip cluster-policies-test.yaml as it requires policies deployed to cluster
  if [[ "$(basename "$test_file")" == "cluster-policies-test.yaml" ]]; then
    echo -e "${YELLOW}Skipping: cluster-policies-test (requires cluster policies)${NC}"
    continue
  fi
  
  # Validate test file dependencies
  if ! validate_test_dependencies "$test_file"; then
    continue
  fi
  
  if ! run_test "$test_file"; then
    FAILED=$((FAILED+1))
  fi
done

# Policy and manifest combination tests
echo -e "${YELLOW}Manifest validation tests...${NC}"

# Policy and manifest test set array
POLICY_TEST_SETS=(
  "${REPO_ROOT}/examples/policies/no-latest-tag-policy.yaml|allowed|${REPO_ROOT}/examples/manifests/allowed-pod-specific-tag.yaml"
  "${REPO_ROOT}/examples/policies/no-latest-tag-policy.yaml|denied|${REPO_ROOT}/examples/manifests/denied-pod-latest-tag.yaml"
  "${REPO_ROOT}/examples/policies/resource-limits-policy.yaml|allowed|${REPO_ROOT}/examples/manifests/allowed-pod-with-resource-limits.yaml"
  "${REPO_ROOT}/examples/policies/resource-limits-policy.yaml|denied|${REPO_ROOT}/examples/manifests/denied-pod-without-resource-limits.yaml"
  "${REPO_ROOT}/examples/policies/no-privileged-policy.yaml|allowed|${REPO_ROOT}/examples/manifests/allowed-pod-no-privileged.yaml"
  "${REPO_ROOT}/examples/policies/no-privileged-policy.yaml|denied|${REPO_ROOT}/examples/manifests/denied-pod-privileged.yaml"
  "${REPO_ROOT}/examples/policies/no-hostpath-policy.yaml|allowed|${REPO_ROOT}/examples/manifests/allowed-pod-no-hostpath.yaml"
  "${REPO_ROOT}/examples/policies/no-hostpath-policy.yaml|denied|${REPO_ROOT}/examples/manifests/denied-pod-with-hostpath.yaml"
)

current_policy=""
for test_set in "${POLICY_TEST_SETS[@]}"; do
  # Expand fields separated by |
  IFS='|' read -r policy_file expected_result manifest_file <<< "$test_set"
  
  # Display header for new policy
  if [ "$current_policy" != "$policy_file" ]; then
    current_policy="$policy_file"
    policy_name="$(basename "$policy_file" -policy.yaml)"
    echo -e "${YELLOW}Policy test: ${policy_name}${NC}"
  fi
  
  if ! check_policy_manifest "$policy_file" "$manifest_file" "$expected_result"; then
    FAILED=$((FAILED+1))
  fi
done

# Cluster resource verification tests
echo -e "${YELLOW}Cluster resource verification tests...${NC}"

# Clean up resources before starting for a clean state
cleanup_cluster_resources

# Setup test resources
setup_cluster_resources

# verify command test set array
VERIFY_TEST_SETS=(
  "${REPO_ROOT}/examples/policies/multi-resource-policy.yaml|default|failure"
  "${REPO_ROOT}/examples/policies/no-latest-tag-policy.yaml|default|success"
  "${REPO_ROOT}/examples/policies/resource-limits-policy.yaml|kube-system|failure"
)

for test_set in "${VERIFY_TEST_SETS[@]}"; do
  # Expand fields separated by |
  IFS='|' read -r policy_file namespace expected_status <<< "$test_set"
  
  if ! verify_cluster "$policy_file" "$namespace" "$expected_status"; then
    FAILED=$((FAILED+1))
  fi
done

# Clean up resources after test completion
cleanup_cluster_resources

# Output results
echo -e "${YELLOW}=== Test Results ===${NC}"
if [ $FAILED -gt 0 ]; then
  echo -e "${RED}${FAILED} tests failed${NC}"
  exit 1
else
  echo -e "${GREEN}All tests passed!${NC}"
  exit 0
fi 