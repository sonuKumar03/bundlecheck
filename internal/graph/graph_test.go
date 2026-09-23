package graph

import (
	"reflect"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/snapshot"
)

func TestClassify(t *testing.T) {
	for _, tt := range []struct {
		name        string
		outputs     []snapshot.BundleOutput
		roots, want []string
		bad         bool
	}{
		{"static and dynamic", []snapshot.BundleOutput{
			{Path: "main.js", Imports: []snapshot.Import{{Path: "A.js"}, {Path: "C.js", Dynamic: true}}},
			{Path: "A.js", Imports: []snapshot.Import{{Path: "B.js"}, {Path: "D.js", Dynamic: true}}},
			{Path: "B.js"}, {Path: "C.js"}, {Path: "D.js"},
		}, []string{"main.js"}, []string{"main.js", "A.js", "B.js"}, false},
		{"cycle and shared roots", []snapshot.BundleOutput{
			{Path: "main.js", Imports: []snapshot.Import{{Path: "A.js"}}},
			{Path: "A.js", Imports: []snapshot.Import{{Path: "B.js"}}},
			{Path: "B.js", Imports: []snapshot.Import{{Path: "A.js"}}},
			{Path: "polyfills.js", Imports: []snapshot.Import{{Path: "B.js"}}},
		}, []string{"main.js", "polyfills.js"}, []string{"main.js", "A.js", "B.js", "polyfills.js"}, false},
		{"relative imports", []snapshot.BundleOutput{{Path: "out/main.js", Imports: []snapshot.Import{{Path: "./A.mjs"}, {Path: "styles.css"}, {Path: "remote.js", External: true}}}, {Path: "out/A.mjs"}}, []string{"out/main.js"}, []string{"out/main.js", "out/A.mjs"}, false},
		{"exact keys before relative", []snapshot.BundleOutput{{Path: "out/main.js", Imports: []snapshot.Import{{Path: "A.js"}}}, {Path: "A.js"}, {Path: "out/A.js"}}, []string{"out/main.js"}, []string{"out/main.js", "A.js"}, false},
		{"missing static", []snapshot.BundleOutput{{Path: "main.js", Imports: []snapshot.Import{{Path: "missing.js"}}}}, []string{"main.js"}, nil, true},
		{"JS asset is not a static import", []snapshot.BundleOutput{{Path: "main.js", Imports: []snapshot.Import{{Path: "worker.js", Asset: true}}}, {Path: "worker.js"}}, []string{"main.js"}, []string{"main.js"}, false},
		{"missing dynamic", []snapshot.BundleOutput{{Path: "main.js", Imports: []snapshot.Import{{Path: "missing.js", Dynamic: true}}}}, []string{"main.js"}, nil, true},
		{"missing root", []snapshot.BundleOutput{{Path: "main.js"}}, []string{"wrong.js"}, nil, true},
		{"no roots", []snapshot.BundleOutput{{Path: "main.js"}}, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Classify(tt.outputs, tt.roots)
			if tt.bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, o := range tt.outputs {
				if o.Initial {
					got = append(got, o.Path)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
