package vcxproj

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// XML structures for parsing .vcxproj files.
// We use a tolerant approach: unknown elements are ignored.

type xmlProject struct {
	XMLName            xml.Name             `xml:"Project"`
	PropertyGroups     []xmlPropertyGroup   `xml:"PropertyGroup"`
	ItemDefinGroups    []xmlItemDefGroup    `xml:"ItemDefinitionGroup"`
	ItemGroups         []xmlItemGroup       `xml:"ItemGroup"`
	Imports            []xmlImport          `xml:"Import"`
	ImportGroups       []xmlImportGroup     `xml:"ImportGroup"`
}

type xmlPropertyGroup struct {
	Condition        string `xml:"Condition,attr"`
	Label            string `xml:"Label,attr"`
	ProjectGuid      string `xml:"ProjectGuid"`
	RootNamespace    string `xml:"RootNamespace"`
	Keyword          string `xml:"Keyword"`
	ConfigurationType string `xml:"ConfigurationType"`
	UseOfMfc         string `xml:"UseOfMfc"`
	PlatformToolset  string `xml:"PlatformToolset"`
	CLRSupport       string `xml:"CLRSupport"`
	ManagedAssembly  string `xml:"ManagedAssembly"`
	WinTargetPlatVer string `xml:"WindowsTargetPlatformVersion"`
	// Generic property support for .props files
	RootProjectDir   string `xml:"RootProjectDir"`
	OdaIncludePaths  string `xml:"OdaIncludePaths"`
	OdaPreprocessor  string `xml:"OdaPreprocessorMacro"`
}

type xmlItemDefGroup struct {
	Condition string        `xml:"Condition,attr"`
	ClCompile xmlClCompile  `xml:"ClCompile"`
}

type xmlClCompile struct {
	AdditionalIncludeDirs string `xml:"AdditionalIncludeDirectories"`
	PreprocessorDefs      string `xml:"PreprocessorDefinitions"`
	AdditionalOptions     string `xml:"AdditionalOptions"`
	PrecompiledHeader     string `xml:"PrecompiledHeader"`
	PrecompiledHeaderFile string `xml:"PrecompiledHeaderFile"`
	RuntimeLibrary        string `xml:"RuntimeLibrary"`
	RuntimeTypeInfo       string `xml:"RuntimeTypeInfo"`
	StructMemberAlignment string `xml:"StructMemberAlignment"`
	LanguageStandard      string `xml:"LanguageStandard"`
	WarningLevel          string `xml:"WarningLevel"`
	CompileAs             string `xml:"CompileAs"`
}

type xmlItemGroup struct {
	ClCompileItems    []xmlClCompileItem `xml:"ClCompile"`
	ProjectReferences []xmlProjRef       `xml:"ProjectReference"`
}

type xmlClCompileItem struct {
	Include           string               `xml:"Include,attr"`
	CompileAsManaged  []xmlConditionalVal  `xml:"CompileAsManaged"`
	PrecompiledHeader []xmlConditionalVal  `xml:"PrecompiledHeader"`
}

type xmlConditionalVal struct {
	Condition string `xml:"Condition,attr"`
	Value     string `xml:",chardata"`
}

type xmlProjRef struct {
	Include string `xml:"Include,attr"`
	Project string `xml:"Project"`
	Name    string `xml:"Name"`
}

type xmlImport struct {
	Project   string `xml:"Project,attr"`
	Condition string `xml:"Condition,attr"`
}

type xmlImportGroup struct {
	Condition string      `xml:"Condition,attr"`
	Label     string      `xml:"Label,attr"`
	Imports   []xmlImport `xml:"Import"`
}

// ParseVcxproj parses a .vcxproj file for the given configuration.
func ParseVcxproj(vcxprojPath string, targetConfig string, resolver *VarResolver) (*ParsedProject, error) {
	absPath, err := filepath.Abs(vcxprojPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve vcxproj path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read vcxproj: %w", err)
	}

	var proj xmlProject
	if err := xml.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("cannot parse vcxproj XML: %w", err)
	}

	projectDir := filepath.Dir(absPath)
	resolver.Set("MSBuildProjectDirectory", projectDir)
	resolver.Set("ProjectDir", projectDir+string(filepath.Separator))

	parsed := &ParsedProject{
		AbsoluteDir: projectDir,
		VcxprojPath: absPath,
		Configs:     make(map[string]*ProjectConfig),
	}

	// Extract globals
	for _, pg := range proj.PropertyGroups {
		if pg.Label == "Globals" {
			parsed.GUID = normalizeGUID(pg.ProjectGuid)
			parsed.Name = pg.RootNamespace
			if parsed.Name == "" {
				parsed.Name = filepath.Base(projectDir)
			}
		}
	}

	// Process imports to find .props files (especially RootPropertySheet.props)
	acc := &propsAccumulator{}
	for _, imp := range proj.Imports {
		resolveAndApplyProps(imp.Project, projectDir, resolver, parsed, acc)
	}
	for _, ig := range proj.ImportGroups {
		if !matchesCondition(ig.Condition, targetConfig) && ig.Condition != "" {
			continue
		}
		for _, imp := range ig.Imports {
			if imp.Condition != "" && !conditionFileExists(imp.Condition, resolver) {
				continue
			}
			resolveAndApplyProps(imp.Project, projectDir, resolver, parsed, acc)
		}
	}

	// Build config from PropertyGroups
	config := &ProjectConfig{}
	parts := strings.SplitN(targetConfig, "|", 2)
	if len(parts) == 2 {
		config.Configuration = parts[0]
		config.Platform = parts[1]
	}

	for _, pg := range proj.PropertyGroups {
		if pg.Label == "Configuration" && matchesCondition(pg.Condition, targetConfig) {
			config.ConfigurationType = pg.ConfigurationType
			config.UseOfMfc = pg.UseOfMfc
			config.PlatformToolset = pg.PlatformToolset
			config.CLRSupport = pg.CLRSupport
		}
		if pg.Label == "Globals" && pg.ManagedAssembly != "" {
			// Don't treat ManagedAssembly=true as CLR for the whole project;
			// it enables mixed mode but per-file CompileAsManaged decides.
		}
		if pg.CLRSupport != "" && matchesCondition(pg.Condition, targetConfig) {
			config.CLRSupport = pg.CLRSupport
		}
	}

	// Start with inherited settings from .props files
	config.IncludePaths = append(config.IncludePaths, acc.includePaths...)
	config.PreprocessorDefines = append(config.PreprocessorDefines, acc.defines...)
	if acc.options != "" {
		config.AdditionalOptions = acc.options
	}

	// Extract ClCompile settings from ItemDefinitionGroup (project-level overrides)
	for _, idg := range proj.ItemDefinGroups {
		if !matchesCondition(idg.Condition, targetConfig) && idg.Condition != "" {
			continue
		}
		cc := idg.ClCompile
		if cc.AdditionalIncludeDirs != "" {
			projectPaths := resolver.ResolvePathList(cc.AdditionalIncludeDirs, projectDir)
			config.IncludePaths = append(config.IncludePaths, projectPaths...)
		}
		if cc.PreprocessorDefs != "" {
			projectDefs := resolver.ResolveDefineList(cc.PreprocessorDefs)
			config.PreprocessorDefines = append(config.PreprocessorDefines, projectDefs...)
		}
		if cc.AdditionalOptions != "" {
			resolved := resolver.Resolve(cc.AdditionalOptions)
			if config.AdditionalOptions != "" {
				config.AdditionalOptions += " "
			}
			config.AdditionalOptions += resolved
		}
		if cc.PrecompiledHeader != "" {
			config.PrecompiledHeader = cc.PrecompiledHeader
		}
		if cc.PrecompiledHeaderFile != "" {
			config.PCHFile = cc.PrecompiledHeaderFile
		}
		if cc.RuntimeLibrary != "" {
			config.RuntimeLibrary = cc.RuntimeLibrary
		}
		if cc.RuntimeTypeInfo != "" {
			config.RuntimeTypeInfo = cc.RuntimeTypeInfo
		}
		if cc.StructMemberAlignment != "" {
			config.StructMemberAlign = cc.StructMemberAlignment
		}
		if cc.LanguageStandard != "" {
			config.LanguageStandard = cc.LanguageStandard
		}
		if cc.WarningLevel != "" {
			config.WarningLevel = cc.WarningLevel
		}
	}

	parsed.Configs[targetConfig] = config

	// Extract source files
	for _, ig := range proj.ItemGroups {
		for _, ci := range ig.ClCompileItems {
			if ci.Include == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(ci.Include))
			if ext != ".cpp" && ext != ".c" && ext != ".cc" && ext != ".cxx" {
				continue
			}

			sf := SourceFile{
				RelativePath: ci.Include,
				AbsolutePath: filepath.Join(projectDir, filepath.FromSlash(ci.Include)),
			}

			// Check per-file CLR status
			sf.IsCLR = isFileCLR(ci.CompileAsManaged, targetConfig, config)

			// Check per-file PCH override
			for _, pch := range ci.PrecompiledHeader {
				if matchesCondition(pch.Condition, targetConfig) || pch.Condition == "" {
					val := strings.TrimSpace(pch.Value)
					if val != "" {
						sf.PCHOverride = val
					}
				}
			}

			parsed.SourceFiles = append(parsed.SourceFiles, sf)
		}

		// Project references
		for _, pr := range ig.ProjectReferences {
			parsed.ProjectReferences = append(parsed.ProjectReferences, ProjectReference{
				Include: pr.Include,
				GUID:    normalizeGUID(pr.Project),
				Name:    pr.Name,
			})
		}
	}

	return parsed, nil
}

// propsIncludes and propsDefines accumulate inherited settings from .props files.
type propsAccumulator struct {
	includePaths []string
	defines      []string
	options      string
}

func resolveAndApplyProps(importPath string, projectDir string, resolver *VarResolver, parsed *ParsedProject, acc *propsAccumulator) {
	if importPath == "" {
		return
	}
	// Skip MSBuild system imports
	if strings.Contains(importPath, "$(VCTargetsPath)") ||
		strings.Contains(importPath, "$(UserRootDir)") {
		return
	}

	resolved := resolver.Resolve(importPath)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(projectDir, resolved)
	}
	resolved = filepath.Clean(resolved)

	if _, err := os.Stat(resolved); err != nil {
		return
	}

	parsed.ImportedProps = append(parsed.ImportedProps, resolved)
	applyPropsFile(resolved, resolver, acc)
}

func applyPropsFile(propsPath string, resolver *VarResolver, acc *propsAccumulator) {
	data, err := os.ReadFile(propsPath)
	if err != nil {
		return
	}

	var proj xmlProject
	if err := xml.Unmarshal(data, &proj); err != nil {
		return
	}

	propsDir := filepath.Dir(propsPath)

	// Extract property values from PropertyGroups.
	// Process in order, respecting conditions. The RootPropertySheet.props pattern is:
	//   PG1: RootProjectDir = $(MSBuildProjectDirectory)\..\
	//   PG2 Condition="!Exists(...)": RootProjectDir = $(RootProjectDir)\..\
	// We must evaluate the condition before applying PG2.
	for _, pg := range proj.PropertyGroups {
		if pg.Condition != "" && !evaluatePropsCondition(pg.Condition, resolver) {
			continue
		}
		if pg.RootProjectDir != "" {
			resolved := resolver.Resolve(pg.RootProjectDir)
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(propsDir, resolved)
			}
			resolved = filepath.Clean(resolved)
			resolver.Set("RootProjectDir", resolved+string(filepath.Separator))
		}
		if pg.OdaIncludePaths != "" {
			resolver.Set("OdaIncludePaths", pg.OdaIncludePaths)
		}
		if pg.OdaPreprocessor != "" {
			resolver.Set("OdaPreprocessorMacro", pg.OdaPreprocessor)
		}
	}

	// Extract ClCompile settings from ItemDefinitionGroup in props.
	// Resolve AFTER all properties have been set above so $(RootProjectDir) etc. are available.
	for _, idg := range proj.ItemDefinGroups {
		cc := idg.ClCompile
		if cc.AdditionalIncludeDirs != "" {
			// The raw string may contain $(OdaIncludePaths) which expands to
			// paths containing $(RootProjectDir), $(PlatformShortName), $(Configuration).
			// Two-pass resolve handles this.
			expanded := resolver.Resolve(cc.AdditionalIncludeDirs)
			expanded = resolver.Resolve(expanded)
			paths := resolver.ResolvePathList(expanded, propsDir)
			acc.includePaths = append(acc.includePaths, paths...)
		}
		if cc.PreprocessorDefs != "" {
			expanded := resolver.Resolve(cc.PreprocessorDefs)
			expanded = resolver.Resolve(expanded)
			defs := resolver.ResolveDefineList(expanded)
			acc.defines = append(acc.defines, defs...)
		}
		if cc.AdditionalOptions != "" {
			resolved := resolver.Resolve(cc.AdditionalOptions)
			if acc.options != "" {
				acc.options += " "
			}
			acc.options += resolved
		}
	}
}

// evaluatePropsCondition handles MSBuild conditions in .props files.
// Supports: !Exists('path'), 'A'=='B', and basic config matching.
func evaluatePropsCondition(condition string, resolver *VarResolver) bool {
	resolved := resolver.Resolve(condition)

	// Handle !Exists('path')
	if strings.Contains(resolved, "!Exists(") {
		start := strings.Index(resolved, "'")
		end := strings.LastIndex(resolved, "'")
		if start >= 0 && end > start {
			path := resolved[start+1 : end]
			_, err := os.Stat(path)
			return err != nil // !Exists = true when file doesn't exist
		}
	}

	// Handle Exists('path')
	if strings.Contains(resolved, "Exists(") && !strings.Contains(resolved, "!Exists(") {
		start := strings.Index(resolved, "'")
		end := strings.LastIndex(resolved, "'")
		if start >= 0 && end > start {
			path := resolved[start+1 : end]
			_, err := os.Stat(path)
			return err == nil
		}
	}

	// Handle 'A'=='B' equality
	if strings.Contains(resolved, "==") {
		parts := strings.SplitN(resolved, "==", 2)
		if len(parts) == 2 {
			a := strings.Trim(strings.TrimSpace(parts[0]), "'")
			b := strings.Trim(strings.TrimSpace(parts[1]), "'")
			return strings.EqualFold(a, b)
		}
	}

	return true
}

func matchesCondition(condition, targetConfig string) bool {
	if condition == "" {
		return true
	}
	return strings.Contains(condition, targetConfig)
}

func conditionFileExists(condition string, resolver *VarResolver) bool {
	// Handle Condition="exists('$(SomePath)')"
	if !strings.Contains(condition, "exists(") {
		return true
	}
	start := strings.Index(condition, "'")
	end := strings.LastIndex(condition, "'")
	if start < 0 || end <= start {
		return true
	}
	path := resolver.Resolve(condition[start+1 : end])
	_, err := os.Stat(path)
	return err == nil
}

func normalizeGUID(guid string) string {
	guid = strings.TrimPrefix(guid, "{")
	guid = strings.TrimSuffix(guid, "}")
	return strings.ToUpper(strings.TrimSpace(guid))
}

func isFileCLR(managed []xmlConditionalVal, targetConfig string, projectConfig *ProjectConfig) bool {
	for _, m := range managed {
		if matchesCondition(m.Condition, targetConfig) || m.Condition == "" {
			val := strings.TrimSpace(strings.ToLower(m.Value))
			if val == "true" {
				return true
			}
			if val == "false" {
				return false
			}
		}
	}
	// If no per-file override, check if project has /clr in options
	if strings.Contains(projectConfig.AdditionalOptions, "/clr") {
		return true
	}
	return strings.ToLower(projectConfig.CLRSupport) == "true"
}
