package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/comparison"
	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func TestCompareFixtures(t *testing.T) {
	base := filepath.Join("..", "testdata", "comparison")
	args := []string{"compare", "--before", filepath.Join(base, "before.json"), "--after", filepath.Join(base, "after.json"), "--format", "json"}
	var previous string
	for i := 0; i < 3; i++ {
		var out, errOut bytes.Buffer
		if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
			t.Fatalf("exit %d: %s", code, errOut.String())
		}
		var got comparison.Result
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Command != "compare" || got.Summary.Delta != (snapshot.Totals{InitialJS: 100, LazyJS: 200, TotalJS: 300}) || len(got.Packages) != 7 {
			t.Fatalf("result: %+v", got)
		}
		var names []string
		for _, p := range got.Packages {
			names = append(names, p.Name)
		}
		if !reflect.DeepEqual(names, []string{"grow", "removed", "shrink", "moved", "added", "lazy-grow", "@angular/core"}) {
			t.Fatalf("order: %v", names)
		}
		if i > 0 && out.String() != previous {
			t.Fatal("nondeterministic JSON stdout")
		}
		previous = out.String()
	}
	var out, errOut bytes.Buffer
	if code := Execute(args[:len(args)-2], &out, &errOut); code != 0 || errOut.Len() != 0 || !strings.HasPrefix(out.String(), "Angular Bundle Comparison\n") {
		t.Fatalf("default text: %s / %s", out.String(), errOut.String())
	}
}

func TestCompareShortFlagsAndOptions(t *testing.T) {
	base := filepath.Join("..", "testdata", "comparison")
	outPath := filepath.Join(t.TempDir(), "comp.json")

	args := []string{"compare", "-b", filepath.Join(base, "before.json"), "-a", filepath.Join(base, "after.json"), "-o", outPath, "-f", "json"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected stdout empty with -o, got %s", out.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var res comparison.Result
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatal(err)
	}
	if res.Summary.Delta.InitialJS != 100 {
		t.Errorf("delta mismatch: %+v", res.Summary.Delta)
	}
}

func TestCompareMarkdown(t *testing.T) {
	base := filepath.Join("..", "testdata", "comparison")
	args := []string{"compare", "-b", filepath.Join(base, "before.json"), "-a", filepath.Join(base, "after.json"), "-f", "markdown"}
	var out, errOut bytes.Buffer
	if code := Execute(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "## 📊 Angular Bundle Comparison") {
		t.Errorf("expected markdown header in compare output, got: %s", output)
	}
	if !strings.Contains(output, "| Category | Before | After | Delta |") {
		t.Errorf("expected delta table in markdown, got: %s", output)
	}
	if !strings.Contains(output, "### Changed Packages") {
		t.Errorf("expected changed packages section, got: %s", output)
	}
}

func TestCompareErrors(t *testing.T) {
	before, after := filepath.Join("..", "testdata", "comparison", "before.json"), filepath.Join("..", "testdata", "comparison", "after.json")
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"required flags", []string{"compare"}, "paths are required"},
		{"missing before", []string{"compare", "--after", after, "--before", ""}, "paths are required"},
		{"missing after", []string{"compare", "--before", before, "--after", ""}, "paths are required"},
		{"empty path", []string{"compare", "--before", "", "--after", after}, "paths are required"},
		{"invalid format", []string{"compare", "--before", before, "--after", after, "--format", "yaml"}, "unsupported format"},
		{"missing file", []string{"compare", "--before", "no-such-snapshot.json", "--after", after}, "before"},
		{"malformed before", []string{"compare", "--before", bad, "--after", after}, "before"},
		{"malformed after", []string{"compare", "--before", before, "--after", bad}, "after"},
		{"too many positional args", []string{"compare", before, after, "extra"}, "accepts at most 2 arg(s)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := Execute(tt.args, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestComparePositionalArguments(t *testing.T) {
	before := filepath.Join("..", "testdata", "comparison", "before.json")
	after := filepath.Join("..", "testdata", "comparison", "after.json")

	var out, errOut bytes.Buffer
	code := Execute([]string{"compare", before, after, "-f", "json"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("compare positional args failed: %s", errOut.String())
	}

	var res comparison.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if res.Summary.Delta.InitialJS != 100 {
		t.Errorf("expected initial delta 100, got %d", res.Summary.Delta.InitialJS)
	}
}

func TestCompareGitHubPRFormat(t *testing.T) {
	base := filepath.Join("..", "testdata", "comparison")
	before := filepath.Join(base, "before.json")
	after := filepath.Join(base, "after.json")

	var out, errOut bytes.Buffer
	code := Execute([]string{"compare", before, after, "-f", "github-pr"}, &out, &errOut)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "<!-- bundleradar-comment -->") {
		t.Errorf("expected sticky marker in compare output, got: %s", output)
	}
	if !strings.Contains(output, "## 📦 BundleRadar PR Report") {
		t.Errorf("expected PR report header in compare output, got: %s", output)
	}
	if !strings.Contains(output, "| Metric | Before | After | Delta | % Change | Visual Diff |") {
		t.Errorf("expected metrics table with visual diff bar, got: %s", output)
	}
}
