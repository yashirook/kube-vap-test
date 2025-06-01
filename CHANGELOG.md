# Changelog

All notable changes to kube-vap-test will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.31.0] - 2024-05-30

### Added
- **Initial release** of kube-vap-test - ValidatingAdmissionPolicy Test Tool
- Full support for Kubernetes ValidatingAdmissionPolicy testing
- Support for Kubernetes 1.28+ ValidatingAdmissionPolicy (GA features)
- Support for Kubernetes 1.30+ features:
  - Variables for reusable expressions
  - MessageExpression for dynamic error messages  
  - MatchConditions for advanced policy filtering
- Support for Kubernetes 1.31+ CEL extensions:
  - IP/CIDR validation functions (`ip()`, `cidr()`, `isIP()`, `isCIDR()`, etc.)
  - Format validation functions (`format.dns1123Label()`, `format.uuid()`, etc.)
- Commands:
  - `run` - Execute ValidatingAdmissionPolicy test definitions
  - `check` - Validate resources against policies
  - `validate` - Syntax validation for policies and test files
  - `scan` - Scan multiple resources against policies
- Multiple source support: local files and Kubernetes cluster
- Development mode with `--skip-bindings` flag
- Parameter support for parameterized policies
- Multiple output formats: table, JSON, YAML
- Comprehensive test framework with ValidatingAdmissionPolicyTest CRD
- Progressive loading indicators for better user experience

### Features
- Native Kubernetes resource format support
- Advanced CEL expression evaluation
- Cluster mode for testing with deployed policies
- Local mode for development and CI/CD
- Comprehensive error handling and user-friendly messages
- Examples and documentation for common use cases