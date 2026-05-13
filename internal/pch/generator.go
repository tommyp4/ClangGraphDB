package pch

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"clang-graphdb/internal/vcxproj"
)

type Result struct {
	PCHByDir  map[string]string // projectDir -> .pch file path
	SkipFiles map[string]bool   // absolute paths of PCH creator files to skip
	NoPCH     map[string]bool   // absolute paths of files that opt out of PCH
}

type task struct {
	projectName string
	projectDir  string
	pchHeader   string
	creatorFile string
	baseArgs    []string
	outputPath  string
}

func Plan(projects []*vcxproj.ParsedProject, targetConfig string, pchOutputDir string) []task {
	var tasks []task

	for _, proj := range projects {
		config, ok := proj.Configs[targetConfig]
		if !ok {
			continue
		}

		if strings.ToLower(config.PrecompiledHeader) != "use" || config.PCHFile == "" {
			continue
		}

		creatorFile := findCreatorFile(proj, targetConfig)
		if creatorFile == "" {
			continue
		}

		baseArgs := vcxproj.BuildProjectArgs(proj, config)
		absPCHDir, _ := filepath.Abs(pchOutputDir)
		outputPath := filepath.Join(absPCHDir, proj.Name+".pch")

		tasks = append(tasks, task{
			projectName: proj.Name,
			projectDir:  proj.AbsoluteDir,
			pchHeader:   config.PCHFile,
			creatorFile: creatorFile,
			baseArgs:    baseArgs,
			outputPath:  outputPath,
		})
	}

	return tasks
}

func Generate(tasks []task, clangPath string, workers int, verbose bool) *Result {
	result := &Result{
		PCHByDir:  make(map[string]string),
		SkipFiles: make(map[string]bool),
		NoPCH:     make(map[string]bool),
	}

	if err := os.MkdirAll(filepath.Dir(tasks[0].outputPath), 0755); err != nil {
		log.Printf("  Warning: failed to create PCH output directory: %v", err)
		return result
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan task, len(tasks))
	for _, t := range tasks {
		work <- t
	}
	close(work)

	succeeded := 0
	failed := 0

	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range work {
				err := generatePCH(t, clangPath)
				mu.Lock()
				if err != nil {
					failed++
					log.Printf("  [PCH FAIL] %s: %v", t.projectName, err)
				} else {
					succeeded++
					result.PCHByDir[t.projectDir] = t.outputPath
					result.SkipFiles[t.creatorFile] = true
					if verbose {
						log.Printf("  [PCH OK] %s -> %s", t.projectName, filepath.Base(t.outputPath))
					}
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	log.Printf("  PCH generation: %d succeeded, %d failed", succeeded, failed)
	return result
}

func PopulateNoPCH(result *Result, projects []*vcxproj.ParsedProject, targetConfig string) {
	for _, proj := range projects {
		if _, hasPCH := result.PCHByDir[proj.AbsoluteDir]; !hasPCH {
			continue
		}
		for _, sf := range proj.SourceFiles {
			override := strings.ToLower(sf.PCHOverride)
			if override == "notusing" || override == "create" {
				result.NoPCH[filepath.Clean(sf.AbsolutePath)] = true
			}
		}
	}
}

func generatePCH(t task, clangPath string) error {
	args := make([]string, 0, len(t.baseArgs)+6)
	args = append(args, "/Yc"+t.pchHeader)
	args = append(args, "/Fp"+t.outputPath)

	for _, a := range t.baseArgs[1:] {
		args = append(args, a)
	}

	args = append(args, "/c", t.creatorFile)

	cmd := exec.Command(clangPath, args...)
	cmd.Dir = t.projectDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clang-cl PCH creation failed: %w\n%s", err, string(output))
	}

	if _, err := os.Stat(t.outputPath); err != nil {
		return fmt.Errorf("PCH file not created at %s", t.outputPath)
	}

	return nil
}

func findCreatorFile(proj *vcxproj.ParsedProject, targetConfig string) string {
	config := proj.Configs[targetConfig]
	if config == nil {
		return ""
	}

	for _, sf := range proj.SourceFiles {
		if strings.ToLower(sf.PCHOverride) == "create" {
			return filepath.Clean(sf.AbsolutePath)
		}
	}

	// Fallback: look for a file matching the PCH header name (stdafx.h -> stdafx.cpp)
	pchBase := strings.TrimSuffix(config.PCHFile, filepath.Ext(config.PCHFile))
	for _, sf := range proj.SourceFiles {
		base := strings.TrimSuffix(filepath.Base(sf.AbsolutePath), filepath.Ext(sf.AbsolutePath))
		if strings.EqualFold(base, pchBase) {
			return filepath.Clean(sf.AbsolutePath)
		}
	}

	return ""
}
