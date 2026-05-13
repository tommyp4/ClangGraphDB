package msvccompat

import "strings"

// droppedFlags are MSVC flags that have no Clang equivalent and should be silently removed.
var droppedFlags = map[string]bool{
	"/Zm200": true, "/Zm225": true, "/Zm125": true, "/Zm150": true,
	"/MP":  true,
	"/Gm":  true, "/Gm-": true,
	"/FS":  true,
	"/Gy":  true, "/Gy-": true,
	"/GF":  true,
	"/RTC1": true, "/RTCu": true, "/RTCs": true, "/RTCc": true,
	"/analyze":  true, "/analyze-": true,
	"/JMC":  true, "/JMC-": true,
	"/sdl":  true, "/sdl-": true,
	"/FC":   true,
	"/GL":   true,
	"/Ob0":  true, "/Ob1": true, "/Ob2": true, "/Ob3": true,
	"/Ox":   true, "/Os": true, "/Ot": true,
	"/fp:precise": true, "/fp:fast": true, "/fp:strict": true,
}

// droppedPrefixes are flag prefixes that should be dropped entirely.
var droppedPrefixes = []string{
	"/Yu", "/Yc", "/Fp", // PCH flags
	"/Fd",               // PDB file
	"/Fa", "/Fo",        // Output paths
	"/FR", "/Fr",        // Browse info
	"/doc",              // XML docs
}

// TranslateFlags converts MSVC compiler flags to Clang-compatible flags.
// Unsupported flags are silently dropped.
func TranslateFlags(msvcFlags []string) []string {
	var result []string
	for _, flag := range msvcFlags {
		flag = strings.TrimSpace(flag)
		if flag == "" || flag == "%(AdditionalOptions)" {
			continue
		}
		if shouldDrop(flag) {
			continue
		}
		result = append(result, flag)
	}
	return result
}

func shouldDrop(flag string) bool {
	if droppedFlags[flag] {
		return true
	}
	for _, prefix := range droppedPrefixes {
		if strings.HasPrefix(flag, prefix) {
			return true
		}
	}
	return false
}

// SplitOptions splits a raw AdditionalOptions string into individual flags.
func SplitOptions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Fields(raw)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "%(AdditionalOptions)" {
			result = append(result, p)
		}
	}
	return result
}
