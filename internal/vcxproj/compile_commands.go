package vcxproj

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"clang-graphdb/internal/msvccompat"
)

// GenerateCompileCommands creates compile_commands.json entries for all
// non-CLR source files in the given projects.
func GenerateCompileCommands(projects []*ParsedProject, targetConfig string) []CompileCommand {
	systemIncludes := GetSystemIncludePaths()

	var commands []CompileCommand
	for _, proj := range projects {
		config, ok := proj.Configs[targetConfig]
		if !ok {
			continue
		}

		// Skip fully-CLR projects
		if strings.ToLower(config.CLRSupport) == "true" {
			fmt.Fprintf(os.Stderr, "[SKIP-CLR] Project %s: CLRSupport=true, skipping entire project\n", proj.Name)
			continue
		}

		baseArgs := buildBaseArgs(proj, config, systemIncludes)

		for _, sf := range proj.SourceFiles {
			if sf.IsCLR {
				fmt.Fprintf(os.Stderr, "[SKIP-CLR] %s: CompileAsManaged=true\n", sf.RelativePath)
				continue
			}

			cmd := CompileCommand{
				Directory: proj.AbsoluteDir,
				File:      filepath.Clean(sf.AbsolutePath),
				Arguments: make([]string, len(baseArgs)+2),
			}
			copy(cmd.Arguments, baseArgs)
			cmd.Arguments[len(baseArgs)] = "/c"
			cmd.Arguments[len(baseArgs)+1] = filepath.Clean(sf.AbsolutePath)

			commands = append(commands, cmd)
		}
	}

	return commands
}

func buildBaseArgs(proj *ParsedProject, config *ProjectConfig, systemIncludes []string) []string {
	args := []string{"clang-cl"}

	// Language standard
	if config.LanguageStandard != "" {
		std := config.LanguageStandard
		if strings.HasPrefix(std, "stdcpp") {
			std = "/std:c++" + strings.TrimPrefix(std, "stdcpp")
		} else if strings.HasPrefix(std, "stdc") {
			std = "/std:c" + strings.TrimPrefix(std, "stdc")
		}
		args = append(args, std)
	}

	// Preprocessor definitions
	for _, d := range config.PreprocessorDefines {
		args = append(args, "/D"+d)
	}

	// Add implicit defines based on project type
	args = appendImplicitDefines(args, config)

	// Project include paths
	for _, p := range config.IncludePaths {
		args = append(args, "/I"+p)
	}

	// System include paths (MSVC, MFC, Windows SDK)
	for _, p := range systemIncludes {
		args = append(args, "/I"+p)
	}

	// Warning level
	switch config.WarningLevel {
	case "Level1":
		args = append(args, "/W1")
	case "Level2":
		args = append(args, "/W2")
	case "Level3":
		args = append(args, "/W3")
	case "Level4":
		args = append(args, "/W4")
	}

	// Runtime library
	switch strings.ToLower(config.RuntimeLibrary) {
	case "multithreadeddll":
		args = append(args, "/MD")
	case "multithreadeddebugdll":
		args = append(args, "/MDd")
	case "multithreaded":
		args = append(args, "/MT")
	case "multithreadeddebug":
		args = append(args, "/MTd")
	default:
		if strings.ToLower(config.UseOfMfc) == "dynamic" {
			args = append(args, "/MD")
		}
	}

	// Exception handling
	args = append(args, "/EHsc")

	// Struct alignment
	if config.StructMemberAlign == "8Bytes" {
		args = append(args, "/Zp8")
	}

	// Translate additional MSVC flags
	if config.AdditionalOptions != "" {
		extraFlags := msvccompat.SplitOptions(config.AdditionalOptions)
		translated := msvccompat.TranslateFlags(extraFlags)
		args = append(args, translated...)
	}

	// Clang MSVC compatibility flags
	args = append(args,
		"-fms-compatibility",
		"-fms-extensions",
		"-fdelayed-template-parsing",
		"-Wno-everything",
		"-ferror-limit=50",
	)

	return args
}

func appendImplicitDefines(args []string, config *ProjectConfig) []string {
	mfc := strings.ToLower(config.UseOfMfc)
	if mfc == "dynamic" {
		args = append(args, "/D_AFXDLL")
	}

	ct := strings.ToLower(config.ConfigurationType)
	if ct == "dynamiclibrary" {
		args = append(args, "/D_WINDLL", "/D_USRDLL")
	} else if ct == "staticlibrary" {
		args = append(args, "/D_LIB")
	}

	return args
}

// BuildProjectArgs returns the base clang-cl arguments for a project config,
// excluding per-file flags (/c and the source file path).
func BuildProjectArgs(proj *ParsedProject, config *ProjectConfig) []string {
	systemIncludes := GetSystemIncludePaths()
	return buildBaseArgs(proj, config, systemIncludes)
}

// WriteCompileCommands writes compile_commands.json to the given path.
func WriteCompileCommands(commands []CompileCommand, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("cannot create compile_commands.json: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(commands); err != nil {
		return fmt.Errorf("cannot write compile_commands.json: %w", err)
	}

	return nil
}
