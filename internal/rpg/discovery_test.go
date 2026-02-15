package rpg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryDomainDiscoverer_DiscoverDomains(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup fake structure
	os.MkdirAll(filepath.Join(tmpDir, "internal", "auth"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "internal", "billing"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "cmd", "app"), 0755)

	discoverer := &DirectoryDomainDiscoverer{
		BaseDirs: []string{"internal", "cmd"},
	}

	domains, err := discoverer.DiscoverDomains(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverDomains failed: %v", err)
	}

	expected := map[string]string{
		filepath.Join("internal", "auth"):    filepath.Join("internal", "auth"),
		filepath.Join("internal", "billing"): filepath.Join("internal", "billing"),
		filepath.Join("cmd", "app"):          filepath.Join("cmd", "app"),
	}

	if len(domains) != 3 {
		t.Errorf("Expected 3 domains, got %d", len(domains))
	}

	for k, v := range expected {
		if domains[k] != v {
			t.Errorf("Domain %s: expected path %s, got %s", k, v, domains[k])
		}
	}
}

func TestDirectoryDomainDiscoverer_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	discoverer := &DirectoryDomainDiscoverer{
		BaseDirs: []string{"nonexistent"},
	}

	domains, err := discoverer.DiscoverDomains(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverDomains failed: %v", err)
	}

	if len(domains) != 1 || domains["root"] != "" {
		t.Errorf("Expected fallback to root, got %v", domains)
	}
}

func TestDirectoryDomainDiscoverer_SmartDiscovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup fake structure
	os.MkdirAll(filepath.Join(tmpDir, "pkg1"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "pkg2"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("data"), 0644)

	// BaseDirs is empty -> Smart Discovery (uses ".")
	discoverer := &DirectoryDomainDiscoverer{
		BaseDirs: []string{},
	}

	domains, err := discoverer.DiscoverDomains(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverDomains failed: %v", err)
	}

	// Should discover pkg1 and pkg2, skip .hidden and file.txt
	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d: %v", len(domains), domains)
	}

	if _, ok := domains["pkg1"]; !ok {
		t.Errorf("Expected domain pkg1, got %v", domains)
	}
	if _, ok := domains["pkg2"]; !ok {
		t.Errorf("Expected domain pkg2, got %v", domains)
	}
}
