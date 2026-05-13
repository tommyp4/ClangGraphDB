package vcxproj

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var varPattern = regexp.MustCompile(`\$\(([^)]+)\)`)

// VarResolver resolves MSBuild $(Variable) references.
type VarResolver struct {
	vars map[string]string
}

func NewVarResolver() *VarResolver {
	return &VarResolver{vars: make(map[string]string)}
}

func (r *VarResolver) Set(name, value string) {
	r.vars[name] = value
}

func (r *VarResolver) Get(name string) string {
	return r.vars[name]
}

func (r *VarResolver) Resolve(input string) string {
	return varPattern.ReplaceAllStringFunc(input, func(match string) string {
		name := match[2 : len(match)-1]
		if val, ok := r.vars[name]; ok {
			return val
		}
		if val := os.Getenv(name); val != "" {
			return val
		}
		return match
	})
}

// SetupDefaults configures standard MSBuild variables.
func (r *VarResolver) SetupDefaults(vcxprojDir, slnDir, config, platform string) {
	r.Set("Configuration", config)
	r.Set("Platform", platform)

	if platform == "Win32" {
		r.Set("PlatformShortName", "x86")
	} else {
		r.Set("PlatformShortName", strings.ToLower(platform))
	}

	absDir, _ := filepath.Abs(vcxprojDir)
	r.Set("MSBuildProjectDirectory", absDir)
	r.Set("ProjectDir", absDir+string(filepath.Separator))

	if slnDir != "" {
		absSln, _ := filepath.Abs(slnDir)
		r.Set("SolutionDir", absSln+string(filepath.Separator))
	}

	vcToolsDir := detectVCToolsDir()
	if vcToolsDir != "" {
		r.Set("VCToolsInstallDir", vcToolsDir)
	}

	wsdkDir, wsdkVer := detectWindowsSDK()
	if wsdkDir != "" {
		r.Set("WindowsSdkDir", wsdkDir)
		r.Set("WindowsSDKVersion", wsdkVer)
	}
}

// ResolvePathList splits a semicolon-separated path list, resolves variables,
// trims whitespace, and returns absolute paths relative to baseDir.
func (r *VarResolver) ResolvePathList(raw string, baseDir string) []string {
	resolved := r.Resolve(raw)
	parts := strings.Split(resolved, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "%(AdditionalIncludeDirectories)" || p == "$(AdditionalIncludeDirectories)" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		p = filepath.Clean(p)
		result = append(result, p)
	}
	return result
}

// ResolveDefineList splits a semicolon-separated define list and resolves variables.
func (r *VarResolver) ResolveDefineList(raw string) []string {
	resolved := r.Resolve(raw)
	parts := strings.Split(resolved, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "%(PreprocessorDefinitions)" {
			continue
		}
		result = append(result, p)
	}
	return result
}

func detectVCToolsDir() string {
	if dir := os.Getenv("VCToolsInstallDir"); dir != "" {
		return dir
	}
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"),
		"Microsoft Visual Studio", "Installer", "vswhere.exe")
	if _, err := os.Stat(vswhere); err != nil {
		return ""
	}
	out, err := exec.Command(vswhere,
		"-latest", "-products", "*",
		"-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
		"-property", "installationPath").Output()
	if err != nil {
		return ""
	}
	vsPath := strings.TrimSpace(string(out))
	if vsPath == "" {
		return ""
	}
	// Find the latest MSVC tools version
	toolsBase := filepath.Join(vsPath, "VC", "Tools", "MSVC")
	entries, err := os.ReadDir(toolsBase)
	if err != nil || len(entries) == 0 {
		return ""
	}
	latest := entries[len(entries)-1].Name()
	return filepath.Join(toolsBase, latest) + string(filepath.Separator)
}

func detectWindowsSDK() (dir string, version string) {
	sdkDir := os.Getenv("WindowsSdkDir")
	sdkVer := os.Getenv("WindowsSDKVersion")
	if sdkDir != "" && sdkVer != "" {
		return sdkDir, sdkVer
	}
	// Try common location
	base := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Windows Kits", "10")
	if _, err := os.Stat(base); err != nil {
		return "", ""
	}
	incDir := filepath.Join(base, "Include")
	entries, err := os.ReadDir(incDir)
	if err != nil || len(entries) == 0 {
		return base + string(filepath.Separator), ""
	}
	// Pick the latest version
	latest := ""
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "10.") {
			latest = e.Name()
		}
	}
	return base + string(filepath.Separator), latest + string(filepath.Separator)
}

// GetMFCIncludePath returns the MFC include directory from the VC tools installation.
func GetMFCIncludePath() string {
	vcTools := detectVCToolsDir()
	if vcTools == "" {
		return ""
	}
	mfcInc := filepath.Join(vcTools, "atlmfc", "include")
	if _, err := os.Stat(mfcInc); err == nil {
		return mfcInc
	}
	return ""
}

// GetVCIncludePath returns the VC tools include directory.
func GetVCIncludePath() string {
	vcTools := detectVCToolsDir()
	if vcTools == "" {
		return ""
	}
	return filepath.Join(vcTools, "include")
}

// GetSystemIncludePaths returns all system include paths needed for Clang.
func GetSystemIncludePaths() []string {
	var paths []string

	if p := GetVCIncludePath(); p != "" {
		paths = append(paths, p)
	}
	if p := GetMFCIncludePath(); p != "" {
		paths = append(paths, p)
	}

	sdkDir, sdkVer := detectWindowsSDK()
	if sdkDir != "" && sdkVer != "" {
		base := filepath.Join(sdkDir, "Include", strings.TrimSuffix(sdkVer, string(filepath.Separator)))
		for _, sub := range []string{"ucrt", "shared", "um", "winrt"} {
			p := filepath.Join(base, sub)
			if _, err := os.Stat(p); err == nil {
				paths = append(paths, p)
			}
		}
	}

	return paths
}
