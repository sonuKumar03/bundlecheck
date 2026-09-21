package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var skillFiles = []string{
	"SKILL.md",
	"references/cli.md",
	"references/json-schema.md",
	"references/nx.md",
	"references/ci.md",
}

func assertSkillTree(t *testing.T, root, installed string) {
	t.Helper()
	for _, rel := range skillFiles {
		want, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "bundlecheck", rel))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(installed, rel))
		if err != nil {
			t.Fatalf("read installed %s: %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("installed %s differs from source", rel)
		}
	}
}

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
				var c *exec.Cmd
				if strings.HasSuffix(strings.ToLower(os.Getenv("OS")), "windows") || filepath.Separator == '\\' {
					shPath, err := exec.LookPath("sh")
					if err != nil {
						shPath, err = exec.LookPath("bash")
					}
					if err != nil {
						t.Skip("sh/bash not available on Windows")
					}
					c = exec.Command(shPath, append([]string{filepath.ToSlash(filepath.Join(root, "install.sh"))}, argsToUse...)...)
				} else {
					c = exec.Command(filepath.Join(root, "install.sh"), argsToUse...)
				}
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
			binName := "bundlecheck"
			if filepath.Separator == '\\' {
				binName = "bundlecheck.exe"
			}
			binary := filepath.Join(bin, binName)
			_, err := os.Stat(binary)
			if (err == nil) != tt.binary {
				t.Fatalf("binary presence: %v", err)
			}
			_, err = os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
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
				for _, dir := range []string{
					skillDir,
					filepath.Join(home, ".claude", "skills", "bundlecheck"),
					filepath.Join(home, ".codex", "skills", "bundlecheck"),
				} {
					assertSkillTree(t, root, dir)
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

	var cmd *exec.Cmd
	if filepath.Separator == '\\' {
		shPath, err := exec.LookPath("sh")
		if err != nil {
			shPath, err = exec.LookPath("bash")
		}
		if err != nil {
			t.Skip("sh/bash not available on Windows")
		}
		cmd = exec.Command(shPath, filepath.ToSlash(filepath.Join(root, "install.sh")), "--skill-dir", filepath.ToSlash(customDir))
	} else {
		cmd = exec.Command(filepath.Join(root, "install.sh"), "--skill-dir", customDir)
	}

	cmd.Env = append(os.Environ(), "GOBIN="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install with --skill-dir failed: %s, %v", out, err)
	}

	assertSkillTree(t, root, customDir)
	for _, dir := range []string{".agents", ".claude", ".codex"} {
		if _, err := os.Stat(filepath.Join(base, dir, "skills", "bundlecheck", "SKILL.md")); !os.IsNotExist(err) {
			t.Fatalf("custom install unexpectedly populated %s", dir)
		}
	}
}

func TestRemoteSkillInstallDownloadsReferences(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer transport fixture")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	installer := filepath.Join(base, "install.sh")
	source, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installer, source, 0755); err != nil {
		t.Fatal(err)
	}

	body := []byte("#!/bin/sh\necho bundlecheck version 9.8.7\n")
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "bundlecheck", Mode: 0755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(base, "release.tar.gz")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	mockBin := filepath.Join(base, "transport")
	if err := os.MkdirAll(mockBin, 0755); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"uname": "#!/bin/sh\nif [ \"$1\" = -s ]; then echo Linux; else echo x86_64; fi\n",
		"go":    "#!/bin/sh\nexit 42\n",
		"curl": `#!/bin/sh
url="$2"
dest="$4"
case "$url" in
  https://api.github.com/*) printf '{"tag_name":"v9.8.7"}' > "$dest" ;;
  https://github.com/*/releases/download/*) cp "$BUNDLECHECK_TEST_ARCHIVE" "$dest" ;;
  https://raw.githubusercontent.com/*/.agents/skills/bundlecheck/*)
    prefix="https://raw.githubusercontent.com/sonuKumar03/bundlecheck/master/.agents/skills/bundlecheck/"
    rel=${url#"$prefix"}
    cp "$BUNDLECHECK_TEST_SKILL_ROOT/$rel" "$dest"
    ;;
  *) exit 22 ;;
esac
`,
	} {
		if err := os.WriteFile(filepath.Join(mockBin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}

	home := filepath.Join(base, "home")
	cmd := exec.Command("sh", installer, "--with-skill")
	cmd.Env = append(os.Environ(),
		"PATH="+mockBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+home,
		"GOBIN="+filepath.Join(base, "bin"),
		"BUNDLECHECK_TEST_ARCHIVE="+archivePath,
		"BUNDLECHECK_TEST_SKILL_ROOT="+filepath.Join(root, ".agents", "skills", "bundlecheck"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("remote skill install failed: %v\n%s", err, out)
	}
	for _, dir := range []string{
		filepath.Join(home, ".agents", "skills", "bundlecheck"),
		filepath.Join(home, ".claude", "skills", "bundlecheck"),
		filepath.Join(home, ".codex", "skills", "bundlecheck"),
	} {
		assertSkillTree(t, root, dir)
	}
}

func TestPrecompiledInstallUsesLatestVersionAndPlatformArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer transport fixture")
	}
	source, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, osName, ext, binary string }{
		{"Linux", "Linux", "tar.gz", "bundlecheck"},
		{"Windows", "MINGW64_NT", "zip", "bundlecheck.exe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ext == "zip" {
				if _, err := exec.LookPath("unzip"); err != nil {
					t.Skip("unzip unavailable")
				}
			}
			root := t.TempDir()
			installer := filepath.Join(root, "install.sh")
			if err := os.WriteFile(installer, source, 0755); err != nil {
				t.Fatal(err)
			}
			body := []byte("#!/bin/sh\necho bundlecheck version 9.8.7\n")
			var archive bytes.Buffer
			if tc.ext == "zip" {
				w := zip.NewWriter(&archive)
				f, err := w.Create(tc.binary)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write(body); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				gz := gzip.NewWriter(&archive)
				w := tar.NewWriter(gz)
				if err := w.WriteHeader(&tar.Header{Name: tc.binary, Mode: 0755, Size: int64(len(body))}); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write(body); err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				if err := gz.Close(); err != nil {
					t.Fatal(err)
				}
			}
			archivePath := filepath.Join(root, "release."+tc.ext)
			if err := os.WriteFile(archivePath, archive.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			mockBin := filepath.Join(root, "transport")
			if err := os.MkdirAll(mockBin, 0755); err != nil {
				t.Fatal(err)
			}
			for name, script := range map[string]string{
				"uname": "#!/bin/sh\nif [ \"$1\" = -s ]; then echo \"$BUNDLECHECK_TEST_OS\"; else echo x86_64; fi\n",
				"curl":  "#!/bin/sh\ncase \"$2\" in\nhttps://api.github.com/repos/sonuKumar03/bundlecheck/releases/latest) printf '{\"tag_name\":\"v9.8.7\"}' > \"$4\" ;;\n\"$BUNDLECHECK_TEST_RELEASE_URL\") cp \"$BUNDLECHECK_TEST_ARCHIVE\" \"$4\" ;;\n*) exit 22 ;;\nesac\n",
				"go":    "#!/bin/sh\nexit 42\n",
			} {
				if err := os.WriteFile(filepath.Join(mockBin, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			osName := "linux"
			if tc.ext == "zip" {
				osName = "windows"
			}
			bin := filepath.Join(root, "installed")
			cmd := exec.Command("sh", installer)
			cmd.Env = append(os.Environ(), "PATH="+mockBin+string(os.PathListSeparator)+os.Getenv("PATH"), "GOBIN="+bin,
				"BUNDLECHECK_TEST_OS="+tc.osName, "BUNDLECHECK_TEST_ARCHIVE="+archivePath,
				"BUNDLECHECK_TEST_RELEASE_URL=https://github.com/sonuKumar03/bundlecheck/releases/download/v9.8.7/bundlecheck_9.8.7_"+osName+"_amd64."+tc.ext)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("precompiled installation failed: %v\n%s", err, out)
			}
			installed, err := os.ReadFile(filepath.Join(bin, tc.binary))
			if err != nil || !bytes.Equal(installed, body) {
				t.Fatalf("release binary was not installed to GOBIN: err=%v", err)
			}
		})
	}
}
