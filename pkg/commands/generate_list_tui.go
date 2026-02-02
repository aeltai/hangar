package commands

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/rancher/listgenerator"
)

// treeNode is one row in the tree: preset, group, chart folder, chart, or image (display-only).
type treeNode struct {
	Id       string
	Label    string
	Kind     string // preset, component, chart_all, chart, image
	Count    int
	Children []treeNode
}

// treeRow is a flattened row for display (with depth for indent).
type treeRow struct {
	Depth  int
	Node   treeNode
	RowIdx int // index in the full flattened list (for selection key)
}

// treeModel shows only 2 groups: Basic and AddOns
type treeModel struct {
	roots          []treeNode
	expanded       map[string]bool // node id -> expanded
	visible        []treeRow       // flattened visible rows (rebuilt when expanded changes)
	cursor         int
	selected       map[string]bool // node id -> selected (for preset/component/chart)
	done           bool
	cniForStandard string
	width          int
	height         int
	// Store chart groups for Basic preview (Fleet/CNI charts)
	fleetCharts []treeNode
	cniCharts   []treeNode
}

func (m *treeModel) buildVisible() {
	m.visible = nil
	var walk func(nodes []treeNode, depth int)
	walk = func(nodes []treeNode, depth int) {
		for _, n := range nodes {
			m.visible = append(m.visible, treeRow{Depth: depth, Node: n})
			if (n.Kind == "preset" || n.Kind == "component" || n.Kind == "chart_all" || n.Kind == "chart") && m.expanded[n.Id] && len(n.Children) > 0 {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(m.roots, 0)
}

func (m *treeModel) rowAt(i int) (treeRow, bool) {
	if i < 0 || i >= len(m.visible) {
		return treeRow{}, false
	}
	return m.visible[i], true
}

func (m *treeModel) selectable(r treeRow) bool {
	return r.Node.Kind != "image"
}

func (m *treeModel) findParentId(r treeRow) string {
	// Find parent by walking up the tree
	var findParent func(nodes []treeNode, targetId string, parentId string) string
	findParent = func(nodes []treeNode, targetId string, parentId string) string {
		for _, n := range nodes {
			if n.Id == targetId {
				return parentId
			}
			if found := findParent(n.Children, targetId, n.Id); found != "" {
				return found
			}
		}
		return ""
	}
	return findParent(m.roots, r.Node.Id, "")
}

func (m *treeModel) checkAllChildrenDeselected(n treeNode, parentId string) bool {
	if n.Id == parentId {
		for _, child := range n.Children {
			if child.Kind == "chart" || child.Kind == "component" {
				if m.selected[child.Id] {
					return false
				}
			}
		}
		return true
	}
	for _, child := range n.Children {
		if !m.checkAllChildrenDeselected(child, parentId) {
			return false
		}
	}
	return true
}

func (m *treeModel) Init() tea.Cmd {
	return nil
}

func (m *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mm := m
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mm.width = msg.Width
		mm.height = msg.Height
		return mm, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if mm.cursor > 0 {
				mm.cursor--
			}
			return mm, nil
		case "down", "j":
			if mm.cursor < len(mm.visible)-1 {
				mm.cursor++
			}
			return mm, nil
		case " ":
			if r, ok := mm.rowAt(mm.cursor); ok && mm.selectable(r) {
				newState := !mm.selected[r.Node.Id]
				mm.selected[r.Node.Id] = newState
				// Hierarchical selection: selecting a parent selects all children
				if newState && len(r.Node.Children) > 0 {
					var selectChildren func(n treeNode)
					selectChildren = func(n treeNode) {
						if n.Kind == "chart" || n.Kind == "component" {
							mm.selected[n.Id] = true
						}
						for _, child := range n.Children {
							selectChildren(child)
						}
					}
					for _, child := range r.Node.Children {
						selectChildren(child)
					}
				}
				// If deselecting a child, check if parent should be deselected
				if !newState && r.Depth > 0 {
					// Find parent and check if all siblings are deselected
					parentId := mm.findParentId(r)
					if parentId != "" {
						allDeselected := true
						for _, root := range mm.roots {
							if mm.checkAllChildrenDeselected(root, parentId) {
								allDeselected = true
								break
							}
						}
						if allDeselected {
							mm.selected[parentId] = false
						}
					}
				}
			}
			return mm, nil
		case "right", "l":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
			if expandable {
				mm.expanded[r.Node.Id] = !mm.expanded[r.Node.Id]
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
				return mm, nil
			}
			return mm, nil
		case "enter":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
			if expandable {
				mm.expanded[r.Node.Id] = !mm.expanded[r.Node.Id]
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
				return mm, nil
			}
			return mm, nil
		case "d":
			mm.done = true
			return mm, tea.Quit
		case "left", "h":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			if r.Depth > 0 && (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") {
				mm.expanded[r.Node.Id] = false
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
			}
			return mm, nil
		}
	}
	return m, nil
}

// collectSelectedItems returns a list of selected charts and their images for preview
func (m *treeModel) collectSelectedItems() (charts []string, images []string) {
	selectedCharts := make(map[string]bool)
	deselectedCharts := make(map[string]bool)

	// Helper to collect charts from a group node
	var collectChartsFromGroup func(n treeNode)
	collectChartsFromGroup = func(n treeNode) {
		if n.Kind == "chart" {
			selectedCharts[n.Id] = true
			// Collect images from this chart
			for _, child := range n.Children {
				if child.Kind == "image" {
					images = append(images, child.Label)
				}
			}
		}
		for _, child := range n.Children {
			collectChartsFromGroup(child)
		}
	}

	// First pass: collect from Basic and AddOns groups
	for _, root := range m.roots {
		if !m.selected[root.Id] {
			continue
		}
		switch root.Kind {
		case "component":
			// Basic group: includes distro, CNI, Rancher components, Fleet
			if root.Id == "basic" {
				// Basic contains all images directly (flat structure)
				// Collect all images from Basic
				for _, child := range root.Children {
					if child.Kind == "image" {
						images = append(images, child.Label)
					}
				}
			}
			// AddOns group: collect charts from all subdirs
			if root.Id == "addons" {
				collectChartsFromGroup(root)
			}
			// Individual component selection
			if root.Id != "basic" && root.Id != "addons" {
				if root.Id == "cni" || root.Id == "fleet" ||
					strings.HasPrefix(root.Id, "addon_") {
					collectChartsFromGroup(root)
				}
			}
		case "chart":
			selectedCharts[root.Id] = true
			for _, child := range root.Children {
				if child.Kind == "image" {
					images = append(images, child.Label)
				}
			}
		}
	}

	// Second pass: find explicitly deselected charts (child of selected parent)
	for _, r := range m.visible {
		if r.Node.Kind == "chart" && !m.selected[r.Node.Id] {
			// Check if parent is selected
			parentId := m.findParentId(r)
			if parentId != "" && m.selected[parentId] {
				deselectedCharts[r.Node.Id] = true
			}
		}
	}

	// Final chart list: selected minus deselected
	for chart := range selectedCharts {
		if !deselectedCharts[chart] {
			charts = append(charts, chart)
		}
	}
	sort.Strings(charts)
	sort.Strings(images)

	return charts, images
}

// getChartsForSelectedGroup returns charts and images for ALL selected groups/charts
func (m *treeModel) getChartsForSelectedGroup() (charts []string, images []string) {
	// Deduplicate images and charts
	seenImages := make(map[string]bool)
	seenCharts := make(map[string]bool)

	// Helper to recursively collect charts and images from a node
	var collectChartsAndImages func(n treeNode, includeChildren bool)
	collectChartsAndImages = func(n treeNode, includeChildren bool) {
		if n.Kind == "chart" {
			if !seenCharts[n.Label] {
				charts = append(charts, n.Label)
				seenCharts[n.Label] = true
			}
			// Collect images from this chart
			for _, child := range n.Children {
				if child.Kind == "image" && !seenImages[child.Label] {
					images = append(images, child.Label)
					seenImages[child.Label] = true
				}
			}
		}
		if includeChildren {
			for _, child := range n.Children {
				collectChartsAndImages(child, true)
			}
		}
	}

	// Collect from ALL selected items in the visible tree
	for _, r := range m.visible {
		if !m.selected[r.Node.Id] {
			continue
		}

		// Basic group: collect all images directly
		if r.Node.Id == "basic" {
			// Collect Fleet charts for Column 2
			for _, chart := range m.fleetCharts {
				if !seenCharts[chart.Label] {
					charts = append(charts, chart.Label)
					seenCharts[chart.Label] = true
				}
			}
			// Collect CNI charts for Column 2 (only selected CNI)
			for _, chart := range m.cniCharts {
				if !seenCharts[chart.Label] {
					charts = append(charts, chart.Label)
					seenCharts[chart.Label] = true
				}
			}
			// Collect ALL images directly from Basic
			for _, child := range r.Node.Children {
				if child.Kind == "image" && !seenImages[child.Label] {
					images = append(images, child.Label)
					seenImages[child.Label] = true
				}
			}
			continue
		}

		// AddOns group: collect from all selected subgroups/charts
		if r.Node.Id == "addons" {
			// Check if any subgroups are selected, otherwise collect all
			hasSelectedSubgroups := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedSubgroups = true
					break
				}
			}
			if hasSelectedSubgroups {
				// Only collect from selected subgroups
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectChartsAndImages(child, true)
					}
				}
			} else {
				// Collect from all subgroups (AddOns is selected but no specific subgroup)
				collectChartsAndImages(r.Node, true)
			}
			continue
		}

		// Subgroups (like Monitoring, Logging, etc.): collect charts and images
		if strings.HasPrefix(r.Node.Id, "addon_") {
			// Check if any charts are selected, otherwise collect all
			hasSelectedCharts := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedCharts = true
					break
				}
			}
			if hasSelectedCharts {
				// Only collect from selected charts
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectChartsAndImages(child, true)
					}
				}
			} else {
				// Collect from all charts in this subgroup
				collectChartsAndImages(r.Node, true)
			}
			continue
		}

		// Individual chart: collect images from this chart
		if r.Node.Kind == "chart" {
			collectChartsAndImages(r.Node, false)
		}
	}

	// Remove duplicates
	chartMap := make(map[string]bool)
	var uniqueCharts []string
	for _, chart := range charts {
		if !chartMap[chart] {
			chartMap[chart] = true
			uniqueCharts = append(uniqueCharts, chart)
		}
	}

	imageMap := make(map[string]bool)
	var uniqueImages []string
	for _, img := range images {
		if !imageMap[img] {
			imageMap[img] = true
			uniqueImages = append(uniqueImages, img)
		}
	}

	sort.Strings(uniqueCharts)
	sort.Strings(uniqueImages)
	return uniqueCharts, uniqueImages
}

func (m *treeModel) View() string {
	// Default width if not set
	width := m.width
	if width == 0 {
		width = 160
	}

	// Split into 3 columns: 30% groups, 30% charts, 40% images
	col1Width := int(float64(width) * 0.3)
	col2Width := int(float64(width) * 0.3)
	col3Width := width - col1Width - col2Width - 2 // -2 for separators

	// Build column 1 (groups/tree)
	var col1Builder strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	col1Builder.WriteString(title("Step 2: Groups") + "\n")
	col1Builder.WriteString("↑/↓ move   Space toggle\n")
	col1Builder.WriteString("←/→ expand   d done\n\n")

	for i, r := range m.visible {
		indent := strings.Repeat("  ", r.Depth)
		prefix := "  "
		if m.selectable(r) && m.selected[r.Node.Id] {
			prefix = "X "
		}
		line := indent + prefix + r.Node.Label
		if r.Node.Count > 0 && r.Node.Kind != "image" {
			line += fmt.Sprintf(" (%d)", r.Node.Count)
		}
		expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
		if expandable {
			if m.expanded[r.Node.Id] {
				line += " ▼"
			} else {
				line += " ▶"
			}
		}
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		// Truncate if too long
		if len(line) > col1Width-2 {
			line = line[:col1Width-5] + "..."
		}
		col1Builder.WriteString(line + "\n")
	}

	// Build column 2 (charts for selected group)
	var col2Builder strings.Builder
	col2Title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render
	col2Builder.WriteString(col2Title("Charts") + "\n")
	col2Builder.WriteString(strings.Repeat("─", col2Width-2) + "\n\n")

	charts, images := m.getChartsForSelectedGroup()

	if len(charts) > 0 {
		maxCharts := 100
		displayCharts := charts
		if len(displayCharts) > maxCharts {
			displayCharts = displayCharts[:maxCharts]
		}
		for _, chart := range displayCharts {
			chartName := chart
			if len(chartName) > col2Width-4 {
				chartName = chartName[:col2Width-7] + "..."
			}
			col2Builder.WriteString("  • " + chartName + "\n")
		}
		if len(charts) > maxCharts {
			col2Builder.WriteString(fmt.Sprintf("\n  ... and %d more\n", len(charts)-maxCharts))
		}
	} else {
		col2Builder.WriteString("Select a group to see charts.\n")
	}

	// Build column 3 (images for selected charts)
	var col3Builder strings.Builder
	col3Title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render
	col3Builder.WriteString(col3Title(fmt.Sprintf("Images (%d)", len(images))) + "\n")
	col3Builder.WriteString(strings.Repeat("─", col3Width-2) + "\n\n")

	if len(images) > 0 {
		maxImages := 200
		displayImages := images
		if len(displayImages) > maxImages {
			displayImages = displayImages[:maxImages]
		}
		for _, img := range displayImages {
			imgName := img
			if len(imgName) > col3Width-4 {
				imgName = imgName[:col3Width-7] + "..."
			}
			col3Builder.WriteString("  • " + imgName + "\n")
		}
		if len(images) > maxImages {
			col3Builder.WriteString(fmt.Sprintf("\n  ... and %d more\n", len(images)-maxImages))
		}
	} else {
		col3Builder.WriteString("Select a group to see images.\n")
	}

	// Style the columns
	col1Style := lipgloss.NewStyle().Width(col1Width)
	col2Style := lipgloss.NewStyle().Width(col2Width).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))
	col3Style := lipgloss.NewStyle().Width(col3Width).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))
	if m.height > 0 {
		col1Style = col1Style.MaxHeight(m.height)
		col2Style = col2Style.MaxHeight(m.height)
		col3Style = col3Style.MaxHeight(m.height)
	}

	col1Content := col1Style.Render(col1Builder.String())
	col2Content := col2Style.Render(col2Builder.String())
	col3Content := col3Style.Render(col3Builder.String())

	// Combine columns horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, col1Content, col2Content, col3Content)
}

// runTreeTUI runs the tree TUI (2 groups: Basic and AddOns).
// components should be a comma-separated string of selected cluster types from Step 1 (e.g., "k3s,rke2").
func runTreeTUI(roots []treeNode, cniForStandard string, components string, fleetCharts []treeNode, cniCharts []treeNode) (componentIDs []string, chartNames []string, err error) {
	expanded := make(map[string]bool)
	selected := make(map[string]bool)
	m := &treeModel{
		roots:          roots,
		expanded:       expanded,
		cursor:         0,
		selected:       selected,
		cniForStandard: cniForStandard,
		fleetCharts:    fleetCharts,
		cniCharts:      cniCharts,
	}
	m.buildVisible()

	// Don't use alt screen so logs remain visible
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, nil, err
	}
	mm := final.(*treeModel)

	// Collect all selected chart names (including from selected groups)
	selectedCharts := make(map[string]bool)
	deselectedCharts := make(map[string]bool)

	// Helper to collect charts from a group node
	var collectChartsFromGroup func(n treeNode)
	collectChartsFromGroup = func(n treeNode) {
		if n.Kind == "chart" {
			selectedCharts[n.Id] = true
		}
		for _, child := range n.Children {
			collectChartsFromGroup(child)
		}
	}

	// First pass: collect from Basic and AddOns groups
	for _, r := range mm.visible {
		if !mm.selected[r.Node.Id] || !mm.selectable(r) {
			continue
		}
		switch r.Node.Kind {
		case "component":
			// Basic group: includes distro, CNI, Rancher components, Fleet
			if r.Node.Id == "basic" {
				// Basic contains all images directly, so use BasicPresetWithCNI to get component IDs
				componentIDs = listgenerator.BasicPresetWithCNI(components, mm.cniForStandard)
				componentIDs = append(componentIDs, "fleet")
			}
			// AddOns group: collect charts from all subdirs
			if r.Node.Id == "addons" {
				collectChartsFromGroup(r.Node)
			}
			// Individual component selection (for backward compatibility)
			if r.Node.Id != "basic" && r.Node.Id != "addons" {
				componentIDs = append(componentIDs, r.Node.Id)
				// If group is selected, collect all its charts
				if r.Node.Id == "cni" || r.Node.Id == "fleet" ||
					strings.HasPrefix(r.Node.Id, "addon_") {
					collectChartsFromGroup(r.Node)
				}
			}
		case "chart":
			selectedCharts[r.Node.Id] = true
		}
	}

	// Second pass: find explicitly deselected charts (child of selected parent)
	for _, r := range mm.visible {
		if r.Node.Kind == "chart" && !mm.selected[r.Node.Id] {
			// Check if parent (group or preset) is selected
			parentId := mm.findParentId(r)
			if parentId != "" {
				// Check if parent is selected (could be group or preset)
				if mm.selected[parentId] {
					deselectedCharts[r.Node.Id] = true
				} else {
					// Check if parent's parent (preset) is selected
					for _, root := range mm.roots {
						if root.Id == parentId && mm.selected[root.Id] {
							// Check if this chart is in preset's children
							var inPreset bool
							var checkPreset func(n treeNode)
							checkPreset = func(n treeNode) {
								if n.Id == r.Node.Id {
									inPreset = true
									return
								}
								for _, child := range n.Children {
									checkPreset(child)
								}
							}
							checkPreset(root)
							if inPreset {
								deselectedCharts[r.Node.Id] = true
							}
						}
					}
				}
			}
		}
	}

	// Final chart list: selected minus deselected
	for chart := range selectedCharts {
		if !deselectedCharts[chart] {
			chartNames = append(chartNames, chart)
		}
	}
	sort.Strings(chartNames)

	return componentIDs, chartNames, nil
}

// imagesFromGroup returns sorted image refs from a ComponentGroup.
func imagesFromGroup(g *listgenerator.ComponentGroup) []string {
	if g == nil {
		return nil
	}
	var out []string
	for img := range g.LinuxImages {
		out = append(out, img)
	}
	for img := range g.WindowsImages {
		out = append(out, img)
	}
	sort.Strings(out)
	return out
}

// --- Step 1 TUI: cluster types + CNI ---

type step1Row struct {
	kind  string // "cluster" or "cni"
	id    string
	label string
}

type step1Model struct {
	rows     []step1Row
	cursor   int
	selected map[int]bool
	done     bool
	showRKE1 bool
	width    int
	height   int
}

func (m step1Model) Init() tea.Cmd {
	return nil
}

func (m step1Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mm := &m
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mm.width = msg.Width
		mm.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if mm.cursor > 0 {
				mm.cursor--
			}
			return m, nil
		case "down", "j":
			if mm.cursor < len(mm.rows)-1 {
				mm.cursor++
			}
			return m, nil
		case " ":
			idx := mm.cursor
			if idx >= 0 && idx < len(mm.rows) {
				r := mm.rows[idx]
				if r.kind == "cluster" {
					mm.selected[idx] = !mm.selected[idx]
				}
				if r.kind == "cni" {
					for i := range mm.rows {
						if mm.rows[i].kind == "cni" {
							mm.selected[i] = (i == idx)
						}
					}
				}
			}
			return m, nil
		case "enter":
			mm.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m step1Model) View() string {
	width := m.width
	if width == 0 {
		width = 120 // Default width
	}
	leftWidth := int(float64(width) * 0.5)
	rightWidth := width - leftWidth - 1

	// Build left column (selection)
	var leftBuilder strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render

	// Determine stage based on row types
	isDistroStage := len(m.rows) > 0 && m.rows[0].kind == "cluster"
	isCNIStage := len(m.rows) > 0 && m.rows[0].kind == "cni"

	var stageTitle string
	if isDistroStage {
		stageTitle = "Step 1: Select Distro"
	} else if isCNIStage {
		stageTitle = "Step 1: Select CNI"
	} else {
		stageTitle = "Step 1: Selection"
	}

	leftBuilder.WriteString(title(stageTitle) + "\n")
	leftBuilder.WriteString("↑/↓ move   Space toggle\n")
	leftBuilder.WriteString("Enter confirm   q quit\n\n")

	if isCNIStage {
		leftBuilder.WriteString("CNI:\n")
	}

	for i, r := range m.rows {
		prefix := "  "
		if m.selected[i] {
			prefix = "X "
		}
		line := prefix + r.label
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		if isCNIStage {
			line = "  " + line // Extra indent for CNI sub-items
		}
		leftBuilder.WriteString(line + "\n")
	}
	leftBuilder.WriteString("\nPress Enter when done.\n")

	// Build right column (preview - show what will be included)
	var rightBuilder strings.Builder
	rightTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render
	rightBuilder.WriteString(rightTitle("Preview") + "\n")
	rightBuilder.WriteString(strings.Repeat("─", rightWidth-2) + "\n\n")

	// Collect selected cluster types and CNI
	var selectedClusters []string
	var selectedCNI string
	for i, r := range m.rows {
		if m.selected[i] {
			if r.kind == "cluster" {
				selectedClusters = append(selectedClusters, r.label)
			}
			if r.kind == "cni" {
				selectedCNI = r.label
			}
		}
	}

	if len(selectedClusters) > 0 {
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Selected Components:") + "\n\n")

		// Show distro images that will be included
		rightBuilder.WriteString("Distro Images:\n")
		for _, cluster := range selectedClusters {
			rightBuilder.WriteString("  • " + cluster + " core images\n")
			rightBuilder.WriteString("    (control-plane, system components)\n")
		}

		// Show CNI images (only for CNI stage)
		if isCNIStage && selectedCNI != "" && selectedCNI != "CNI: None" {
			rightBuilder.WriteString("\nCNI Images:\n")
			// Map CNI IDs to display names
			cniName := selectedCNI
			if strings.HasPrefix(selectedCNI, "CNI: ") {
				cniName = strings.TrimPrefix(selectedCNI, "CNI: ")
			} else if selectedCNI == "cni_canal" {
				cniName = "Canal"
			} else if selectedCNI == "cni_calico" {
				cniName = "Calico"
			} else if selectedCNI == "cni_cilium" {
				cniName = "Cilium"
			} else if selectedCNI == "cni_flannel" {
				cniName = "Flannel"
			}
			rightBuilder.WriteString("  • " + cniName + " CNI images\n")
		}

		rightBuilder.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("(Exact images depend on\nselected Kubernetes versions)") + "\n")
	} else {
		rightBuilder.WriteString("Select cluster types and CNI\nto see preview.\n")
	}

	// Style columns
	leftStyle := lipgloss.NewStyle().Width(leftWidth)
	rightStyle := lipgloss.NewStyle().Width(rightWidth).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))

	leftContent := leftStyle.Render(leftBuilder.String())
	rightContent := rightStyle.Render(rightBuilder.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftContent, rightContent)
}

// RunStep1TUI runs the Step 1 TUI in stages: distro → CNI → versions.
// hasRKE1 controls whether RKE1 is shown.
// capabilities provides Kubernetes versions for each cluster type.
// Returns components (e.g. "k3s,rke2"), k3sVers, rke2Vers, rkeVers, cni.
func RunStep1TUI(hasRKE1 bool, capabilities map[string]kdmimages.ClusterVersionInfo) (components string, k3sVers string, rke2Vers string, rkeVers string, cni string, err error) {
	// Stage 1: Select distro (K3s, RKE2, RKE1)
	var distroRows []step1Row
	distroRows = append(distroRows, step1Row{"cluster", "k3s", "K3s"})
	distroRows = append(distroRows, step1Row{"cluster", "rke2", "RKE2"})
	if hasRKE1 {
		distroRows = append(distroRows, step1Row{"cluster", "rke", "RKE1"})
	}

	distroSelected := make(map[int]bool)
	for i := range distroRows {
		distroSelected[i] = false
	}
	// Default: select K3s and RKE2
	for i, r := range distroRows {
		if r.id == "k3s" || r.id == "rke2" {
			distroSelected[i] = true
		}
	}

	distroModel := step1Model{
		rows:     distroRows,
		cursor:   0,
		selected: distroSelected,
		done:     false,
		showRKE1: hasRKE1,
	}
	p1 := tea.NewProgram(distroModel)
	final1, err := p1.Run()
	if err != nil {
		return "", "", "", "", "", err
	}
	mm1 := final1.(step1Model)

	var selectedDistros []string
	for i, r := range mm1.rows {
		if mm1.selected[i] {
			selectedDistros = append(selectedDistros, r.id)
		}
	}
	if len(selectedDistros) == 0 {
		return "", "", "", "", "", fmt.Errorf("at least one distro must be selected")
	}

	// Stage 2: Select CNI (based on selected distros - Flannel only for K3s)
	var cniRows []step1Row
	cniRows = append(cniRows, step1Row{"cni", "cni_canal", "canal"})
	cniRows = append(cniRows, step1Row{"cni", "cni_calico", "calico"})
	cniRows = append(cniRows, step1Row{"cni", "cni_cilium", "cilium"})
	// Flannel only available for K3s
	hasK3s := false
	for _, d := range selectedDistros {
		if d == "k3s" {
			hasK3s = true
			break
		}
	}
	if hasK3s {
		cniRows = append(cniRows, step1Row{"cni", "cni_flannel", "flannel"})
	}

	cniSelected := make(map[int]bool)
	for i := range cniRows {
		cniSelected[i] = false
	}
	// Default: select first CNI (Canal)
	if len(cniRows) > 0 {
		cniSelected[0] = true
	}

	cniModel := step1Model{
		rows:     cniRows,
		cursor:   0,
		selected: cniSelected,
		done:     false,
		showRKE1: hasRKE1,
	}
	p2 := tea.NewProgram(cniModel)
	final2, err := p2.Run()
	if err != nil {
		return "", "", "", "", "", err
	}
	mm2 := final2.(step1Model)

	var cniSel string
	for i, r := range mm2.rows {
		if mm2.selected[i] {
			cniSel = r.id
			break
		}
	}
	if cniSel == "" {
		cniSel = "cni_canal" // Default
	}

	// Stage 3: Version selection for each selected cluster type
	k3sVers = "all"
	rke2Vers = "all"
	rkeVers = "all"

	for _, comp := range selectedDistros {
		var versions []string
		info, ok := capabilities[comp]
		if !ok || len(info.Versions) == 0 {
			continue
		}
		versions = info.Versions

		// Show version selection screen
		selectedVers, verr := runVersionSelectionTUI(comp, versions)
		if verr != nil {
			return "", "", "", "", "", verr
		}
		if len(selectedVers) > 0 {
			versStr := strings.Join(selectedVers, ",")
			switch comp {
			case "k3s":
				k3sVers = versStr
			case "rke2":
				rke2Vers = versStr
			case "rke":
				rkeVers = versStr
			}
		}
	}

	return strings.Join(selectedDistros, ","), k3sVers, rke2Vers, rkeVers, cniSel, nil
}

// versionSelectionModel is a TUI for selecting Kubernetes versions.
type versionSelectionModel struct {
	clusterType string
	versions    []string
	cursor      int
	selected    map[int]bool
	done        bool
}

func (m *versionSelectionModel) Init() tea.Cmd {
	return nil
}

func (m *versionSelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.versions)-1 {
				m.cursor++
			}
			return m, nil
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
			return m, nil
		case "a":
			// Select all
			for i := range m.versions {
				m.selected[i] = true
			}
			return m, nil
		case "enter", "d":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *versionSelectionModel) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	b.WriteString(title(fmt.Sprintf("Step 1: Select %s Kubernetes versions", strings.ToUpper(m.clusterType))) + "\n")
	b.WriteString("↑/↓ move   Space toggle   a select all   Enter/d done   q quit\n\n")
	for i, v := range m.versions {
		prefix := "  "
		if m.selected[i] {
			prefix = "X "
		}
		line := prefix + v
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nPress Enter or 'd' when done (or 'a' to select all, empty = all versions).\n")
	return b.String()
}

// runVersionSelectionTUI shows a TUI for selecting Kubernetes versions for a cluster type.
func runVersionSelectionTUI(clusterType string, versions []string) ([]string, error) {
	if len(versions) == 0 {
		return nil, nil
	}

	selected := make(map[int]bool)
	for i := range versions {
		selected[i] = false // Default: none selected = use "all"
	}

	m := &versionSelectionModel{
		clusterType: clusterType,
		versions:    versions,
		cursor:      0,
		selected:    selected,
		done:        false,
	}

	// Don't use alt screen so logs remain visible
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	mm := final.(*versionSelectionModel)

	var selectedVers []string
	hasSelection := false
	for i := range mm.versions {
		if mm.selected[i] {
			hasSelection = true
			selectedVers = append(selectedVers, mm.versions[i])
		}
	}

	// If nothing selected, return empty = use "all"
	if !hasSelection {
		return nil, nil
	}
	return selectedVers, nil
}
