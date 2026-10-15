package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/louisescher/hangar/internal/lockfile"
)

func TestMixedProfileRoundTripAndSelection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := Profile{Name: "mixed", Packages: []PiPackage{{Source: "npm:@tintinweb/pi-subagents@0.22.0", Options: map[string]any{"skills": []string{}, "extensions": []string{"./src/index.ts"}, "autoload": false}}}}
	for _, kind := range []string{lockfile.KindSkill, lockfile.KindRef, lockfile.KindAgent, lockfile.KindRule, lockfile.KindPrompt} {
		p.Skills = append(p.Skills, lockfile.Entry{Name: "check", Kind: kind, Source: "file:///content", Subpath: "resources/check", Harnesses: []string{"pi"}, PiRulesFile: "AGENTS.override.md", PiPackage: true})
	}
	if err := SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProfile(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 6 || got.Skills[3].PiRulesFile != "AGENTS.override.md" || !got.Skills[0].PiPackage || got.Skills[0].Harnesses[0] != "pi" {
		t.Fatal(got)
	}
	data, _ := json.Marshal(got.Packages[0].Options)
	if !strings.Contains(string(data), `"skills":[]`) || !strings.Contains(string(data), `"autoload":false`) {
		t.Fatalf("filters changed: %s", data)
	}
	if _, err := got.RemoveEntry("check"); err == nil {
		t.Fatal("ambiguous name accepted")
	}
	if ok, err := got.RemoveEntry("agent:check"); err != nil || !ok || got.Count() != 5 {
		t.Fatalf("remove: %v, %v", ok, err)
	}
	if selected, err := got.Filter("package"); err != nil || selected.Count() != 1 {
		t.Fatalf("package filter: %#v, %v", selected, err)
	}
	if selected, err := got.Filter("prompt"); err != nil || selected.Count() != 1 || len(selected.Packages) != 0 {
		t.Fatalf("prompt filter: %#v, %v", selected, err)
	}
	if len(p.Skills) != 5 {
		t.Fatal("filter mutated original")
	}
}

func TestPiPackageSettingsMergeAndScope(t *testing.T) {
	base := t.TempDir()
	path := piSettingsPath(base, false)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := `{"theme":"original","packages":["../local",{"source":"npm:tools","skills":[],"autoload":false}]}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	packages, err := ReadPiPackages(base, false)
	if err != nil || len(packages) != 2 || packages[0].Source != filepath.Join(base, "local") {
		t.Fatalf("read: %#v, %v", packages, err)
	}
	if global, err := ReadPiPackages(base, true); err != nil || len(global) != 0 {
		t.Fatalf("scope mixed: %#v, %v", global, err)
	}
	pkg := PiPackage{Source: "npm:tools", Options: map[string]any{"prompts": []string{}, "extensions": []string{"./index.ts"}}}
	for range 2 {
		if err := SetPiPackage(base, false, pkg); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	items := settings["packages"].([]any)
	if len(items) != 2 || items[0] != "../local" || settings["theme"] != "original" {
		t.Fatalf("unrelated settings changed: %s", data)
	}
	if !reflect.DeepEqual(items[1].(map[string]any)["prompts"], []any{}) {
		t.Fatalf("empty filters changed: %s", data)
	}
	if err := os.WriteFile(path, []byte(`{"packages":false}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPiPackages(base, false); err == nil {
		t.Fatal("invalid packages accepted")
	}
}
