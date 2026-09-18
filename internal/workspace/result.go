package workspace

import (
	"cmp"
	"fmt"
	"math"
	"path"
	"slices"
	"strings"

	"bundlecheck/internal/analysis"
	"bundlecheck/internal/snapshot"
)

type App struct {
	Name       string                   `json:"name"`
	Status     string                   `json:"status"`
	Diagnostic string                   `json:"diagnostic,omitempty"`
	Stats      string                   `json:"stats,omitempty"`
	Dist       string                   `json:"dist,omitempty"`
	Analysis   *analysis.AnalysisResult `json:"analysis,omitempty"`
	DrillDown  [][]string               `json:"drillDown,omitempty"`
}
type Bytes struct {
	InitialBytes int64 `json:"initialBytes"`
	LazyBytes    int64 `json:"lazyBytes"`
	TotalBytes   int64 `json:"totalBytes"`
}
type Contributor struct {
	Name string `json:"name"`
	Bytes
	Apps map[string]*Bytes `json:"apps"`
}
type Finding struct {
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	InitialBytes int64    `json:"initialBytes"`
	Apps         []string `json:"apps"`
}
type Result struct {
	SchemaVersion string        `json:"schemaVersion"`
	ToolVersion   string        `json:"toolVersion"`
	Command       string        `json:"command"`
	Root          string        `json:"root"`
	Target        string        `json:"target"`
	Configuration string        `json:"configuration"`
	Complete      bool          `json:"complete"`
	Apps          []App         `json:"apps"`
	Packages      []Contributor `json:"packages"`
	Libraries     []Contributor `json:"libraries"`
	Findings      []Finding     `json:"findings"`
}

func NewResult(root, target, configuration string) *Result {
	return &Result{SchemaVersion: "1", ToolVersion: analysis.ToolVersion, Command: "workspace-summary", Root: root, Target: target, Configuration: configuration, Complete: true, Apps: []App{}, Packages: []Contributor{}, Libraries: []Contributor{}, Findings: []Finding{}}
}
func add(total *int64, n int64) error {
	if n < 0 || n > math.MaxInt64-*total {
		return fmt.Errorf("workspace byte total exceeds int64 or is negative")
	}
	*total += n
	return nil
}

func Libraries(root string, projects map[string]Project, s *snapshot.BundleSnapshot) ([]snapshot.Package, error) {
	libraries := map[string]snapshot.Package{}
	normalizedRoot := snapshot.CleanPath(root)
	owners := map[string]string{}
	for name, p := range projects {
		dir := snapshot.CleanPath(p.Data.Root)
		if dir == "." || dir == "" {
			continue
		}
		if !portableAbsolute(dir) {
			dir = path.Join(normalizedRoot, dir)
		}
		if previous, exists := owners[dir]; exists {
			return nil, fmt.Errorf("project roots overlap exactly: %s and %s", previous, name)
		}
		owners[dir] = name
	}
	for _, output := range s.Outputs {
		if !snapshot.IsJavaScript(output.Path) {
			continue
		}
		seen := map[string]bool{}
		for _, contribution := range output.Inputs {
			input := snapshot.CleanPath(contribution.Input)
			if seen[input] || contribution.Bytes <= 0 {
				continue
			}
			seen[input] = true
			if _, npm := analysis.PackageName(input); npm {
				continue
			}
			if !portableAbsolute(input) {
				input = path.Join(normalizedRoot, input)
			}
			owner, best := "", 0
			for dir, name := range owners {
				if strings.HasPrefix(input, dir+"/") && len(dir) > best {
					owner, best = name, len(dir)
				}
			}
			if owner == "" || (projects[owner].Type != "lib" && projects[owner].Data.ProjectType != "library") {
				continue
			}
			p := libraries[owner]
			p.Name = owner
			bucket := &p.LazyBytes
			if output.Initial {
				bucket = &p.InitialBytes
			}
			if err := add(bucket, contribution.Bytes); err != nil {
				return nil, err
			}
			if err := add(&p.TotalBytes, contribution.Bytes); err != nil {
				return nil, err
			}
			libraries[owner] = p
		}
	}
	result := []snapshot.Package{}
	for _, p := range libraries {
		result = append(result, p)
	}
	slices.SortFunc(result, func(a, b snapshot.Package) int {
		if n := cmp.Compare(b.InitialBytes, a.InitialBytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return result, nil
}

func Matrix(apps []App, contributions map[string][]snapshot.Package) ([]Contributor, error) {
	rows := map[string]Contributor{}
	for _, app := range apps {
		if app.Status != "analyzed" {
			continue
		}
		for _, p := range contributions[app.Name] {
			row, exists := rows[p.Name]
			if !exists {
				row = Contributor{Name: p.Name, Apps: map[string]*Bytes{}}
				for _, a := range apps {
					if a.Status == "analyzed" {
						row.Apps[a.Name] = &Bytes{}
					} else if a.Status != "unsupported" {
						row.Apps[a.Name] = nil
					}
				}
			}
			row.Apps[app.Name] = &Bytes{p.InitialBytes, p.LazyBytes, p.TotalBytes}
			if err := add(&row.InitialBytes, p.InitialBytes); err != nil {
				return nil, err
			}
			if err := add(&row.LazyBytes, p.LazyBytes); err != nil {
				return nil, err
			}
			if err := add(&row.TotalBytes, p.TotalBytes); err != nil {
				return nil, err
			}
			rows[p.Name] = row
		}
	}
	result := []Contributor{}
	for _, row := range rows {
		result = append(result, row)
	}
	slices.SortFunc(result, func(a, b Contributor) int {
		if n := cmp.Compare(b.InitialBytes, a.InitialBytes); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return result, nil
}
func Findings(packages, libraries []Contributor) []Finding {
	result := []Finding{}
	for i, rows := range [][]Contributor{packages, libraries} {
		kind := "repeated-initial-package"
		if i == 1 {
			kind = "repeated-initial-library"
		}
		for _, row := range rows {
			apps := []string{}
			for name, value := range row.Apps {
				if value != nil && value.InitialBytes > 0 {
					apps = append(apps, name)
				}
			}
			if len(apps) < 2 {
				continue
			}
			slices.Sort(apps)
			result = append(result, Finding{kind, row.Name, row.InitialBytes, apps})
		}
	}
	slices.SortFunc(result, func(a, b Finding) int {
		if n := cmp.Compare(b.InitialBytes, a.InitialBytes); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return cmp.Compare(a.Kind, b.Kind)
	})
	return result
}

func portableAbsolute(p string) bool {
	return path.IsAbs(p) || (len(p) >= 3 && p[1] == ':' && p[2] == '/')
}
