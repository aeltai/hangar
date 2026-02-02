<div align="center">
  <h1>Hangar</h1>
  <p>
    <a href="https://build.opensuse.org/package/show/home:StarryWang/hangar"><img src="https://build.opensuse.org/projects/home:StarryWang/packages/hangar/badge.svg?type=default"></a>
    <a href="https://aur.archlinux.org/packages/hangar"><img src="https://img.shields.io/aur/version/hangar"></a>
    <a href="https://goreportcard.com/report/github.com/rancher/hangar"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/rancher/hangar"></a>
    <a href="https://github.com/rancher/hangar/releases"><img alt="GitHub release" src="https://img.shields.io/github/v/release/rancher/hangar?color=default&label=release&logo=github"></a>
    <a href="https://github.com/rancher/hangar/releases"><img alt="GitHub pre-release" src="https://img.shields.io/github/v/release/rancher/hangar?include_prereleases&label=pre-release&logo=github"></a>
    <img alt="License" src="https://img.shields.io/badge/License-Apache_2.0-blue.svg">
  </p>
</div>

> English

Hangar is a command line utility for container images with the following features:

- Multi-platform container images.
- Copy container images between registry servers.
- Export container images as archive files and import them into image repositories.
- Sign container images with sigstore key-pairs.
- Scan container image vulnerabilities.
- **Generate Rancher image lists** for air-gapped deployments (Hangar Genesis).

## Why use hangar?

- Hangar does not require any container runtime (daemon) to copy container images.
- Hangar is cross-platform and works in all Unix-like operating systems.
- Hangar supports both [docker images](https://github.com/moby/docker-image-spec/blob/main/README.md) and [OCI images](https://github.com/opencontainers/image-spec).
- Hangar supports copying/saving/loading/signing/scanning images in parallel to increase speed.
- Hangar is designed to export container images as archive files and import them into image repositories in Air-Gapped environments.

## Getting started

For documentation, visit the [Hangar Documentation](https://prime.ribs.rancher.io/hangar/docs/v1.9).

## Hangar Genesis - Generate Rancher Image Lists

Hangar Genesis (`genesis` or `generate-list`) is an interactive tool for generating comprehensive image manifests for Rancher air-gapped deployments. It helps you create image lists from Rancher Charts and KDM (Kubernetes Distribution Manifest) data.

### Quick Start

Generate an image list for a specific Rancher version:

```bash
hangar genesis --rancher="v2.13.1"
```

### Interactive Mode

Use the interactive terminal UI to select components:

```bash
hangar genesis --rancher="v2.13.1" --tui
```

The interactive mode guides you through:
1. **Step 1**: Select Kubernetes distributions (K3s, RKE2, RKE1) and CNI (Canal, Calico, Cilium, Flannel)
2. **Step 2**: Select Kubernetes versions for each distribution
3. **Step 3**: Choose components and charts to include (Basic vs Add-ons)

### Features

- **Interactive TUI**: Terminal-based graphical interface with hierarchical selection
- **Component Grouping**: Organize images by functional components (CNI, Monitoring, Logging, Storage, Security)
- **Chart Categorization**: Automatic categorization of Rancher charts into logical groups
- **Version Filtering**: Filter images by specific Kubernetes versions
- **YAML Configuration**: Use config files for automation and CI/CD pipelines
- **Vulnerability Scanning**: Optional integration with `hangar scan` for security analysis

### YAML Configuration

Create a config file for non-interactive mode:

```yaml
distros: ["k3s", "rke2"]
cni: "cni_calico"
versions:
  k3s: ["v1.34.3"]
  rke2: ["v1.34.3"]
groups: ["basic", "addons"]
charts: ["rancher-monitoring", "rancher-logging"]
scan:
  enabled: true
  jobs: 4
```

Then use it:

```bash
hangar genesis --rancher="v2.13.1" --config=config.yaml
```

### Examples

**Basic usage (non-interactive):**
```bash
hangar genesis --rancher="v2.13.1" --components=k3s,rke2 --cni=cni_calico
```

**Interactive text mode:**
```bash
hangar genesis --rancher="v2.13.1" --interactive
```

**With vulnerability scanning:**
```bash
hangar genesis --rancher="v2.13.1" --scan --scan-jobs=4
```

**List available charts and categories:**
```bash
hangar list-charts --rancher="v2.13.1"
```

### Author

Hangar Genesis was developed by **ala.eltai@suse.com**

For more details, see the [generate-list-config.example.yaml](generate-list-config.example.yaml) file.

## Contributing

Hangar is open-source and any [issues](https://github.com/rancher/hangar/issues) or [pull requests](https://github.com/rancher/hangar/pulls) are welcomed if you have any suggestions while using Hangar.

## License

Copyright 2025 SUSE Rancher

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
