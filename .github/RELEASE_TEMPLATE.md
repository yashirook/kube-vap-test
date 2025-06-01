# Release Notes for kube-vap-test v{{.Version}}

## Overview
This release of kube-vap-test includes [brief description of major changes].

## What's Changed

### Features
<!-- List new features added in this release -->
- Feature 1
- Feature 2

### Bug Fixes
<!-- List bug fixes included in this release -->
- Fix 1
- Fix 2

### Improvements
<!-- List improvements and enhancements -->
- Improvement 1
- Improvement 2

### Breaking Changes
<!-- List any breaking changes that require user action -->
- None

## Installation

### Binary Downloads
Download the pre-compiled binaries from the [releases page](https://github.com/yashirook/kube-vap-test/releases/tag/v{{.Version}}).

### Using curl (Linux/macOS)
```bash
# Linux (x86_64)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v{{.Version}}/kube-vap-test_{{.Version}}_Linux_x86_64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/

# macOS (x86_64)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v{{.Version}}/kube-vap-test_{{.Version}}_Darwin_x86_64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/

# macOS (arm64)
curl -L https://github.com/yashirook/kube-vap-test/releases/download/v{{.Version}}/kube-vap-test_{{.Version}}_Darwin_arm64.tar.gz | tar xz
sudo mv kube-vap-test /usr/local/bin/
```

### Using go install
```bash
go install github.com/yashirook/kube-vap-test/cmd/kube-vap-test@v{{.Version}}
```

## Checksums
Please verify the checksums of downloaded files against the `checksums.txt` file provided in the release assets.

## Documentation
For detailed documentation and examples, please visit:
- [README](https://github.com/yashirook/kube-vap-test/blob/v{{.Version}}/README.md)
- [Examples](https://github.com/yashirook/kube-vap-test/tree/v{{.Version}}/examples)

## Contributors
Thanks to all contributors who made this release possible!

## Full Changelog
For a complete list of changes, see the [CHANGELOG](https://github.com/yashirook/kube-vap-test/blob/v{{.Version}}/CHANGELOG.md).