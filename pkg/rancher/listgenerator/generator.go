package listgenerator

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/rancher/rke/types/kdm"
	"github.com/sirupsen/logrus"
)

type GeneratorOption struct {
	RancherVersion string
	MinKubeVersion string

	ChartsPaths map[string]chartimages.ChartRepoType // map[url]type
	ChartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	KDMPath string // The path of KDM data.json file.
	KDMURL  string // The remote URL of KDM data.json.

	InsecureSkipTLS     bool
	RemoveDeprecatedKDM bool

	// IncludeClusterTypes limits which cluster types should be included when
	// generating images from KDM data. If empty, all supported cluster types
	// (K3S, RKE2, and optionally RKE) are included.
	IncludeClusterTypes []kdmimages.ClusterType

	// IncludeK3sVersions, IncludeRKE2Versions and IncludeRKE1Versions limit
	// which Kubernetes versions should be included for each cluster type when
	// generating images from KDM data. If empty, all compatible versions for
	// the selected cluster type are included.
	IncludeK3sVersions  []string
	IncludeRKE2Versions []string
	IncludeRKE1Versions []string

	// IncludeChartImages controls whether images discovered from Rancher charts
	// should be included. When false, chart images are skipped entirely even if
	// chart URLs or paths are configured.
	IncludeChartImages bool

	// IncludeChartNames limits which charts' images are included. When empty,
	// all charts are included. When non-empty, only images whose source can be
	// mapped to one of the specified chart names will be added.
	IncludeChartNames []string
}

// Generator is a generator to generate image list from charts, KDM data, etc.
type Generator struct {
	rancherVersion string // Rancher version, should be va.b.c
	minKubeVersion string // Minimum RKE1 kube verision, should be va.b.c

	chartsPaths map[string]chartimages.ChartRepoType // map[url]type
	chartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	kdmPath string
	kdmURL  string

	insecureSkipTLS     bool
	removeDeprecatedKDM bool

	// includeClusterTypes limits which cluster types are included when
	// generating images from KDM data. A nil or empty map means all supported
	// cluster types are included.
	includeClusterTypes map[kdmimages.ClusterType]bool

	// includeK3sVersions/includeRKE2Versions/includeRKE1Versions limit the
	// Kubernetes versions that are included per cluster type. A nil or empty
	// map means all compatible versions are included.
	includeK3sVersions  map[string]bool
	includeRKE2Versions map[string]bool
	includeRKE1Versions map[string]bool

	// includeChartImages controls whether chart images are included.
	includeChartImages bool
	// includeChartNames limits which charts are included by name. A nil or
	// empty map means all charts are included.
	includeChartNames map[string]bool

	// All generated images, map[image]map[source]true
	LinuxImages   map[string]map[string]bool
	WindowsImages map[string]map[string]bool

	RKE1LinuxImages   map[string]map[string]bool
	RKE2LinuxImages   map[string]map[string]bool
	K3sLinuxImages    map[string]map[string]bool
	RKE2WindowsImages map[string]map[string]bool

	RKE1Versions map[string]bool
	RKE2Versions map[string]bool
	K3sVersions  map[string]bool
}

func NewGenerator(o *GeneratorOption) (*Generator, error) {
	if o.RancherVersion == "" {
		return nil, fmt.Errorf("invalid rancher version")
	}
	rancherVersion, err := utils.EnsureSemverValid(o.RancherVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher version: %v", o.RancherVersion)
	}
	if o.ChartURLs == nil && o.ChartsPaths == nil &&
		o.KDMPath == "" && o.KDMURL == "" {
		return nil, fmt.Errorf("no input source provided")
	}

	includeClusterTypes := make(map[kdmimages.ClusterType]bool)
	for _, t := range o.IncludeClusterTypes {
		includeClusterTypes[t] = true
	}
	includeK3sVersions := make(map[string]bool)
	for _, v := range o.IncludeK3sVersions {
		includeK3sVersions[v] = true
	}
	includeRKE2Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE2Versions {
		includeRKE2Versions[v] = true
	}
	includeRKE1Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE1Versions {
		includeRKE1Versions[v] = true
	}
	includeChartNames := make(map[string]bool)
	for _, name := range o.IncludeChartNames {
		includeChartNames[name] = true
	}

	g := &Generator{
		rancherVersion:      rancherVersion,
		minKubeVersion:      o.MinKubeVersion,
		chartsPaths:         o.ChartsPaths,
		chartURLs:           o.ChartURLs,
		kdmPath:             o.KDMPath,
		kdmURL:              o.KDMURL,
		insecureSkipTLS:     o.InsecureSkipTLS,
		removeDeprecatedKDM: o.RemoveDeprecatedKDM,

		includeClusterTypes: includeClusterTypes,
		includeK3sVersions:  includeK3sVersions,
		includeRKE2Versions: includeRKE2Versions,
		includeRKE1Versions: includeRKE1Versions,
		includeChartImages:  o.IncludeChartImages,
		includeChartNames:   includeChartNames,

		LinuxImages:       make(map[string]map[string]bool),
		WindowsImages:     make(map[string]map[string]bool),
		K3sLinuxImages:    make(map[string]map[string]bool),
		K3sVersions:       make(map[string]bool),
		RKE1LinuxImages:   make(map[string]map[string]bool),
		RKE1Versions:      make(map[string]bool),
		RKE2LinuxImages:   make(map[string]map[string]bool),
		RKE2WindowsImages: make(map[string]map[string]bool),
		RKE2Versions:      make(map[string]bool),
	}
	return g, nil
}

func (g *Generator) Run(ctx context.Context) error {
	if err := g.generateFromChartPaths(ctx); err != nil {
		return err
	}
	if err := g.generateFromChartURLs(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMPath(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMURL(ctx); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateFromChartPaths(ctx context.Context) error {
	if len(g.chartsPaths) == 0 {
		return nil
	}
	if !g.includeChartImages {
		return nil
	}
	for path := range g.chartsPaths {
		c := chartimages.Chart{
			RancherVersion: g.rancherVersion,
			OS:             chartimages.Linux,
			Type:           g.chartsPaths[path],
			Path:           path,
		}
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			for source := range c.ImageSet[image] {
				if g.shouldIncludeChartSource(source) {
					utils.AddSourceToImage(g.LinuxImages, image, source)
				}
			}
		}
		// fetch windows images
		c.OS = chartimages.Windows
		c.ImageSet = make(map[string]map[string]bool)
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			for source := range c.ImageSet[image] {
				if g.shouldIncludeChartSource(source) {
					utils.AddSourceToImage(g.WindowsImages, image, source)
				}
			}
		}
	}
	return nil
}

func (g *Generator) generateFromChartURLs(ctx context.Context) error {
	if len(g.chartURLs) == 0 {
		return nil
	}
	if !g.includeChartImages {
		return nil
	}
	for url := range g.chartURLs {
		c := chartimages.Chart{
			RancherVersion:  g.rancherVersion,
			OS:              chartimages.Linux,
			Type:            g.chartURLs[url].Type,
			Branch:          g.chartURLs[url].Branch,
			URL:             url,
			InsecureSkipTLS: g.insecureSkipTLS,
		}
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			if chartimages.IgnoreChartImages[image] {
				continue
			}
			for source := range c.ImageSet[image] {
				if g.shouldIncludeChartSource(source) {
					utils.AddSourceToImage(g.LinuxImages, image, source)
				}
			}
		}
		// fetch windows images
		c.OS = chartimages.Windows
		c.ImageSet = make(map[string]map[string]bool)
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			if chartimages.IgnoreChartImages[image] {
				continue
			}
			for source := range c.ImageSet[image] {
				if g.shouldIncludeChartSource(source) {
					utils.AddSourceToImage(g.WindowsImages, image, source)
				}
			}
		}
	}
	return nil
}

func (g *Generator) generateFromKDMPath(ctx context.Context) error {
	if g.kdmPath == "" {
		return nil
	}
	b, err := os.ReadFile(g.kdmPath)
	if err != nil {
		return err
	}
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMURL(ctx context.Context) error {
	if g.kdmURL == "" {
		return nil
	}
	logrus.Infof("Get KDM data from URL: %q", g.kdmURL)

	client := &http.Client{
		Timeout: time.Second * 15,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: g.insecureSkipTLS,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.kdmURL, nil)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMData(ctx context.Context, b []byte) error {
	data, err := kdm.FromData(b)
	if err != nil {
		return fmt.Errorf("generateFromKDMData: %w", err)
	}
	clusters := []kdmimages.ClusterType{
		kdmimages.K3S,
		kdmimages.RKE2,
	}
	if ok, _ := utils.SemverCompare(g.rancherVersion, "v2.12.0-0"); ok < 0 {
		clusters = append(clusters, kdmimages.RKE)
	}

	// Filter clusters if IncludeClusterTypes is specified
	if len(g.includeClusterTypes) > 0 {
		filteredClusters := []kdmimages.ClusterType{}
		for _, t := range clusters {
			if g.includeClusterTypes[t] {
				filteredClusters = append(filteredClusters, t)
			}
		}
		clusters = filteredClusters
	}

	for _, t := range clusters {
		var includeVersions []string
		switch t {
		case kdmimages.K3S:
			if len(g.includeK3sVersions) > 0 {
				for v := range g.includeK3sVersions {
					includeVersions = append(includeVersions, v)
				}
			}
		case kdmimages.RKE2:
			if len(g.includeRKE2Versions) > 0 {
				for v := range g.includeRKE2Versions {
					includeVersions = append(includeVersions, v)
				}
			}
		case kdmimages.RKE:
			if len(g.includeRKE1Versions) > 0 {
				for v := range g.includeRKE1Versions {
					includeVersions = append(includeVersions, v)
				}
			}
		}

		getter, err := kdmimages.NewGetter(&kdmimages.GetterOptions{
			Type:             t,
			RancherVersion:   g.rancherVersion,
			MinKubeVersion:   g.minKubeVersion,
			KDMData:          data,
			InsecureSkipTLS:  g.insecureSkipTLS,
			RemoveDeprecated: g.removeDeprecatedKDM,
			IncludeVersions:  includeVersions,
		})
		if err != nil {
			return err
		}

		if err = getter.Get(ctx); err != nil {
			return err
		}
		utils.MergeImageSourceSet(g.LinuxImages, getter.LinuxImageSet())
		utils.MergeImageSourceSet(g.WindowsImages, getter.WindowsImageSet())
		// Merge sets
		switch getter.Source() {
		case kdmimages.RKE:
			utils.MergeSets(g.RKE1Versions, getter.VersionSet())
			utils.MergeImageSourceSet(g.RKE1LinuxImages, getter.LinuxImageSet())
		case kdmimages.RKE2:
			utils.MergeSets(g.RKE2Versions, getter.VersionSet())
			utils.MergeImageSourceSet(g.RKE2LinuxImages, getter.LinuxImageSet())
			// RKE2 supports Windows
			utils.MergeImageSourceSet(g.RKE2WindowsImages, getter.WindowsImageSet())
		case kdmimages.K3S:
			utils.MergeSets(g.K3sVersions, getter.VersionSet())
			utils.MergeImageSourceSet(g.K3sLinuxImages, getter.LinuxImageSet())
		}
	}
	return nil
}

// shouldIncludeChartSource checks if a chart source should be included based on
// the IncludeChartNames filter. Chart sources have the format:
// [path;chartName:version]
func (g *Generator) shouldIncludeChartSource(source string) bool {
	if len(g.includeChartNames) == 0 {
		return true
	}
	// Parse chart name from source format: [path;chartName:version]
	// Extract the part between ';' and ':'
	start := strings.Index(source, ";")
	if start == -1 {
		// Not a chart source format, include it
		return true
	}
	end := strings.Index(source[start+1:], ":")
	if end == -1 {
		// Malformed source, include it to be safe
		return true
	}
	chartName := source[start+1 : start+1+end]
	return g.includeChartNames[chartName]
}
