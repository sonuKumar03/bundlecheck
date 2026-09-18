package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	goEnv, err := exec.Command("go", "env", "GOPATH", "GOCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.Split(strings.TrimSpace(string(goEnv)), "\n")
	if len(paths) != 2 {
		t.Fatalf("unexpected Go env: %q", goEnv)
	}
	for _, tt := range []struct {
		name          string
		args          []string
		exit          int
		binary, skill bool
	}{
		{"CLI only", nil, 0, true, false},
		{"CLI and skill", []string{"--with-skill"}, 0, true, true},
		{"help", []string{"--help"}, 0, false, false},
		{"unknown option", []string{"--unknown"}, 2, false, false},
		{"extra argument", []string{"--with-skill", "extra"}, 2, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			caller, home, bin := filepath.Join(base, "caller with spaces"), filepath.Join(base, "home with spaces"), filepath.Join(base, "bin with spaces")
			if err := os.MkdirAll(caller, 0700); err != nil {
				t.Fatal(err)
			}
			skillDir := filepath.Join(home, ".agents", "skills", "bundlecheck")
			if tt.skill {
				if err := os.MkdirAll(skillDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(skillDir, "notes.txt"), []byte("keep me"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{}
			for _, v := range os.Environ() {
				key, _, _ := strings.Cut(v, "=")
				if key != "HOME" && key != "GOBIN" && key != "GOPATH" && key != "GOCACHE" {
					env = append(env, v)
				}
			}
			env = append(env, "HOME="+home, "GOBIN="+bin, "GOPATH="+paths[0], "GOCACHE="+paths[1])
			install := func(extraArgs ...string) (int, []byte) {
				argsToUse := tt.args
				if len(extraArgs) > 0 {
					argsToUse = extraArgs
				}
				c := exec.Command(filepath.Join(root, "install.sh"), argsToUse...)
				c.Dir, c.Env = caller, env
				out, err := c.CombinedOutput()
				if err == nil {
					return 0, out
				}
				if e, ok := err.(*exec.ExitError); ok {
					return e.ExitCode(), out
				}
				t.Fatal(err)
				return -1, out
			}
			code, out := install()
			if code != tt.exit {
				t.Fatalf("exit %d, want %d: %s", code, tt.exit, out)
			}
			binary := filepath.Join(bin, "bundlecheck")
			_, err := os.Stat(binary)
			if (err == nil) != tt.binary {
				t.Fatalf("binary presence: %v", err)
			}
			installed, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
			if (err == nil) != tt.skill {
				t.Fatalf("skill presence: %v", err)
			}
			if tt.binary {
				version, err := exec.Command(binary, "--version").CombinedOutput()
				if err != nil || !strings.HasPrefix(string(version), "bundlecheck version ") {
					t.Fatalf("installed binary: %s,%v", version, err)
				}
				if code, out := install(); code != 0 {
					t.Fatalf("reinstall: %s", out)
				}
			}
			if tt.skill {
				source, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "bundlecheck", "SKILL.md"))
				if err != nil || !bytes.Equal(installed, source) {
					t.Fatalf("installed skill differs from source: %v", err)
				}
				notes, err := os.ReadFile(filepath.Join(skillDir, "notes.txt"))
				if err != nil || string(notes) != "keep me" {
					t.Fatal("reinstall changed unrelated files")
				}
			}
		})
	}
}

func TestInstallCustomSkillDir(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	customDir := filepath.Join(base, "custom", "skills", "bundlecheck")
	bin := filepath.Join(base, "bin")

	cmd := exec.Command(filepath.Join(root, "install.sh"), "--skill-dir", customDir)
	cmd.Env = append(os.Environ(), "GOBIN="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install with --skill-dir failed: %s, %v", out, err)
	}

	skillFile := filepath.Join(customDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("expected skill at %s, got err: %v", skillFile, err)
	}
}
