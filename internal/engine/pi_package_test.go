package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/louisescher/hangar/internal/lockfile"
)

func TestPiPackageResourcesLifecycle(t *testing.T) {
	source := contentFixture(t, map[string]string{
		"package.json":                   `{"pi":{"skills":["./resources/tools"],"prompts":["./resources/templates/*.md"],"subagents":{"agents":["./resources/helpers"]}}}`,
		"resources/tools/check/SKILL.md": "---\nname: check\n---\nCheck code",
		"resources/templates/check.md":   "Check $ARGUMENTS",
		"resources/helpers/helper.md":    "---\nname: check\ndescription: Check code\ntools: read, grep\n---\nCheck changes",
	})
	base := t.TempDir()
	eng := New()
	s, err := eng.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	opt := InstallOptions{BaseDir: base, Agents: []string{"pi"}, All: true}
	report, err := eng.Install(context.Background(), s, opt)
	if err != nil || len(report.Skills) != 3 {
		t.Fatalf("install: %#v, %v", report, err)
	}
	lf, err := lockfile.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"skill:check", "prompt:check", "agent:check"} {
		if _, ok := lf.Find(key); !ok {
			t.Fatalf("missing %s", key)
		}
	}
	if entry, _ := lf.Find("agent:check"); entry.Subpath != "resources/helpers/helper.md" {
		t.Fatal(entry)
	}
	for _, path := range []string{".pi/skills/check/SKILL.md", ".pi/prompts/check.md", ".pi/agents/check.md"} {
		if _, err := os.Stat(filepath.Join(base, path)); err != nil {
			t.Fatal(err)
		}
	}
	changed := "Updated check $ARGUMENTS"
	if err := os.WriteFile(filepath.Join(source, "resources/templates/check.md"), []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	opt.Agents = nil
	if _, err := eng.Update(context.Background(), "prompt:check", opt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(base, ".pi/prompts/check.md"))
	if err != nil || string(data) != changed {
		t.Fatalf("update: %q, %v", data, err)
	}
	if err := os.Remove(filepath.Join(base, ".pi/agents/check.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Sync(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, ".pi/agents/check.md")); err != nil {
		t.Fatal(err)
	}
	// Removing a declaration must not rediscover the skill through its subpath.
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"pi":{"skills":[]}}`), 0644); err != nil {
		t.Fatal(err)
	}
	report, err = eng.Update(context.Background(), "skill:check", opt)
	if err != nil || len(report.Gone) != 1 || report.Gone[0] != "check" {
		t.Fatalf("removed declaration: %#v, %v", report, err)
	}
	for _, entry := range lf.Skills {
		if err := eng.Remove(entry.Key(), opt); err != nil {
			t.Fatal(err)
		}
	}
}
