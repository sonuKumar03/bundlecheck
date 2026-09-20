package analysis

import (
	"math"
	"reflect"
	"testing"

	"github.com/sonuKumar03/bundlecheck/internal/snapshot"
)

func TestPackageName(t *testing.T) {
	for _, tt := range []struct {
		path, name string
		ok         bool
	}{
		{"node_modules/lodash/lodash.js", "lodash", true},
		{"node_modules/@angular/core/fesm2022/core.mjs", "@angular/core", true},
		{`C:\app\node_modules\@angular\core\core.mjs`, "@angular/core", true},
		{"node_modules/a/node_modules/b/index.js", "b", true},
		{"node_modules/.pnpm/rxjs@7/node_modules/rxjs/index.js", "rxjs", true},
		{"src/app/app.component.ts", "", false},
		{"my_node_modules/a/index.js", "", false},
		{"node_modules/@angular", "", false},
		{"node_modules/", "", false},
		{"node_modules/.pnpm/internal.js", "", false},
	} {
		t.Run(tt.path, func(t *testing.T) {
			name, ok := PackageName(tt.path)
			if name != tt.name || ok != tt.ok {
				t.Fatalf("got %q,%v; want %q,%v", name, ok, tt.name, tt.ok)
			}
		})
	}
}

func TestAnalyze(t *testing.T) {
	s := &snapshot.BundleSnapshot{SchemaVersion: "1", Outputs: []snapshot.BundleOutput{
		{Path: "main.js", Bytes: 80, Initial: true, Inputs: []snapshot.Contribution{
			{Input: "node_modules/pdfjs-dist/a.js", Bytes: 20}, {Input: "node_modules/pdfjs-dist/b.js", Bytes: 30},
			{Input: `node_modules\pdfjs-dist\a.js`, Bytes: 20}, {Input: "node_modules/@angular/core/core.mjs", Bytes: 10}, {Input: "src/main.ts", Bytes: 10},
		}},
		{Path: "lazy.mjs", Bytes: 50, Inputs: []snapshot.Contribution{{Input: "node_modules/pdfjs-dist/a.js", Bytes: 20}, {Input: "node_modules/rxjs/rxjs.js", Bytes: 15}}},
		{Path: "styles.css", Bytes: 10000, Initial: true, Inputs: []snapshot.Contribution{{Input: "node_modules/styles/index.css", Bytes: 9000}}},
	}}
	r, err := Analyze(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary != (snapshot.Totals{InitialJS: 80, LazyJS: 50, TotalJS: 130}) {
		t.Fatalf("totals %+v", r.Summary)
	}
	want := []snapshot.Package{{Name: "pdfjs-dist", InitialBytes: 50, LazyBytes: 20, TotalBytes: 70}, {Name: "@angular/core", InitialBytes: 10, TotalBytes: 10}, {Name: "rxjs", LazyBytes: 15, TotalBytes: 15}}
	if !reflect.DeepEqual(r.Packages, want) {
		t.Fatalf("packages %+v, want %+v", r.Packages, want)
	}
	if s.Totals != r.Summary || !reflect.DeepEqual(s.Packages, r.Packages) {
		t.Fatal("snapshot totals/packages not populated")
	}
}

func TestAnalyzeTiesAndEmpty(t *testing.T) {
	s := &snapshot.BundleSnapshot{SchemaVersion: "1", Outputs: []snapshot.BundleOutput{{Path: "main.js", Bytes: 30, Initial: true, Inputs: []snapshot.Contribution{
		{Input: "node_modules/z/a.js", Bytes: 10}, {Input: "node_modules/a/a.js", Bytes: 10},
	}}}}
	r, err := Analyze(s)
	if err != nil || r.Packages[0].Name != "a" || r.Packages[1].Name != "z" {
		t.Fatalf("ties: %+v,%v", r, err)
	}
	r, err = Analyze(&snapshot.BundleSnapshot{SchemaVersion: "1"})
	if err != nil || r.Packages == nil || len(r.Packages) != 0 {
		t.Fatalf("empty packages: %+v,%v", r, err)
	}
}

func TestAnalyzeInvalid(t *testing.T) {
	for _, tt := range []struct {
		name    string
		outputs []snapshot.BundleOutput
	}{
		{"negative bytes", []snapshot.BundleOutput{{Path: "main.js", Bytes: -1}}},
		{"overflow", []snapshot.BundleOutput{{Path: "main.js", Bytes: math.MaxInt64}, {Path: "lazy.js", Bytes: 1}}},
		{"duplicate outputs", []snapshot.BundleOutput{{Path: "main.js", Bytes: 10}, {Path: "./main.js", Bytes: 10}}},
		{"conflicting contributions", []snapshot.BundleOutput{{Path: "main.js", Bytes: 30, Inputs: []snapshot.Contribution{{Input: "node_modules/a/a.js", Bytes: 10}, {Input: "./node_modules/a/a.js", Bytes: 11}}}}},
		{"negative contribution", []snapshot.BundleOutput{{Path: "main.js", Inputs: []snapshot.Contribution{{Input: "node_modules/a/a.js", Bytes: -1}}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Analyze(&snapshot.BundleSnapshot{SchemaVersion: "1", Outputs: tt.outputs}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
