package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/answers"
	"github.com/jedwards1230/plugxfer/internal/engine"
	"github.com/jedwards1230/plugxfer/internal/fsx"
	"github.com/jedwards1230/plugxfer/internal/marketplace"
	"github.com/jedwards1230/plugxfer/internal/model"
	"github.com/jedwards1230/plugxfer/internal/reader"
	"github.com/jedwards1230/plugxfer/internal/report"
	"github.com/jedwards1230/plugxfer/internal/rulebook"
)

type Mode string

const (
	Check   Mode = "check"
	Convert Mode = "convert"
)

type Options struct {
	Mode      Mode
	Input     string
	Output    string
	Target    model.Dialect
	MapPath   string
	RulesPath string
	Strict    bool
	Only      map[string]bool
}

type Result struct {
	Report   model.Report
	Markdown []byte
	ExitCode int
}

func Run(options Options) (Result, error) {
	if options.Mode != Check && options.Mode != Convert {
		return Result{}, errors.New("invalid mode")
	}
	absInput, err := filepath.Abs(options.Input)
	if err != nil {
		return Result{}, err
	}
	source, market, err := reader.Detect(absInput, options.Target)
	if err != nil {
		return Result{}, err
	}
	target := options.Target
	if !target.Valid() {
		target = source.Other()
	}
	if target == source {
		return Result{}, fmt.Errorf("input is already %s; --to must select the other dialect", target)
	}
	book, err := rulebook.Load(options.RulesPath)
	if err != nil {
		return Result{}, err
	}
	answerFile, err := answers.Load(options.MapPath)
	if err != nil {
		return Result{}, err
	}
	converter := &engine.Converter{Rules: book, Answers: answerFile}
	if market {
		return runMarketplace(options, absInput, source, target, converter, answerFile)
	}
	return runPlugin(options, absInput, source, target, converter, answerFile)
}

func runPlugin(options Options, input string, source, target model.Dialect, converter *engine.Converter, answerFile *answers.File) (Result, error) {
	plugin, err := reader.Read(input, source)
	if err != nil {
		return Result{}, err
	}
	outputLabel := options.Output
	files, conversionReport, err := converter.Plugin(plugin, input, outputLabel)
	if err != nil {
		return Result{}, err
	}
	conversionReport.Target = target
	markdown := report.Markdown(conversionReport)
	if options.Mode == Convert {
		output, err := fsx.ValidateOutput(input, options.Output)
		if err != nil {
			return Result{}, err
		}
		conversionReport.Output = output
		markdown = report.Markdown(conversionReport)
		files = append(files, model.File{Path: "PLUGXFER-REPORT.md", Mode: 0o644, Data: markdown})
		if answerFile.HasStubs() {
			data, err := answerFile.Marshal()
			if err != nil {
				return Result{}, err
			}
			files = append(files, model.File{Path: "plugxfer.map.yaml", Mode: 0o644, Data: data})
		}
		if err := writeAtomic(output, files); err != nil {
			return Result{}, err
		}
	}
	code := conversionReport.ExitCode(options.Strict)
	return Result{Report: conversionReport, Markdown: markdown, ExitCode: code}, nil
}

func runMarketplace(options Options, input string, source, target model.Dialect, converter *engine.Converter, answerFile *answers.File) (Result, error) {
	registryPath := ".claude-plugin/marketplace.json"
	if source == model.Codex {
		registryPath = ".agents/plugins/marketplace.json"
	}
	registryFullPath := filepath.Join(input, filepath.FromSlash(registryPath))
	registryData, err := os.ReadFile(registryFullPath)
	if err != nil {
		return Result{}, fmt.Errorf("read marketplace registry: %w", err)
	}
	registryInfo, err := os.Stat(registryFullPath)
	if err != nil {
		return Result{}, fmt.Errorf("stat marketplace registry: %w", err)
	}
	registryFile := model.File{Path: registryPath, Mode: uint32(registryInfo.Mode().Perm()), Data: registryData}
	registry, err := marketplace.Parse(registryFile, source)
	if err != nil {
		return Result{}, err
	}
	if err := validateOnly(registry, options.Only); err != nil {
		return Result{}, err
	}
	aggregate := model.Report{Source: source, Target: target, Input: input, Output: options.Output, Marketplace: true}
	var outputFiles []model.File
	for _, entry := range registry.Entries {
		rel, local, err := entry.LocalPath()
		if err != nil {
			return Result{}, err
		}
		if !local {
			if len(options.Only) > 0 && !options.Only[entry.Name] {
				continue
			}
			aggregate.Add(model.Finding{Component: "marketplace", Status: model.Skipped, Path: registryPath, Class: model.Info, Severity: model.Low, Message: fmt.Sprintf("remote plugin %s was not fetched: %v", entry.Name, entry.Source)})
			aggregate.Children = append(aggregate.Children, model.ChildReport{Name: entry.Name, Source: fmt.Sprint(entry.Source), ExitCode: 0})
			continue
		}
		pluginRoot := filepath.Join(input, filepath.FromSlash(rel))
		if len(options.Only) > 0 && !options.Only[entry.Name] {
			plugin, err := reader.Read(pluginRoot, source)
			if err != nil {
				return Result{}, fmt.Errorf("unselected plugin %s: %w", entry.Name, err)
			}
			for _, file := range plugin.Files {
				file.Path = filepath.ToSlash(filepath.Join(rel, file.Path))
				outputFiles = append(outputFiles, file)
			}
			aggregate.Add(model.Finding{Component: "marketplace", Status: model.Skipped, Path: registryPath, Class: model.Info, Severity: model.Low, Message: "preserved unselected local plugin " + entry.Name})
			continue
		}
		actualSource, _, err := reader.Detect(pluginRoot, target)
		if err != nil {
			return Result{}, fmt.Errorf("plugin %s: %w", entry.Name, err)
		}
		if actualSource != source {
			return Result{}, fmt.Errorf("plugin %s dialect %s does not match marketplace %s", entry.Name, actualSource, source)
		}
		plugin, err := reader.Read(pluginRoot, source)
		if err != nil {
			return Result{}, fmt.Errorf("plugin %s: %w", entry.Name, err)
		}
		childFiles, childReport, err := converter.Plugin(plugin, rel, rel)
		if err != nil {
			return Result{}, fmt.Errorf("plugin %s: %w", entry.Name, err)
		}
		childReport.Target = target
		childMarkdown := report.Markdown(childReport)
		childReportPath := filepath.ToSlash(filepath.Join(rel, "PLUGXFER-REPORT.md"))
		if options.Mode == Check {
			childReportPath = ""
		}
		aggregate.Children = append(aggregate.Children, model.ChildReport{Name: entry.Name, Source: rel, ReportPath: childReportPath, ExitCode: childReport.ExitCode(options.Strict), Counts: childReport.Counts()})
		for _, file := range childFiles {
			file.Path = filepath.ToSlash(filepath.Join(rel, file.Path))
			outputFiles = append(outputFiles, file)
		}
		if options.Mode == Convert {
			outputFiles = append(outputFiles, model.File{Path: childReportPath, Mode: 0o644, Data: childMarkdown})
		}
	}
	convertedRegistry, err := registry.Render(target)
	if err != nil {
		return Result{}, err
	}
	outputFiles = append(outputFiles, convertedRegistry)
	if target == model.Codex {
		outputFiles = append(outputFiles, registryFile)
	}
	markdown := report.Markdown(aggregate)
	if options.Mode == Convert {
		output, err := fsx.ValidateOutput(input, options.Output)
		if err != nil {
			return Result{}, err
		}
		aggregate.Output = output
		markdown = report.Markdown(aggregate)
		outputFiles = append(outputFiles, model.File{Path: "PLUGXFER-REPORT.md", Mode: 0o644, Data: markdown})
		if answerFile.HasStubs() {
			data, err := answerFile.Marshal()
			if err != nil {
				return Result{}, err
			}
			outputFiles = append(outputFiles, model.File{Path: "plugxfer.map.yaml", Mode: 0o644, Data: data})
		}
		if err := writeAtomic(output, outputFiles); err != nil {
			return Result{}, err
		}
	}
	return Result{Report: aggregate, Markdown: markdown, ExitCode: aggregate.ExitCode(options.Strict)}, nil
}

func writeAtomic(output string, files []model.File) (err error) {
	stage, finish, err := fsx.Stage(output)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = finish(false)
		}
	}()
	if err := dedupe(files); err != nil {
		return err
	}
	if err := fsx.Write(stage, files); err != nil {
		return err
	}
	if err := finish(true); err != nil {
		return err
	}
	committed = true
	return nil
}

func dedupe(files []model.File) error {
	seen := map[string]bool{}
	for _, file := range files {
		if seen[file.Path] {
			return fmt.Errorf("duplicate output path %s", file.Path)
		}
		seen[file.Path] = true
	}
	return nil
}

func validateOnly(registry *marketplace.Registry, only map[string]bool) error {
	if len(only) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, entry := range registry.Entries {
		if only[entry.Name] {
			found[entry.Name] = true
		}
	}
	var missing []string
	for name := range only {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("--only plugin(s) not found: %s", strings.Join(missing, ", "))
	}
	return nil
}

func DefaultMapPath(cwd string) string {
	path := filepath.Join(cwd, "plugxfer.map.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}
