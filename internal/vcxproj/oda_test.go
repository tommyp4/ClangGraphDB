package vcxproj

import (
	"os"
	"testing"
)

func TestParseODAProject(t *testing.T) {
	// LAYIO imports RootPropertySheet.props
	layioPath := `C:\Repos\VIEW\Layout\layio\LAYIO.vcxproj`
	if _, err := os.Stat(layioPath); err != nil {
		t.Skip("LAYIO.vcxproj not available")
	}

	resolver := NewVarResolver()
	resolver.SetupDefaults(`C:\Repos\VIEW\Layout\layio`, `C:\Repos\VIEW\ais`, "Debug", "Win32")

	proj, err := ParseVcxproj(layioPath, "Debug|Win32", resolver)
	if err != nil {
		t.Fatal(err)
	}

	config := proj.Configs["Debug|Win32"]
	if config == nil {
		t.Fatal("Debug|Win32 config not found")
	}

	t.Logf("Project: %s", proj.Name)
	t.Logf("Include paths (%d):", len(config.IncludePaths))
	for _, p := range config.IncludePaths {
		t.Logf("  %s", p)
	}
	t.Logf("Defines: %v", config.PreprocessorDefines)
	t.Logf("Source files: %d", len(proj.SourceFiles))

	// LAYIO should have ODA include paths since it imports RootPropertySheet.props
	hasODA := false
	for _, p := range config.IncludePaths {
		if len(p) > 7 && p[len(p)-7:] == "include" {
			t.Logf("  -> ODA-related: %s", p)
		}
		if len(p) > 3 && (p[len(p)-3:] == "ODA" || contains(p, "ODA_CAD")) {
			hasODA = true
		}
	}
	t.Logf("Has ODA paths: %v", hasODA)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
