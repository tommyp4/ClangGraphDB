package vcxproj

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSolution_Minimal(t *testing.T) {
	content := `Microsoft Visual Studio Solution File, Format Version 12.00
Project("{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}") = "MyApp", "MyApp.vcxproj", "{AAAAAAAA-1111-2222-3333-444444444444}"
	ProjectSection(ProjectDependencies) = postProject
		{BBBBBBBB-1111-2222-3333-444444444444} = {BBBBBBBB-1111-2222-3333-444444444444}
	EndProjectSection
EndProject
Project("{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}") = "MyLib", "..\libs\MyLib\MyLib.vcxproj", "{BBBBBBBB-1111-2222-3333-444444444444}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "CSharpProj", "..\CSharpProj\CSharpProj.csproj", "{CCCCCCCC-1111-2222-3333-444444444444}"
EndProject
`
	tmp := t.TempDir()
	slnPath := filepath.Join(tmp, "test.sln")
	if err := os.WriteFile(slnPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sol, err := ParseSolution(slnPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(sol.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(sol.Projects))
	}

	cpp := sol.CppProjects()
	if len(cpp) != 2 {
		t.Fatalf("expected 2 C++ projects, got %d", len(cpp))
	}

	if cpp[0].Name != "MyApp" {
		t.Errorf("expected MyApp, got %s", cpp[0].Name)
	}
	if len(cpp[0].Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(cpp[0].Dependencies))
	}
	if cpp[0].Dependencies[0] != "BBBBBBBB-1111-2222-3333-444444444444" {
		t.Errorf("unexpected dep GUID: %s", cpp[0].Dependencies[0])
	}

	if cpp[1].Name != "MyLib" {
		t.Errorf("expected MyLib, got %s", cpp[1].Name)
	}

	absPath := sol.ResolveProjectPath(cpp[1])
	if !strings.Contains(absPath, "libs") {
		t.Errorf("expected resolved path to contain 'libs', got %s", absPath)
	}
}

func TestParseSolution_GUIDMap(t *testing.T) {
	content := `Microsoft Visual Studio Solution File, Format Version 12.00
Project("{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}") = "Foo", "Foo.vcxproj", "{AAAAAAAA-0000-0000-0000-000000000000}"
EndProject
Project("{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}") = "Bar", "Bar.vcxproj", "{BBBBBBBB-0000-0000-0000-000000000000}"
EndProject
`
	tmp := t.TempDir()
	slnPath := filepath.Join(tmp, "test.sln")
	os.WriteFile(slnPath, []byte(content), 0644)

	sol, err := ParseSolution(slnPath)
	if err != nil {
		t.Fatal(err)
	}

	m := sol.BuildGUIDMap()
	if m["AAAAAAAA-0000-0000-0000-000000000000"] != "Foo" {
		t.Errorf("expected Foo, got %s", m["AAAAAAAA-0000-0000-0000-000000000000"])
	}
	if m["BBBBBBBB-0000-0000-0000-000000000000"] != "Bar" {
		t.Errorf("expected Bar, got %s", m["BBBBBBBB-0000-0000-0000-000000000000"])
	}
}
