# kaptest Project Overview

## Project Overview

kaptest is a tool that simplifies testing of Kubernetes ValidatingAdmissionPolicy. By testing policies locally before deploying them to a cluster, it enables safe and efficient policy management.

## Key Features

### 1. Policy Test Execution

- **Test Suite Execution**: Validates policies based on defined test cases
- **Cluster Integration**: Can fetch policies from cluster for testing
- **Local Testing**: Test using only local files without cluster connection

### 2. Direct Manifest Validation

- **Single/Multiple File Validation**: Validates one or multiple manifest files simultaneously
- **Operation Specification**: Validates with operation types like CREATE/UPDATE/DELETE
- **Result Display**: Shows test results in an easy-to-understand format

### 3. Test Definition Validation

- **Syntax Validation**: Validates syntax of test definition files
- **Reference Integrity Check**: Confirms validity of file paths

### 4. E2E Test Automation

- **Automated Testing**: Automated test execution for all policies
- **CI/CD Support**: Can integrate with GitHub Actions and other CI/CD tools

## Implemented Policies

1. **No Latest Tag Policy** - Ensures image stability
2. **Resource Limits Required Policy** - Protects cluster resources
3. **No Privileged Container Policy** - Enhances security
4. **HostPath Usage Restriction Policy** - Improves data isolation and security

## Technical Features

- **Go Implementation**: Fast and efficient processing
- **CEL Expression Support**: Full support for Common Expression Language
- **Flexible Output Formats**: Results output in table/JSON/YAML formats
- **Error Handling**: Detailed error messages and debugging support

## Development Status

Currently completed features:

- Basic test execution engine
- Direct manifest validation command
- Multiple policy type implementation and validation
- E2E test automation framework

Features in development:

- WebUI for policy testing
- Additional policy templates
- Package management integration

## Use Cases

- **Development Phase**: Local testing during policy development
- **CI/CD**: Continuous validation through automated testing
- **Auditing**: Verification and documentation of cluster policies
- **Education**: Learning tool for Kubernetes ValidatingAdmissionPolicy 