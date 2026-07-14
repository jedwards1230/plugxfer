package answers

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Version      int                          `yaml:"version"`
	Models       map[string]map[string]string `yaml:"models,omitempty"`
	Replacements map[string]string            `yaml:"replacements,omitempty"`
}

func Empty() *File {
	return &File{Version: 1, Models: map[string]map[string]string{}, Replacements: map[string]string{}}
}

func Load(path string) (*File, error) {
	if path == "" {
		return Empty(), nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read map file: %w", err)
	}
	answer := Empty()
	if err := yaml.Unmarshal(data, answer); err != nil {
		return nil, fmt.Errorf("parse map file: %w", err)
	}
	if answer.Version != 1 {
		return nil, fmt.Errorf("unsupported map file version %d", answer.Version)
	}
	if answer.Models == nil {
		answer.Models = map[string]map[string]string{}
	}
	if answer.Replacements == nil {
		answer.Replacements = map[string]string{}
	}
	return answer, nil
}

func (f *File) Model(direction, value string) (string, bool) {
	values := f.Models[direction]
	mapped, ok := values[value]
	return mapped, ok && mapped != ""
}

func (f *File) StubModel(direction, value string) {
	if f.Models[direction] == nil {
		f.Models[direction] = map[string]string{}
	}
	if _, ok := f.Models[direction][value]; !ok {
		f.Models[direction][value] = ""
	}
}

func (f *File) StubReplacement(locator string) {
	if _, ok := f.Replacements[locator]; !ok {
		f.Replacements[locator] = ""
	}
}

func (f *File) HasStubs() bool {
	for _, values := range f.Models {
		for _, value := range values {
			if value == "" {
				return true
			}
		}
	}
	for _, value := range f.Replacements {
		if value == "" {
			return true
		}
	}
	return false
}

func (f *File) Marshal() ([]byte, error) {
	// yaml.v3 sorts string map keys, but rebuild the direction map for stable nil handling.
	for direction, values := range f.Models {
		if len(values) == 0 {
			delete(f.Models, direction)
		}
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}
