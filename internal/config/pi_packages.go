package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/louisescher/hangar/internal/fsutil"
)

func piSettingsPath(base string, global bool) string {
	if global {
		return filepath.Join(base, ".pi", "agent", "settings.json")
	}
	return filepath.Join(base, ".pi", "settings.json")
}

func readPiSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		return nil, fmt.Errorf("%s must contain a settings object", path)
	}
	return settings, nil
}

// Package paths in Pi settings are relative to the settings directory. Store
// local sources as absolute paths so applying elsewhere keeps the same source.
func piPackageSource(source, settingsDir string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("Pi package source must not be empty")
	}
	if strings.Contains(source, ":") || strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "github.com/") {
		return source, nil
	}
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, source[2:]), nil
	}
	if filepath.IsAbs(source) {
		return filepath.Clean(source), nil
	}
	return filepath.Abs(filepath.Join(settingsDir, source))
}

func piPackagesFromSettings(settings map[string]json.RawMessage, dir string) ([]PiPackage, error) {
	var raw []json.RawMessage
	if data, ok := settings["packages"]; ok {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("Pi settings packages: %w", err)
		}
	}
	var packages []PiPackage
	for _, item := range raw {
		var pkg PiPackage
		if len(item) > 0 && item[0] == '"' {
			if err := json.Unmarshal(item, &pkg.Source); err != nil {
				return nil, err
			}
		} else {
			if err := json.Unmarshal(item, &pkg.Options); err != nil {
				return nil, err
			}
			pkg.Source, _ = pkg.Options["source"].(string)
			delete(pkg.Options, "source")
		}
		var err error
		pkg.Source, err = piPackageSource(pkg.Source, dir)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// ReadPiPackages captures only the selected scope's native package declarations.
func ReadPiPackages(base string, global bool) ([]PiPackage, error) {
	path := piSettingsPath(base, global)
	settings, err := readPiSettings(path)
	if err != nil {
		return nil, err
	}
	return piPackagesFromSettings(settings, filepath.Dir(path))
}

// SetPiPackage restores a declaration after pi install, keeping other packages
// and unrelated settings. Pi itself handles source identity and installation.
func SetPiPackage(base string, global bool, pkg PiPackage) error {
	path := piSettingsPath(base, global)
	settings, err := readPiSettings(path)
	if err != nil {
		return err
	}
	packages, err := piPackagesFromSettings(settings, filepath.Dir(path))
	if err != nil {
		return err
	}
	var declarations []json.RawMessage
	if data, ok := settings["packages"]; ok {
		if err := json.Unmarshal(data, &declarations); err != nil {
			return err
		}
	}
	var declaration any = pkg.Source
	if len(pkg.Options) > 0 {
		object := map[string]any{"source": pkg.Source}
		for key, value := range pkg.Options {
			object[key] = value
		}
		declaration = object
	}
	raw, err := json.Marshal(declaration)
	if err != nil {
		return err
	}
	found := false
	for i, existing := range packages {
		if existing.Source == pkg.Source {
			declarations[i], found = raw, true
			break
		}
	}
	if !found {
		declarations = append(declarations, raw)
	}
	settings["packages"], err = json.Marshal(declarations)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.AtomicWriteFile(path, append(data, '\n'), 0644)
}
