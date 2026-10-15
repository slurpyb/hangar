package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/louisescher/hangar/internal/lockfile"
	"github.com/louisescher/hangar/internal/security/sanitize"
)

func contentFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPiContentLifecycle(t *testing.T) {
	agent := "---\nname: review\ndescription: Review code\ntools: read, grep, find\nthinking: high\nmax_turns: 4\n---\nReview carefully\u200b"
	src := contentFixture(t, map[string]string{
		".pi/agents/definition.md":      agent,
		".pi/skills/review/SKILL.md":    "---\nname: review\n---\nA skill",
		"prompts/review.md":             "---\ndescription: Review changes\n---\nReview $ARGUMENTS\u200b",
		"rules/00-general.md":           "# General\n",
		"rules/backend/rules/10-api.md": "# API\n",
	})
	base := t.TempDir()
	eng := New()
	s, err := eng.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	d, err := eng.Discover(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if len(d.Skills) != 4 {
		t.Fatalf("discovered %#v", d.Skills)
	}
	opt := InstallOptions{BaseDir: base, Agents: []string{"pi"}, All: true, Security: sanitize.InvisibleOnly}
	rep, err := eng.Install(context.Background(), s, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skills) != 4 {
		t.Fatalf("installed %#v", rep)
	}
	for _, p := range []string{".pi/agents/review.md", ".pi/prompts/review.md", ".pi/skills/review/SKILL.md", ".pi/APPEND_SYSTEM.md"} {
		data, err := os.ReadFile(filepath.Join(base, p))
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsRune(string(data), '\u200b') {
			t.Fatal("content not sanitized")
		}
	}
	instruction, _ := os.ReadFile(filepath.Join(base, ".pi/APPEND_SYSTEM.md"))
	if string(instruction) != "# General\n\n# API\n" {
		t.Fatalf("got %q", instruction)
	}
	lf, err := lockfile.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Skills) != 4 {
		t.Fatal(lf.Skills)
	}
	if _, _, err := lf.Lookup("review"); err == nil {
		t.Fatal("ambiguous name accepted")
	}
	for _, key := range []string{"skill:review", "prompt:review", "agent:review"} {
		if _, ok := lf.Find(key); !ok {
			t.Fatal(key)
		}
	}
	installedAgent, err := os.ReadFile(filepath.Join(base, ".pi/agents/review.md"))
	if err != nil || string(installedAgent) != sanitize.Skill(agent, sanitize.InvisibleOnly) {
		t.Fatalf("provider definition changed: %q, %v", installedAgent, err)
	}
	t.Chdir(base)
	info, err := eng.InstalledInfo("agent:review", false)
	if err != nil || info == nil {
		t.Fatalf("installed agent info: %v", err)
	}
	doc, err := eng.ReadSkillDoc(info.Skills[0])
	if err != nil || !strings.Contains(doc.Body, "Review carefully") || !strings.Contains(doc.Skill.FrontmatterRaw, "max_turns: 4") {
		t.Fatalf("agent preview: %#v, %v", doc, err)
	}
	// Updating uses the recorded harness even when it is not auto-detected.
	os.WriteFile(filepath.Join(src, "prompts/review.md"), []byte("Changed"), 0644)
	opt.Agents = nil
	pending, err := eng.PreviewUpdate(context.Background(), "prompt:review", opt)
	if err != nil || !strings.Contains(pending, "Changed") {
		t.Fatalf("preview: %s, %v", pending, err)
	}
	if _, err := eng.Update(context.Background(), "prompt:review", opt); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(base, ".pi/prompts/review.md"))
	if string(data) != "Changed" {
		t.Fatalf("update: %q", data)
	}
	changedAgent := strings.Replace(agent, "Review carefully", "Review the API", 1)
	if err := os.WriteFile(filepath.Join(src, ".pi/agents/definition.md"), []byte(changedAgent), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Update(context.Background(), "agent:review", opt); err != nil {
		t.Fatal(err)
	}
	installedAgent, err = os.ReadFile(filepath.Join(base, ".pi/agents/review.md"))
	if err != nil || string(installedAgent) != sanitize.Skill(changedAgent, sanitize.InvisibleOnly) {
		t.Fatalf("agent update: %q, %v", installedAgent, err)
	}
	if _, err := eng.Sync(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	for _, entry := range lf.Skills {
		if err := eng.Remove(entry.Key(), opt); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{".pi/agents/review.md", ".pi/prompts/review.md", ".pi/APPEND_SYSTEM.md"} {
		if _, err := os.Lstat(filepath.Join(base, p)); !os.IsNotExist(err) {
			t.Fatalf("left %s", p)
		}
	}
}

func TestPiUnsupportedAndUnmanaged(t *testing.T) {
	eng := New()
	src := contentFixture(t, map[string]string{"agents/reviewer.toml": `name = "reviewer"
description = "Review"
developer_instructions = "Review code"`})
	s, _ := eng.Parse(src)
	base := t.TempDir()
	if _, err := eng.Install(context.Background(), s, InstallOptions{BaseDir: base, Agents: []string{"pi"}, All: true}); err == nil || !strings.Contains(err.Error(), "@tintinweb/pi-subagents require Markdown .md") {
		t.Fatalf("got %v", err)
	}
	src = contentFixture(t, map[string]string{"rules/one.md": "Rules"})
	s, _ = eng.Parse(src)
	os.MkdirAll(filepath.Join(base, ".pi"), 0755)
	os.WriteFile(filepath.Join(base, ".pi/APPEND_SYSTEM.md"), []byte("User instructions"), 0644)
	if _, err := eng.Install(context.Background(), s, InstallOptions{BaseDir: base, Agents: []string{"pi"}, All: true}); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("got %v", err)
	}
}

func TestHarnessContentSmoke(t *testing.T) {
	eng := New()
	ctx := context.Background()
	codex := contentFixture(t, map[string]string{
		".codex/agents/helper.toml": "name = \"helper\"\ndescription = \"Review\"\ndeveloper_instructions = \"Review code\"\n",
		".codex/rules/git.rules":    "prefix_rule(pattern = [\"git\", \"push\"], decision = \"prompt\")\n",
		"rules/general.md":          "# Agreement\n",
	})
	base := t.TempDir()
	s, _ := eng.Parse(codex)
	if _, err := eng.Install(ctx, s, InstallOptions{BaseDir: base, Agents: []string{"codex"}, All: true}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".codex/agents/helper.toml", ".codex/rules/git.rules", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(base, p)); err != nil {
			t.Fatal(err)
		}
	}
	cursor := contentFixture(t, map[string]string{
		".cursor/agents/helper.md":        "---\nname: helper\ndescription: Review\n---\nReview code",
		".cursor/commands/review.md":      "Review changes",
		".cursor/rules/backend/style.mdc": "---\nalwaysApply: true\n---\nUse Go\u200b",
	})
	base = t.TempDir()
	s, _ = eng.Parse(cursor)
	if _, err := eng.Install(ctx, s, InstallOptions{BaseDir: base, Agents: []string{"cursor"}, All: true, Security: sanitize.InvisibleOnly}); err != nil {
		t.Fatal(err)
	}
	d, err := eng.Discover(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ruleName := ""
	for _, sk := range d.Skills {
		if sk.Type() == "rule" {
			ruleName = sk.Name
		}
	}
	for _, p := range []string{".cursor/agents/helper.md", ".cursor/commands/review.md", filepath.Join(".cursor/rules", ruleName, "backend/style.mdc")} {
		data, err := os.ReadFile(filepath.Join(base, p))
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsRune(string(data), '\u200b') {
			t.Fatal("mdc sanitization missing")
		}
	}
	// Explicitly unsupported combinations fail, rather than installing inert files.
	s, _ = eng.Parse(cursor)
	if _, err := eng.Install(ctx, s, InstallOptions{BaseDir: t.TempDir(), Agents: []string{"codex"}, Type: "prompt", All: true}); err == nil || !strings.Contains(err.Error(), "global-only") {
		t.Fatalf("got %v", err)
	}
	if _, err := eng.Install(ctx, s, InstallOptions{BaseDir: t.TempDir(), Agents: []string{"cursor"}, Global: true, Type: "rule", All: true}); err == nil || !strings.Contains(err.Error(), "project-only") {
		t.Fatalf("got %v", err)
	}
	// Codex legacy prompts and Pi templates load from their distinct global homes.
	for _, harness := range []string{"pi", "codex", "cursor"} {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if _, err := eng.Install(ctx, s, InstallOptions{BaseDir: home, Agents: []string{harness}, Global: true, Type: "prompt", All: true}); err != nil {
			t.Fatal(err)
		}
		prefix := map[string]string{"pi": ".pi/agent/prompts", "codex": ".codex/prompts", "cursor": ".cursor/commands"}[harness]
		if _, err := os.Stat(filepath.Join(home, prefix, "review.md")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPiSubagentsGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := contentFixture(t, map[string]string{"agents/reviewer.md": "Review the current changes.\n"})
	eng := New()
	spec, err := eng.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	opt := InstallOptions{Agents: []string{"pi"}, Global: true, Type: "agent", All: true}
	if _, err := eng.Install(context.Background(), spec, opt); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(home, ".pi/agent/agents/reviewer.md")
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("global definition should be a copied file: %v", info.Mode())
	}
	raw, err := os.ReadFile(dest)
	if err != nil || string(raw) != "Review the current changes.\n" {
		t.Fatalf("global definition: %q, %v", raw, err)
	}
	opt.Agents = nil
	if err := eng.Remove("agent:reviewer", opt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatalf("global agent left after removal: %v", err)
	}
}
