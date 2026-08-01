package fsx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func ValidateOutput(input, output string) (string, error) {
	if output == "" {
		return "", errors.New("output directory is required")
	}
	inAbs, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	outAbs, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	if inAbs == outAbs || strings.HasPrefix(outAbs+string(filepath.Separator), inAbs+string(filepath.Separator)) {
		return "", errors.New("output must not be the input directory or a descendant of it")
	}
	if _, err := os.Lstat(outAbs); err == nil {
		return "", fmt.Errorf("output already exists: %s", outAbs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return outAbs, nil
}

func Stage(output string) (string, func(bool) error, error) {
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, err
	}
	stage, err := os.MkdirTemp(parent, ".plugxfer-tmp-")
	if err != nil {
		return "", nil, err
	}
	finish := func(commit bool) error {
		if !commit {
			return os.RemoveAll(stage)
		}
		if err := os.Rename(stage, output); err != nil {
			_ = os.RemoveAll(stage)
			return err
		}
		return nil
	}
	return stage, finish, nil
}

func Write(root string, files []model.File) error {
	for _, file := range files {
		clean := filepath.Clean(filepath.FromSlash(file.Path))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("unsafe output path %q", file.Path)
		}
		path := filepath.Join(root, clean)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(file.Mode)
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(path, file.Data, mode); err != nil {
			return err
		}
	}
	return nil
}
