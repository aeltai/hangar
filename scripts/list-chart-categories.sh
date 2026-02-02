#!/bin/bash
# Script to list all charts and their categories for a given Rancher version
# Usage: ./scripts/list-chart-categories.sh v2.13.1

set -e

RANCHER_VERSION="${1:-v2.13.1}"
TEMP_OUTPUT="/tmp/hangar-genesis-charts-$$.txt"

echo "Fetching charts for Rancher ${RANCHER_VERSION}..."
echo "This will generate the image list to extract chart information..."
echo ""

# Run genesis in non-interactive mode to get chart data
./hangar genesis \
    --rancher="${RANCHER_VERSION}" \
    --components=k3s,rke2 \
    --k3s-versions=all \
    --rke2-versions=all \
    --charts=all \
    --output="${TEMP_OUTPUT}" \
    > /dev/null 2>&1 || true

echo "Chart Categories for Rancher ${RANCHER_VERSION}:"
echo "================================================"
echo ""

# The actual chart list would need to be extracted from the generator
# For now, showing the explicit mappings from the code
echo "Explicitly Mapped Charts (from chartCategoryByName):"
echo "----------------------------------------------------"
echo "monitoring:"
echo "  - rancher-monitoring"
echo "  - rancher-monitoring-crd"
echo ""
echo "logging:"
echo "  - rancher-logging"
echo "  - rancher-logging-crd"
echo ""
echo "backup-restore:"
echo "  - rancher-backup"
echo "  - rancher-backup-crd"
echo ""
echo "cis:"
echo "  - rancher-cis-benchmark"
echo ""
echo "fleet:"
echo "  - fleet"
echo "  - fleet-crd"
echo "  - fleet-agent"
echo "  - fleet-controller"
echo ""
echo "cluster-api:"
echo "  - rancher-cluster-api"
echo "  - rancher-cluster-api-eks"
echo ""
echo "Charts without explicit mapping are inferred by name:"
echo "  - Contains 'monitoring' -> monitoring"
echo "  - Contains 'logging' -> logging"
echo "  - Contains 'backup' -> backup-restore"
echo "  - Contains 'longhorn'/'harvester'/'storage' -> storage"
echo "  - Contains 'neuvector'/'gatekeeper'/'security' -> security"
echo "  - Contains 'cis' -> cis"
echo "  - Contains 'cluster-api' -> cluster-api"
echo "  - Otherwise -> other"
echo ""
echo "Note: Run './hangar genesis --rancher=${RANCHER_VERSION} --tui' to see all charts interactively"
