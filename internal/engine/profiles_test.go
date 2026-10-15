package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/louisescher/hangar/internal/config"
	"github.com/louisescher/hangar/internal/lockfile"
)

func TestProfileMixedContentRestore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	source := contentFixture(t, map[string]string{
		"package.json":                   `{"pi":{"skills":["./resources/tools"],"prompts":["./resources/templates"],"subagents":["./resources/helpers"]}}`,
		"resources/tools/check/SKILL.md": "---\nname: check\n---\nCheck skill",
		"resources/templates/check.md":   "Check $ARGUMENTS",
		"resources/helpers/check.md":     "---\nname: check\n---\nCheck agent",
		"rules/check.md":                 "# Check rules\n",
	})
	base := t.TempDir()
	eng := New()
	s, err := eng.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Install(context.Background(), s, InstallOptions{BaseDir: base, Agents: []string{"pi"}, PiRulesFile: "AGENTS.override.md", All: true}); err != nil {
		t.Fatal(err)
	}
	p, err := eng.CaptureProfile("mixed", InstallOptions{BaseDir: base})
	if err != nil || p.Count() != 4 {
		t.Fatalf("capture: %#v, %v", p, err)
	}
	if err := config.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	p, err = config.LoadProfile(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	// No harness is detectable in the fresh destination; use each saved target.
	for _, global := range []bool{false, true} {
		dst := t.TempDir()
		t.Setenv("HOME", dst)
		if _, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: dst, Global: global}); err != nil {
			t.Fatal(err)
		}
		prefix := ".pi"
		rulesPath := "AGENTS.override.md"
		if global {
			prefix = ".pi/agent"
			rulesPath = ".pi/agent/AGENTS.override.md"
		}
		for _, path := range []string{prefix + "/skills/check/SKILL.md", prefix + "/agents/check.md", prefix + "/prompts/check.md", rulesPath} {
			if _, err := os.Stat(filepath.Join(dst, path)); err != nil {
				t.Fatalf("restore %s: %v", path, err)
			}
		}
		lf, err := lockfile.Load(dst)
		if err != nil || len(lf.Skills) != 4 {
			t.Fatalf("lockfile: %#v, %v", lf, err)
		}
	}
	dst := t.TempDir()
	if _, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: dst, Type: "prompt"}); err != nil {
		t.Fatal(err)
	}
	lf, _ := lockfile.Load(dst)
	if len(lf.Skills) != 1 || lf.Skills[0].Kind != lockfile.KindPrompt {
		t.Fatal(lf.Skills)
	}
	if _, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: t.TempDir(), Type: "rule", PiRulesFile: "APPEND_SYSTEM.md"}); err != nil {
		t.Fatal(err)
	}
}

func TestProfileNativePackageRestore(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	// Keep this focused on Hangar's installer invocation and settings merge.
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$PI_TEST_CALLS\"\nif [ -n \"$PI_TEST_FAIL\" ]; then echo failed >&2; exit 7; fi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PI_TEST_CALLS", calls)
	p := config.Profile{Name: "packages", Packages: []config.PiPackage{{Source: "npm:@tintinweb/pi-subagents@0.22.0", Options: map[string]any{"skills": []string{}, "autoload": false}}}}
	eng := New()
	for _, global := range []bool{false, true} {
		base := t.TempDir()
		rep, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: base, Global: global})
		if err != nil || len(rep.Packages) != 1 {
			t.Fatalf("restore: %#v, %v", rep, err)
		}
		captured, err := eng.CaptureProfile("again", InstallOptions{BaseDir: base, Global: global})
		if err != nil || len(captured.Packages) != 1 || captured.Packages[0].Options["autoload"] != false {
			t.Fatalf("recapture: %#v, %v", captured, err)
		}
	}
	data, _ := os.ReadFile(calls)
	if string(data) != "install npm:@tintinweb/pi-subagents@0.22.0 --local --approve\ninstall npm:@tintinweb/pi-subagents@0.22.0\n" {
		t.Fatalf("Pi commands: %q", data)
	}
	t.Setenv("PI_TEST_FAIL", "1")
	base := t.TempDir()
	if _, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: base}); err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("installer failure: %v", err)
	}
	if pkgs, _ := config.ReadPiPackages(base, false); len(pkgs) != 0 {
		t.Fatal("recorded failed installation")
	}
	if _, err := eng.RestoreProfile(context.Background(), p, InstallOptions{BaseDir: base, Agents: []string{"cursor"}}); err == nil || !strings.Contains(err.Error(), "target Pi") {
		t.Fatalf("unsupported harness: %v", err)
	}
}
