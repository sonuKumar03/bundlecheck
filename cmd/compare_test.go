package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bundlecheck/internal/comparison"
	"bundlecheck/internal/snapshot"
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
		{"required flags", []string{"compare"}, "nonempty paths"},
		{"missing before", []string{"compare", "--after", after, "--before", ""}, "nonempty paths"},
		{"missing after", []string{"compare", "--before", before, "--after", ""}, "nonempty paths"},
		{"empty path", []string{"compare", "--before", "", "--after", after}, "nonempty"},
		{"invalid format", []string{"compare", "--before", before, "--after", after, "--format", "yaml"}, "unsupported format"},
		{"missing file", []string{"compare", "--before", "no-such-snapshot.json", "--after", after}, "--before"},
		{"malformed before", []string{"compare", "--before", bad, "--after", after}, "--before"},
		{"malformed after", []string{"compare", "--before", before, "--after", bad}, "--after"},
		{"positional argument", []string{"compare", "extra", "--before", before, "--after", after}, "unknown command"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := Execute(tt.args, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
			}
		})
	}
}
