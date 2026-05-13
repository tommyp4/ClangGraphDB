package vcxproj

// SolutionProject represents a project entry from a .sln file.
type SolutionProject struct {
	Name         string
	RelativePath string
	GUID         string
	TypeGUID     string
	Dependencies []string // GUIDs of projects this depends on
}

// Solution represents a parsed .sln file.
type Solution struct {
	Dir      string // absolute directory containing the .sln
	Projects []SolutionProject
}

// ProjectConfig holds compiler settings for a build configuration.
type ProjectConfig struct {
	Configuration       string
	Platform            string
	ConfigurationType   string // Application, DynamicLibrary, StaticLibrary
	PlatformToolset     string // v145, v140, etc.
	UseOfMfc            string // Dynamic, Static, false
	LanguageStandard    string // stdcpp17, etc.
	IncludePaths        []string
	PreprocessorDefines []string
	AdditionalOptions   string
	PrecompiledHeader   string // Use, Create, NotUsing
	PCHFile             string // stdafx.h
	RuntimeLibrary      string
	RuntimeTypeInfo     string
	StructMemberAlign   string
	CLRSupport          string // true, false, empty
	WarningLevel        string
}

// SourceFile represents a single compilable source file.
type SourceFile struct {
	RelativePath      string
	AbsolutePath      string
	IsCLR             bool
	PCHOverride       string // per-file PCH override
	ExtraDefines      []string
	ExtraIncludePaths []string
}

// ProjectReference is a build dependency on another project.
type ProjectReference struct {
	Include string // relative path to the .vcxproj
	GUID    string
	Name    string
}

// ParsedProject is a fully resolved representation of a .vcxproj.
type ParsedProject struct {
	Name              string
	GUID              string
	AbsoluteDir       string
	VcxprojPath       string
	Configs           map[string]*ProjectConfig // key: "Debug|Win32"
	SourceFiles       []SourceFile
	ProjectReferences []ProjectReference
	ImportedProps     []string // absolute paths of imported .props files
}

// CompileCommand is a single entry in compile_commands.json.
type CompileCommand struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
}
