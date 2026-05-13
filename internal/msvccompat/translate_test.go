package msvccompat

import (
	"testing"
)

func TestTranslateFlags(t *testing.T) {
	input := []string{
		"/Zm200",
		"/wd4996",
		"/wd4503",
		"/MP",
		"/EHsc",
		"/Gm",
		"/Yu\"stdafx.h\"",
		"/Fp\"Debug/foo.pch\"",
		"/std:c++17",
		"/W4",
		"/RTC1",
		"/FS",
		"/Gy",
		"/GR",
		"%(AdditionalOptions)",
		"/Zp8",
		"/permissive-",
	}

	result := TranslateFlags(input)

	kept := map[string]bool{
		"/wd4996":     true,
		"/wd4503":     true,
		"/EHsc":       true,
		"/std:c++17":  true,
		"/W4":         true,
		"/GR":         true,
		"/Zp8":        true,
		"/permissive-": true,
	}

	dropped := map[string]bool{
		"/Zm200":              true,
		"/MP":                 true,
		"/Gm":                 true,
		"/Yu\"stdafx.h\"":     true,
		"/Fp\"Debug/foo.pch\"": true,
		"/RTC1":               true,
		"/FS":                 true,
		"/Gy":                 true,
		"%(AdditionalOptions)": true,
	}

	for _, f := range result {
		if dropped[f] {
			t.Errorf("flag %s should have been dropped", f)
		}
	}

	resultSet := make(map[string]bool)
	for _, f := range result {
		resultSet[f] = true
	}
	for f := range kept {
		if !resultSet[f] {
			t.Errorf("flag %s should have been kept", f)
		}
	}
}

func TestSplitOptions(t *testing.T) {
	raw := "/Zm200 /wd4996 /wd5031 %(AdditionalOptions)"
	result := SplitOptions(raw)

	if len(result) != 3 {
		t.Fatalf("expected 3 options, got %d: %v", len(result), result)
	}
	if result[0] != "/Zm200" {
		t.Errorf("expected /Zm200, got %s", result[0])
	}
}
