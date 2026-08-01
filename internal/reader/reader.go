package reader

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

const (
	MaxFiles     = 10_000
	MaxFileSize  = 8 << 20
	MaxTotalSize = 256 << 20
)

func Detect(root string, target model.Dialect) (model.Dialect, bool, error) {
	claudeManifest := exists(filepath.Join(root, ".claude-plugin", "plugin.json"))
	codexManifest := exists(filepath.Join(root, ".codex-plugin", "plugin.json"))
	claudeMarket := exists(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	codexMarket := exists(filepath.Join(root, ".agents", "plugins", "marketplace.json"))
	if (claudeManifest || claudeMarket) && (codexManifest || codexMarket) {
		if !target.Valid() {
			return "", false, errors.New("both Claude and Codex markers exist; pass --to")
		}
		return target.Other(), claudeMarket || codexMarket, nil
	}
	if claudeManifest || claudeMarket {
		return model.Claude, claudeMarket, nil
	}
	if codexManifest || codexMarket {
		return model.Codex, codexMarket, nil
	}
	claudeSignals := anyExists(root, "commands", "agents", "output-styles", ".lsp.json", "CLAUDE.md")
	codexSignals := anyExists(root, ".codex/agents", ".app.json", "AGENTS.md")
	if claudeSignals && codexSignals {
		if !target.Valid() {
			return "", false, errors.New("ambiguous component directories; pass --to")
		}
		return target.Other(), false, nil
	}
	if claudeSignals {
		return model.Claude, false, nil
	}
	if codexSignals {
		return model.Codex, false, nil
	}
	if exists(filepath.Join(root, "skills")) || exists(filepath.Join(root, ".mcp.json")) || exists(filepath.Join(root, "hooks")) {
		if !target.Valid() {
			return "", false, errors.New("shared-only plugin is ambiguous; pass --to")
		}
		return target.Other(), false, nil
	}
	return "", false, errors.New("no plugin or marketplace markers found")
}

func Read(root string, dialect model.Dialect) (*model.Plugin, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat input: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("input must be a directory")
	}
	p := &model.Plugin{Root: abs, Dialect: dialect}
	var total int64
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == abs {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported: %s", rel)
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || strings.HasPrefix(entry.Name(), ".plugxfer-tmp-") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(p.Files) >= MaxFiles {
			return fmt.Errorf("input exceeds %d files", MaxFiles)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxFileSize {
			return fmt.Errorf("file exceeds %d bytes: %s", MaxFileSize, rel)
		}
		total += info.Size()
		if total > MaxTotalSize {
			return fmt.Errorf("input exceeds %d bytes", MaxTotalSize)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		p.Files = append(p.Files, model.File{Path: rel, Mode: uint32(info.Mode().Perm()), Data: data})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	return p, nil
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func anyExists(root string, names ...string) bool {
	for _, name := range names {
		if exists(filepath.Join(root, filepath.FromSlash(name))) {
			return true
		}
	}
	return false
}
