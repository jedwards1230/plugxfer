package rulebook

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jedwards1230/plugxfer/internal/model"
	"gopkg.in/yaml.v3"
)

//go:embed default/*.yaml
var defaults embed.FS

type Feature struct {
	ID            string `yaml:"id"`
	ClaudeToCodex string `yaml:"claude_to_codex"`
	CodexToClaude string `yaml:"codex_to_claude"`
}

type Detector struct {
	ID            string         `yaml:"id"`
	Pattern       string         `yaml:"pattern"`
	ClaudeToCodex string         `yaml:"claude_to_codex"`
	CodexToClaude string         `yaml:"codex_to_claude"`
	Severity      string         `yaml:"severity"`
	Regexp        *regexp.Regexp `yaml:"-"`
}

type featureFile struct {
	Version  int                  `yaml:"version"`
	Verified *model.RulesVerified `yaml:"verified"`
	Features []Feature            `yaml:"features"`
}

type detectorFile struct {
	Version   int        `yaml:"version"`
	Detectors []Detector `yaml:"detectors"`
}

type DirectionMap struct {
	ClaudeToCodex map[string]string `yaml:"claude_to_codex"`
	CodexToClaude map[string]string `yaml:"codex_to_claude"`
}

type valueFile struct {
	Version        int          `yaml:"version"`
	Effort         DirectionMap `yaml:"effort"`
	PermissionMode DirectionMap `yaml:"permission_mode"`
	HookEvents     DirectionMap `yaml:"hook_events"`
}

type Book struct {
	Features       map[string]Feature
	Detectors      []Detector
	Effort         DirectionMap
	PermissionMode DirectionMap
	HookEvents     DirectionMap
	// Verified records the upstream CLI specs this rulebook was confirmed
	// against. It is optional: custom --rules directories that omit the
	// `verified:` block leave it nil and the report header line is skipped.
	Verified *model.RulesVerified
}

var strategies = map[string]bool{
	"reshape-manifest": true, "copy-skills": true, "mcp-env-rewrite": true,
	"hook-filter": true, "fold-into-skill": true, "unfold-command-skill": true,
	"md-toml-agent": true, "rename-path": true, "copy-warn": true,
	"drop-warn": true, "copy": true,
}

func Load(override string) (*Book, error) {
	var source fs.FS = defaults
	prefix := "default"
	if override != "" {
		source = os.DirFS(override)
		prefix = "."
	}
	read := func(name string, dst any) error {
		data, err := fs.ReadFile(source, filepath.ToSlash(filepath.Join(prefix, name)))
		if err != nil {
			return fmt.Errorf("read rulebook %s: %w", name, err)
		}
		if err := yaml.Unmarshal(data, dst); err != nil {
			return fmt.Errorf("parse rulebook %s: %w", name, err)
		}
		return nil
	}
	var ff featureFile
	var df detectorFile
	var vf valueFile
	if err := read("features.yaml", &ff); err != nil {
		return nil, err
	}
	if err := read("detectors.yaml", &df); err != nil {
		return nil, err
	}
	if err := read("values.yaml", &vf); err != nil {
		return nil, err
	}
	if ff.Version != 1 || df.Version != 1 || vf.Version != 1 {
		return nil, fmt.Errorf("unsupported rulebook version; expected 1")
	}
	b := &Book{Features: make(map[string]Feature), Detectors: df.Detectors, Effort: vf.Effort, PermissionMode: vf.PermissionMode, HookEvents: vf.HookEvents, Verified: ff.Verified}
	for _, feature := range ff.Features {
		if feature.ID == "" || !strategies[feature.ClaudeToCodex] || !strategies[feature.CodexToClaude] {
			return nil, fmt.Errorf("feature %q has an unknown strategy", feature.ID)
		}
		b.Features[feature.ID] = feature
	}
	for i := range b.Detectors {
		rx, err := regexp.Compile(b.Detectors[i].Pattern)
		if err != nil {
			return nil, fmt.Errorf("detector %q: %w", b.Detectors[i].ID, err)
		}
		b.Detectors[i].Regexp = rx
	}
	return b, nil
}

func (b *Book) Strategy(feature string, from model.Dialect) string {
	f := b.Features[feature]
	if from == model.Claude {
		return f.ClaudeToCodex
	}
	return f.CodexToClaude
}

func (m DirectionMap) Lookup(from model.Dialect, value string) (string, bool) {
	if from == model.Claude {
		v, ok := m.ClaudeToCodex[value]
		return v, ok
	}
	v, ok := m.CodexToClaude[value]
	return v, ok
}
