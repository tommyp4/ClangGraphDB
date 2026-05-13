package vcxproj

import (
	"fmt"
	"os"
	"testing"
)

func TestParseRealSolution(t *testing.T) {
	slnPath := `C:\Repos\VIEW\ais\FullBuild.sln`
	if _, err := os.Stat(slnPath); err != nil {
		t.Skip("VIEW repo not available")
	}

	sol, err := ParseSolution(slnPath)
	if err != nil {
		t.Fatal(err)
	}

	cpp := sol.CppProjects()
	t.Logf("Total projects: %d, C++ projects: %d", len(sol.Projects), len(cpp))

	if len(cpp) < 100 {
		t.Errorf("expected 100+ C++ projects, got %d", len(cpp))
	}

	// Verify AIS project exists and has dependencies
	var ais *SolutionProject
	for i := range cpp {
		if cpp[i].Name == "AIS" {
			ais = &cpp[i]
			break
		}
	}
	if ais == nil {
		t.Fatal("AIS project not found")
	}
	t.Logf("AIS dependencies: %d", len(ais.Dependencies))
	if len(ais.Dependencies) < 50 {
		t.Errorf("expected 50+ AIS dependencies, got %d", len(ais.Dependencies))
	}

	// Verify GUID map
	guidMap := sol.BuildGUIDMap()
	t.Logf("GUID map entries: %d", len(guidMap))
}

func TestParseRealVcxproj(t *testing.T) {
	slnPath := `C:\Repos\VIEW\ais\FullBuild.sln`
	if _, err := os.Stat(slnPath); err != nil {
		t.Skip("VIEW repo not available")
	}

	sol, err := ParseSolution(slnPath)
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewVarResolver()
	resolver.SetupDefaults("", sol.Dir, "Debug", "Win32")

	// Parse GenSubs (a representative lib project)
	gensubsPath := `C:\Repos\VIEW\libs\gensubs\GenSubs.vcxproj`
	if _, err := os.Stat(gensubsPath); err != nil {
		t.Skip("GenSubs.vcxproj not available")
	}

	proj, err := ParseVcxproj(gensubsPath, "Debug|Win32", resolver)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Project: %s (GUID: %s)", proj.Name, proj.GUID)
	t.Logf("Source files: %d", len(proj.SourceFiles))

	config := proj.Configs["Debug|Win32"]
	if config == nil {
		t.Fatal("Debug|Win32 config not found")
	}
	t.Logf("Config type: %s", config.ConfigurationType)
	t.Logf("Toolset: %s", config.PlatformToolset)
	t.Logf("MFC: %s", config.UseOfMfc)
	t.Logf("Include paths: %d", len(config.IncludePaths))
	for _, p := range config.IncludePaths {
		t.Logf("  %s", p)
	}
	t.Logf("Defines: %v", config.PreprocessorDefines)
	t.Logf("Language: %s", config.LanguageStandard)

	if config.ConfigurationType != "DynamicLibrary" {
		t.Errorf("expected DynamicLibrary, got %s", config.ConfigurationType)
	}
	if config.UseOfMfc != "Dynamic" {
		t.Errorf("expected Dynamic MFC, got %s", config.UseOfMfc)
	}
	if len(proj.SourceFiles) < 10 {
		t.Errorf("expected 10+ source files, got %d", len(proj.SourceFiles))
	}

	// Count CLR files
	clrCount := 0
	for _, sf := range proj.SourceFiles {
		if sf.IsCLR {
			clrCount++
			t.Logf("  CLR file: %s", sf.RelativePath)
		}
	}
	t.Logf("CLR files: %d / %d", clrCount, len(proj.SourceFiles))
}

func TestParseAISProject(t *testing.T) {
	aisPath := `C:\Repos\VIEW\ais\AIS.vcxproj`
	if _, err := os.Stat(aisPath); err != nil {
		t.Skip("AIS.vcxproj not available")
	}

	resolver := NewVarResolver()
	resolver.SetupDefaults(`C:\Repos\VIEW\ais`, `C:\Repos\VIEW\ais`, "Debug", "Win32")

	proj, err := ParseVcxproj(aisPath, "Debug|Win32", resolver)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Project: %s", proj.Name)
	t.Logf("Source files: %d", len(proj.SourceFiles))

	clrCount := 0
	for _, sf := range proj.SourceFiles {
		if sf.IsCLR {
			clrCount++
			t.Logf("  CLR file: %s", sf.RelativePath)
		}
	}
	t.Logf("CLR files: %d / %d", clrCount, len(proj.SourceFiles))

	if clrCount == 0 {
		t.Log("WARNING: expected at least 1 CLR file (GetITWCPLicense.cpp)")
	}
	if clrCount > 5 {
		t.Errorf("too many CLR files (%d) — ManagedAssembly=true should NOT mark all files as CLR", clrCount)
	}
}

func TestGenerateCompileCommandsSmall(t *testing.T) {
	gensubsPath := `C:\Repos\VIEW\libs\gensubs\GenSubs.vcxproj`
	if _, err := os.Stat(gensubsPath); err != nil {
		t.Skip("GenSubs.vcxproj not available")
	}

	resolver := NewVarResolver()
	resolver.SetupDefaults(`C:\Repos\VIEW\libs\gensubs`, `C:\Repos\VIEW\ais`, "Debug", "Win32")

	proj, err := ParseVcxproj(gensubsPath, "Debug|Win32", resolver)
	if err != nil {
		t.Fatal(err)
	}

	commands := GenerateCompileCommands([]*ParsedProject{proj}, "Debug|Win32")
	t.Logf("Generated %d compile commands", len(commands))

	if len(commands) == 0 {
		t.Fatal("no compile commands generated")
	}

	// Print first command for inspection
	cmd := commands[0]
	t.Logf("First command:")
	t.Logf("  directory: %s", cmd.Directory)
	t.Logf("  file: %s", cmd.File)
	fmt.Fprintf(os.Stderr, "  args: ")
	for _, a := range cmd.Arguments {
		fmt.Fprintf(os.Stderr, "%s ", a)
	}
	fmt.Fprintln(os.Stderr)
}
