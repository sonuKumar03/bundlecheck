# BundleCheck Agent Skill v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not commit this plan unless the user explicitly asks.

**Goal:** Turn BundleCheck's distributable Agent Skill from a command reference into a reliable Angular bundle-optimization workflow, package its conditional references correctly across supported agents, add a measurable evaluation corpus, and align public claims with the repository's real contracts.

**Architecture:** Keep activation, workflow selection, safety rules, MCP/CLI routing, and reporting in a short `SKILL.md`. Move command, schema, Nx, and CI detail into routed references. Treat the Skill as the reasoning layer, prefer MCP when its tools are available, and use JSON CLI commands as the universal fallback. Extend the existing installer and Go documentation-contract tests instead of adding a second packaging or validation system.

**Tech Stack:** Agent Skills Markdown/YAML, POSIX shell, Go 1.27.1 tests, Cobra/MCP contracts, static HTML documentation.

**Spec:** User-provided Agent Skill audit dated 2026-09-21. Its requirements are restated below so this plan is self-contained.

## Global Constraints

- Keep the skill name `bundlecheck` and automatic discovery behavior.
- Target Angular applications built with esbuild. Do not activate for Lighthouse, Web Vitals, runtime CPU/memory, generic Vite/webpack, or Node bundle work unless compatible Angular/esbuild stats are available.
- The binary and Angular `stats.json` are prerequisites. Node/Nx is required only when the agent must build or inspect an Nx workspace; Git is required only for git-ref baseline workflows.
- Prefer available BundleCheck MCP tools. Otherwise run the CLI with `--format json`. MCP and CLI implement the same diagnostic workflow; neither replaces the Skill.
- Never claim LCP, INP, memory, CPU, or runtime improvements from bundle-size evidence alone.
- Treat `savingsBytes` as an estimate until a production rebuild and `measure` confirm the delta.
- Establish a baseline before edits, inspect the traced application usage before changing code, run relevant project tests after edits, and preserve unrelated user work.
- Keep `SKILL.md` at or below 120 lines; place conditional detail in references.
- Preserve `--skill-dir` behavior and unrelated files already present in destination skill directories.
- Use no new runtime dependency or separate docs generator.
- Prefix plan execution shell commands with `rtk`.

## Review Focus

- Running the skill outside the BundleCheck source checkout must never suggest local `./install.sh`.
- A CLI-only environment and an MCP-enabled environment must follow the same baseline-to-verification decisions.
- Installing the split skill must include every referenced file in local-source and remote-installer modes.
- Negative trigger prompts about runtime performance or non-Angular bundles must not activate the skill.
- Documentation must not reintroduce stale versions, Go requirements for prebuilt users, or unqualified speed/dependency claims.

## File Map and Interfaces

- Modify `.agents/skills/bundlecheck/SKILL.md`: discriminating frontmatter, decision routing, optimization loop, safety rules, final report contract.
- Create `.agents/skills/bundlecheck/references/cli.md`: CLI fallback, discovery, command matrix, errors, and install prerequisite.
- Create `.agents/skills/bundlecheck/references/json-schema.md`: summary, suggestion, trace, comparison, exit-code, and compatibility contracts.
- Create `.agents/skills/bundlecheck/references/nx.md`: Nx discovery, partial-result, freshness, and multi-app interpretation rules.
- Create `.agents/skills/bundlecheck/references/ci.md`: baseline, measure, check, Markdown, and GitHub Action workflows.
- Modify `install.sh` and `install_test.go`: install the complete skill tree into canonical agent locations.
- Create `testdata/skill-evals/triggers.yaml` and `testdata/skill-evals/workflows.yaml`: positive, negative, and workflow evaluation cases.
- Create `testdata/skill-evals/README.md`: repeatable forward-evaluation protocol and acceptance thresholds.
- Create `skill_contract_test.go`: skill metadata/reference/eval corpus contracts.
- Modify `README.md`, `npm/bundlecheck/README.md`, `docs/agents.md`, `docs/index.html`: Skill → MCP → CLI positioning and corrected trust claims.
- Modify `docs_contract_test.go`: version and marketing-copy drift prevention.
- Modify `scripts/bump-version.sh` only if the new contract exposes a missing version target.

---

### Task 1: Add failing skill structure contracts

**Files:**
- Create: `skill_contract_test.go`
- Test: `.agents/skills/bundlecheck/SKILL.md` and future `references/*.md`

**Interfaces:**
- Consumes: YAML frontmatter from `SKILL.md`.
- Produces: repository tests that enforce metadata, routing, reference existence, and context-size limits.

- [ ] **Step 1: Create a frontmatter reader and the failing metadata test**

Add a root-package test with this metadata type:

~~~go
type skillMetadata struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	Compatibility string `yaml:"compatibility"`
}
~~~

Implement `readSkill(t *testing.T) (skillMetadata, string)` by reading `.agents/skills/bundlecheck/SKILL.md`, requiring the opening `---` delimiter, splitting at the next `---`, and unmarshalling the first section with the already-installed `gopkg.in/yaml.v3`.

Add `TestAgentSkillMetadata` assertions:

~~~go
if meta.Name != "bundlecheck" {
	t.Fatalf("name = %q, want bundlecheck", meta.Name)
}
for _, term := range []string{"Angular", "esbuild", "bundle size", "Do not use"} {
	if !strings.Contains(meta.Description, term) {
		t.Errorf("description must contain %q", term)
	}
}
for _, term := range []string{"bundlecheck CLI", "stats.json"} {
	if !strings.Contains(meta.Compatibility, term) {
		t.Errorf("compatibility must contain %q", term)
	}
}
~~~

- [ ] **Step 2: Add failing body and reference tests**

Add `TestAgentSkillProgressiveDisclosure`. Count body lines with `strings.Count(body, "\n")+1` and require at most 120. Extract Markdown targets matching `references/[a-z0-9-]+\.md`, require exactly these four unique paths, and `os.Stat` every target relative to the skill directory:

~~~text
references/cli.md
references/json-schema.md
references/nx.md
references/ci.md
~~~

Reject the fragile local-install instruction with:

~~~go
if strings.Contains(body, "./install.sh --with-skill") {
	t.Fatal("skill must not assume the bundlecheck source checkout")
}
~~~

- [ ] **Step 3: Run the focused test and observe the expected failure**

Run: `rtk go test . -run TestAgentSkill -count=1`

Expected: FAIL because compatibility, routed references, and the line limit are absent.

### Task 2: Rewrite the Skill as the decision and safety layer

**Files:**
- Modify: `.agents/skills/bundlecheck/SKILL.md`
- Create: `.agents/skills/bundlecheck/references/cli.md`
- Create: `.agents/skills/bundlecheck/references/json-schema.md`
- Create: `.agents/skills/bundlecheck/references/nx.md`
- Create: `.agents/skills/bundlecheck/references/ci.md`
- Test: `skill_contract_test.go`

**Interfaces:**
- Consumes: current CLI/MCP contracts from `cmd/`, `internal/mcp/server.go`, and `docs/agents.md`.
- Produces: a <=120-line entrypoint and four conditional references used by the installer.

- [ ] **Step 1: Replace the frontmatter with intent and boundary metadata**

Use this exact frontmatter:

~~~yaml
---
name: bundlecheck
description: >
  Use this skill when the user wants to understand, compare, or reduce the
  JavaScript bundle size of an Angular application built with esbuild.
  Use it to identify initial or lazy bundle contributors, trace why a package
  or source file is bundled, diagnose bundle regressions, measure an
  optimization against a baseline, enforce bundle budgets, or inspect costs
  across Nx applications. Do not use it for runtime CPU, memory, Lighthouse,
  Web Vitals, network profiling, or non-Angular builds unless compatible
  Angular/esbuild stats are available.
compatibility: >
  Requires the bundlecheck CLI and Angular esbuild stats.json. Node and Nx are
  required only to build or inspect an Nx workspace. Git is required only for
  git-ref baseline workflows.
---
~~~

- [ ] **Step 2: Write the core intent router in `SKILL.md`**

Keep this routing in the entrypoint:

| User intent | First operation |
| --- | --- |
| What is large? | summary |
| Why is X bundled? | why |
| What should I optimize? | suggest, then why |
| Optimize this app | baseline → summary/suggest → why → inspect source → edit → rebuild/test → measure/check |
| Did this PR regress? | measure or compare |
| Prevent regressions | check |
| Compare Nx applications | workspace summary |

State execution selection immediately after the table:

1. Use matching `bundle_*` / `workspace_summary` MCP tools when available.
2. Otherwise verify `command -v bundlecheck` and use CLI JSON.
3. If the command is missing, stop and give the supported remote install command; never assume the BundleCheck repository or local `./install.sh` exists.

- [ ] **Step 3: Encode the autonomous optimization loop**

The loop must require, in order:

1. Build production stats if current artifacts are missing or stale.
2. Capture the pre-edit baseline.
3. Rank candidates with summary/suggest.
4. Confirm a candidate with why and inspect its actual source usage.
5. Make the smallest authorized application change.
6. Rebuild production stats and run relevant application tests.
7. Measure against the baseline and run the requested gate.
8. Keep the change only when measured evidence supports it; otherwise report the result without deleting unrelated or pre-existing work.

- [ ] **Step 4: Encode interpretation and reporting guardrails**

Require the final report to include baseline/current initial bytes, signed delta, affected packages or sources, edited files, tests/gates run, and remaining uncertainty. Explicitly forbid runtime-performance claims from static bundle data and label suggestion savings as estimates until measured.

- [ ] **Step 5: Route conditional detail**

Add links with load conditions:

- Read `references/cli.md` only for CLI fallback, installation, discovery, flags, or failures.
- Read `references/json-schema.md` only when manually interpreting JSON or writing automation.
- Read `references/nx.md` only for Nx/multi-app work.
- Read `references/ci.md` only for baselines, budget gates, GitHub Actions, or PR reporting.

- [ ] **Step 6: Move existing detail without duplication**

Move the current command catalog and troubleshooting to `cli.md`, JSON and exit-code contracts to `json-schema.md`, workspace semantics to `nx.md`, and baseline/measure/check/Markdown CI procedures to `ci.md`. Keep each fact in one file and preserve actual command names and field names.

- [ ] **Step 7: Run the contract**

Run: `rtk go test . -run TestAgentSkill -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the skill unit**

~~~sh
rtk git add .agents/skills/bundlecheck skill_contract_test.go
rtk git commit -m "feat(skill): turn bundlecheck into an optimization workflow"
~~~

### Task 3: Package the full skill tree for supported agents

**Files:**
- Modify: `install.sh`
- Modify: `install_test.go`
- Test: `.agents/skills/bundlecheck/references/*.md`

**Interfaces:**
- Consumes: the five-file skill manifest from Task 2.
- Produces: `install_skill_tree <destination-directory>` for local and remote installer modes.

- [ ] **Step 1: Extend installer tests before changing the script**

Add `assertSkillTree(t *testing.T, root, installed string)` in `install_test.go`. For each path below, compare the installed bytes with `root/.agents/skills/bundlecheck/<path>`:

~~~text
SKILL.md
references/cli.md
references/json-schema.md
references/nx.md
references/ci.md
~~~

Update `TestInstall` so `--with-skill` requires identical trees at:

~~~text
$HOME/.agents/skills/bundlecheck
$HOME/.claude/skills/bundlecheck
$HOME/.codex/skills/bundlecheck
~~~

Keep the existing `notes.txt` assertion. Update `TestInstallCustomSkillDir` to require the complete tree only in the custom destination.

- [ ] **Step 2: Add a remote-mode test**

Create `TestRemoteSkillInstallDownloadsReferences` by copying `install.sh` to a temporary directory without `go.mod`, placing mock `uname`, `curl`, and a failing `go` on `PATH`, and invoking `--with-skill`. The mock `curl` must return the existing fake binary archive for release URLs and copy the matching repository skill file for each raw `.agents/skills/bundlecheck/...` URL. Assert all three default destinations contain the five files.

- [ ] **Step 3: Run the installer tests and observe failure**

Run: `rtk go test . -run 'Test(Install|RemoteSkill)' -count=1`

Expected: FAIL because only `SKILL.md` is currently copied.

- [ ] **Step 4: Replace the single-file installer with a fixed manifest**

Define this newline-separated POSIX-shell manifest:

~~~sh
skill_files='SKILL.md
references/cli.md
references/json-schema.md
references/nx.md
references/ci.md'
~~~

Implement `install_skill_tree()` to create each parent directory and either copy from `$script_dir/.agents/skills/bundlecheck/$rel` or download `https://raw.githubusercontent.com/$REPO/master/.agents/skills/bundlecheck/$rel`. Do not delete the destination directory.

- [ ] **Step 5: Install explicit destinations**

For `--with-skill` without `--skill-dir`, call `install_skill_tree` for `$HOME/.agents/skills/bundlecheck`, `$HOME/.claude/skills/bundlecheck`, and `$HOME/.codex/skills/bundlecheck`. Retain the current conditional Antigravity legacy destination. For `--skill-dir`, install only the requested destination.

- [ ] **Step 6: Verify syntax and behavior**

Run:

~~~sh
rtk sh -n install.sh
rtk go test . -run 'Test(Install|RemoteSkill)' -count=1
~~~

Expected: PASS.

- [ ] **Step 7: Commit the packaging unit**

~~~sh
rtk git add install.sh install_test.go
rtk git commit -m "feat(installer): distribute the complete agent skill"
~~~

### Task 4: Add trigger and workflow evaluation corpora

**Files:**
- Create: `testdata/skill-evals/triggers.yaml`
- Create: `testdata/skill-evals/workflows.yaml`
- Create: `testdata/skill-evals/README.md`
- Modify: `skill_contract_test.go`

**Interfaces:**
- Consumes: skill activation boundary and intent router.
- Produces: deterministic eval fixtures plus a repeatable three-run forward-evaluation protocol.

- [ ] **Step 1: Add ten positive and ten near-miss trigger cases**

Use fields `id`, `prompt`, and `should_trigger`. Include these positive intents, with at least two casual/typo phrasings:

1. Angular initial bundle grew after a PR.
2. Reduce initial JS by 100 KB.
3. Find which Nx app pulls `exceljs`.
4. Explain why `pdfjs-dist` is in main.
5. Compare a refactor with a saved baseline.
6. Add a bundle regression gate.
7. Identify the largest lazy chunks.
8. Trace an Angular source file into an emitted chunk.
9. Rank safe bundle optimizations.
10. Compare shared-library cost across Angular apps.

Include these negative intents:

1. Diagnose Angular INP.
2. Find a browser memory leak.
3. Run Lighthouse.
4. Profile CPU.
5. Optimize a React Vite bundle without Angular stats.
6. Analyze a Node server bundle.
7. Debug network waterfalls.
8. Improve image loading.
9. Reduce API latency.
10. Explain webpack configuration without compatible Angular/esbuild stats.

- [ ] **Step 2: Add workflow-output cases**

Use fields `id`, `prompt`, `required_stages`, and `forbidden_claims`. Add cases for `diagnose`, `optimize`, `pr-regression`, `ci-gate`, and `nx-comparison`. The optimize case must require:

~~~yaml
required_stages:
  - baseline
  - summary-or-suggest
  - why
  - source-inspection
  - authorized-edit
  - production-rebuild
  - project-tests
  - measure
  - report-signed-delta
forbidden_claims:
  - runtime performance improved
  - LCP improved
  - INP improved
~~~

- [ ] **Step 3: Add corpus contract tests**

Extend `skill_contract_test.go` with typed YAML loading. Require:

- exactly 10 positive and 10 negative trigger cases;
- unique non-empty IDs and prompts;
- all five workflow case IDs;
- `optimize` contains all nine stages above;
- every workflow has at least one forbidden claim.

- [ ] **Step 4: Document forward evaluation**

In `testdata/skill-evals/README.md`, require three fresh-session runs per trigger prompt and one run per workflow prompt. Record observed activation/stages in a temporary results table. Acceptance is at least 90% positive activation, at least 90% negative rejection, and 100% inclusion of required workflow stages without forbidden claims. Failed cases must drive the smallest description or instruction change, followed by a full rerun.

- [ ] **Step 5: Run corpus contracts**

Run: `rtk go test . -run 'Test(SkillEval|AgentSkill)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the eval unit**

~~~sh
rtk git add testdata/skill-evals skill_contract_test.go
rtk git commit -m "test(skill): add activation and workflow eval corpora"
~~~

### Task 5: Reposition the public Agent Skill and remove trust drift

**Files:**
- Modify: `README.md`
- Modify: `npm/bundlecheck/README.md`
- Modify: `docs/agents.md`
- Modify: `docs/index.html`
- Modify: `docs_contract_test.go`
- Conditional modify: `scripts/bump-version.sh`

**Interfaces:**
- Consumes: `VERSION`, `go.mod`, Skill/MCP/CLI architecture.
- Produces: synchronized public documentation and regression tests.

- [ ] **Step 1: Add failing documentation contracts**

Extend `TestDocumentationContract_VersionSync` to extract the `docs/agents.md` tool-version sentence and `docs/index.html` `softwareVersion` plus visible version badge, requiring all to equal `VERSION`.

Add `TestDocumentationContract_MarketingClaims`. Reject these strings from `docs/index.html`:

~~~text
Zero-Dependency Go CLI
SUB-MILLISECOND SPEED
< 5ms Execution
Go v1.23+
For coding assistants without direct MCP connections
~~~

Require `self-contained` and `runtime dependencies` language in the website and both READMEs.

- [ ] **Step 2: Run the contracts and observe failure**

Run: `rtk go test . -run 'TestDocumentationContract_(VersionSync|MarketingClaims)' -count=1`

Expected: FAIL on the stale `0.3.0` docs value and current website claims.

- [ ] **Step 3: Rewrite the integration story**

In README, npm README, `docs/agents.md`, and the website Agent section, present:

~~~text
Agent Skill = reasoning and optimization workflow
MCP = preferred structured tool interface when available
CLI JSON = universal fallback
~~~

Replace “two integration options” and all MCP-or-Skill framing. Lead with “Give your coding agent a measured bundle optimization loop,” show the baseline → diagnose → trace → edit → rebuild/test → measure → gate flow, and include this prompt:

~~~text
Reduce the initial JavaScript bundle by at least 50 KB without changing
application behavior. Establish a baseline first, identify the highest-confidence
optimization, trace its import path, make the change, rebuild, run tests, and
report the measured delta.
~~~

- [ ] **Step 4: Correct install documentation**

Document that `--with-skill` installs the complete skill for generic agents, Claude Code, and Codex. Keep `--skill-dir <path>` as the explicit custom-location escape hatch. In agent instructions, make the installed binary a prerequisite rather than suggesting a checkout-relative installer.

- [ ] **Step 5: Correct version and trust claims**

Change `docs/agents.md` tool version to `0.4.2`. Remove the website's Go-version footer because prebuilt users do not need Go. Replace “Zero-Dependency Go CLI” with “Self-contained Go binary” and describe “zero runtime dependencies” rather than zero source dependencies.

Use one qualified performance statement across README, npm README, and the site:

~~~text
Fast native analysis; reported timings exclude Angular builds and external Nx subprocesses.
~~~

Remove the competing `<5ms`, “sub-millisecond,” `<10ms`, and `<1ms` marketing claims unless a separately published benchmark methodology is added in the same change.

- [ ] **Step 6: Keep synchronized surfaces synchronized**

Edit `README.md` first, copy it to `npm/bundlecheck/README.md`, then update HTML and agent docs. Inspect `scripts/bump-version.sh`; change it only if the extended version test shows one of the new checked surfaces is not already updated.

- [ ] **Step 7: Run documentation contracts**

Run: `rtk go test . -run TestDocumentationContract -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the documentation unit**

~~~sh
rtk git add README.md npm/bundlecheck/README.md docs/agents.md docs/index.html docs_contract_test.go scripts/bump-version.sh
rtk git commit -m "docs(agent): explain the measured optimization workflow"
~~~

### Task 6: Production-readiness verification

- [ ] Run skill and docs contracts: `rtk go test . -run 'Test(AgentSkill|SkillEval|DocumentationContract)' -count=1`.
- [ ] Run installer tests: `rtk go test . -run 'Test(Install|RemoteSkill)' -count=1`.
- [ ] Validate shell syntax: `rtk sh -n install.sh`.
- [ ] Run the complete suite: `rtk go test ./... -count=1`.
- [ ] Run static checks: `rtk go vet ./...`.
- [ ] Build the binary: `rtk go build ./...`.
- [ ] Smoke-test a custom destination without touching real home directories: `rtk sh -c 'skill_tmp=$(mktemp -d); GOBIN="$skill_tmp/bin" ./install.sh --skill-dir "$skill_tmp/skill"; test -f "$skill_tmp/skill/references/ci.md"'`.
- [ ] Execute the three-run trigger protocol and workflow cases from `testdata/skill-evals/README.md` in fresh sessions; meet all stated thresholds.
- [ ] Review `rtk git diff --check` and `rtk git status --short`.

## Out of Scope

- Adding new MCP tools or changing CLI/JSON schemas.
- Claiming runtime performance improvements from bundle results.
- Automatic source-code edits without the user's authorization.
- A general Lighthouse, browser profiler, React/Vite, webpack, or Node bundle skill.
- A new documentation generator, benchmark service, or model-evaluation service.

## Self-Review Checklist

- [ ] Every audit finding maps to a task or an explicit out-of-scope statement.
- [ ] All created references are linked from `SKILL.md` and packaged by the installer.
- [ ] Tests validate behavior/invariants rather than exact prose except for prohibited misleading claims.
- [ ] Installer tests cover local, remote, default, and custom destinations.
- [ ] Eval corpus covers implicit, typo, positive, and near-miss negative prompts.
- [ ] No placeholder code, undefined interface, or unresolved design choice remains.
