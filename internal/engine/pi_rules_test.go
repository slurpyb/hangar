package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/louisescher/hangar/internal/lockfile"
)

func TestPiRulesContextDestinations(t *testing.T) {
	for _, filename := range []string{"AGENTS.md", "AGENTS.override.md"} {
		t.Run(filename, func(t *testing.T) {
			source := contentFixture(t, map[string]string{"rules/a.md": "# Rules\n"})
			base := t.TempDir()
			eng := New()
			s, err := eng.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			opt := InstallOptions{BaseDir: base, Agents: []string{"pi"}, Type: "rule", All: true, PiRulesFile: filename}
			if _, err := eng.Install(context.Background(), s, opt); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(base, filename)
			content, err := os.ReadFile(dst)
			if err != nil || string(content) != "# Rules\n" {
				t.Fatalf("context output: %q, %v", content, err)
			}
			lf, err := lockfile.Load(base)
			if err != nil {
				t.Fatal(err)
			}
			entry, ok := lf.Find("rule:rules")
			if !ok || entry.PiRulesFile != filename {
				t.Fatalf("recorded choice: %#v", entry)
			}
			profileBase := t.TempDir()
			if _, err := eng.ApplyProfile(context.Background(), []lockfile.Entry{entry}, InstallOptions{BaseDir: profileBase}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(profileBase, filename)); err != nil {
				t.Fatalf("profile lost destination: %v", err)
			}
			if err := os.WriteFile(filepath.Join(source, "rules/a.md"), []byte("# Updated\n"), 0644); err != nil {
				t.Fatal(err)
			}
			opt.Agents = nil
			opt.PiRulesFile = ""
			if _, err := eng.Update(context.Background(), "rule:rules", opt); err != nil {
				t.Fatal(err)
			}
			content, err = os.ReadFile(dst)
			if err != nil || string(content) != "# Updated\n" {
				t.Fatalf("update: %q, %v", content, err)
			}
			if err := os.Remove(dst); err != nil {
				t.Fatal(err)
			}
			if _, err := eng.Sync(context.Background(), opt); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dst); err != nil {
				t.Fatalf("sync lost destination: %v", err)
			}
			if _, err := os.Stat(filepath.Join(base, ".pi/APPEND_SYSTEM.md")); !os.IsNotExist(err) {
				t.Fatal("context install also generated append-system file")
			}
			if err := eng.Remove("rule:rules", opt); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				t.Fatalf("context output left after removal: %v", err)
			}
		})
	}
	// Global context belongs in Pi's agent directory, rather than the home root.
	source := contentFixture(t, map[string]string{"rules/a.md": "Global rules"})
	home := t.TempDir()
	eng := New()
	s, _ := eng.Parse(source)
	if _, err := eng.Install(context.Background(), s, InstallOptions{BaseDir: home, Global: true, Agents: []string{"pi"}, PiRulesFile: "AGENTS.override.md", All: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi/agent/AGENTS.override.md")); err != nil {
		t.Fatal(err)
	}
}

func TestPiRulesChangeDestination(t *testing.T) {
	source := contentFixture(t, map[string]string{"rules/a.md": "# Managed\n"})
	base := t.TempDir()
	eng := New()
	s, _ := eng.Parse(source)
	opt := InstallOptions{BaseDir: base, Agents: []string{"pi"}, Type: "rule", All: true}
	if _, err := eng.Install(context.Background(), s, opt); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(base, "AGENTS.md")
	if err := os.WriteFile(userFile, []byte("My instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	opt.PiRulesFile = "AGENTS.md"
	if _, err := eng.Update(context.Background(), "rule:rules", opt); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("expected unmanaged-file error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, ".pi/APPEND_SYSTEM.md")); err != nil {
		t.Fatal("failed move removed the original output")
	}
	opt.PiRulesFile = "AGENTS.override.md"
	if _, err := eng.Update(context.Background(), "rule:rules", opt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "AGENTS.override.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, ".pi/APPEND_SYSTEM.md")); !os.IsNotExist(err) {
		t.Fatal("previous append-system output left behind")
	}
	content, err := os.ReadFile(userFile)
	if err != nil || string(content) != "My instructions" {
		t.Fatalf("unmanaged file modified: %q, %v", content, err)
	}
	opt.PiRulesFile = ""
	if _, err := eng.Update(context.Background(), "rule:rules", opt); err != nil {
		t.Fatal(err)
	}
	if err := eng.Remove("rule:rules", opt); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(userFile)
	if err != nil || string(content) != "My instructions" {
		t.Fatalf("unmanaged file removed: %q, %v", content, err)
	}
}

func TestPiRulesMoveRetainsSharedCodexOutput(t *testing.T) {
	source := contentFixture(t, map[string]string{"rules/a.md": "# Shared\n"})
	base := t.TempDir()
	eng := New()
	s, _ := eng.Parse(source)
	opt := InstallOptions{BaseDir: base, Agents: []string{"pi", "codex"}, PiRulesFile: "AGENTS.md", All: true}
	if _, err := eng.Install(context.Background(), s, opt); err != nil {
		t.Fatal(err)
	}
	opt.Agents = []string{"pi"}
	opt.PiRulesFile = "APPEND_SYSTEM.md"
	if _, err := eng.Update(context.Background(), "rule:rules", opt); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"AGENTS.md", ".pi/APPEND_SYSTEM.md"} {
		if _, err := os.Stat(filepath.Join(base, p)); err != nil {
			t.Fatal(err)
		}
	}
	lf, err := lockfile.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	en, _ := lf.Find("rule:rules")
	if len(en.Harnesses) != 2 {
		t.Fatalf("lost harness destinations: %#v", en)
	}
}

func TestPiRulesSeparateBundleDestinations(t *testing.T) {
	source := contentFixture(t, map[string]string{"context/rules/a.md": "Context guidance", "system/rules/b.md": "System additions"})
	base := t.TempDir()
	eng := New()
	s, _ := eng.Parse(source)
	opt := InstallOptions{BaseDir: base, Agents: []string{"pi"}, All: true}
	s.Skill = "rule:context-rules"
	opt.PiRulesFile = "AGENTS.md"
	if _, err := eng.Install(context.Background(), s, opt); err != nil {
		t.Fatal(err)
	}
	s.Skill = "rule:system-rules"
	opt.PiRulesFile = "APPEND_SYSTEM.md"
	if _, err := eng.Install(context.Background(), s, opt); err != nil {
		t.Fatal(err)
	}
	opt.PiRulesFile = ""
	opt.Agents = nil
	if _, err := eng.Sync(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{"AGENTS.md": "Context guidance", ".pi/APPEND_SYSTEM.md": "System additions"} {
		got, err := os.ReadFile(filepath.Join(base, p))
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q, %v", p, got, err)
		}
	}
}
