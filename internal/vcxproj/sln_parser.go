package vcxproj

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	projectLineRe = regexp.MustCompile(
		`^Project\("\{([^}]+)\}"\)\s*=\s*"([^"]+)",\s*"([^"]+)",\s*"\{([^}]+)\}"`)
	depLineRe = regexp.MustCompile(`^\s*\{([^}]+)\}\s*=\s*\{[^}]+\}`)
)

const cppProjectTypeGUID = "8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942"

func ParseSolution(slnPath string) (*Solution, error) {
	absPath, err := filepath.Abs(slnPath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve sln path: %w", err)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open sln: %w", err)
	}
	defer f.Close()

	sol := &Solution{
		Dir: filepath.Dir(absPath),
	}

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	var currentProject *SolutionProject
	inDeps := false

	for scanner.Scan() {
		line := scanner.Text()

		if m := projectLineRe.FindStringSubmatch(line); m != nil {
			proj := SolutionProject{
				TypeGUID:     strings.ToUpper(m[1]),
				Name:         m[2],
				RelativePath: m[3],
				GUID:         strings.ToUpper(m[4]),
			}
			sol.Projects = append(sol.Projects, proj)
			currentProject = &sol.Projects[len(sol.Projects)-1]
			inDeps = false
			continue
		}

		if currentProject != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed == "ProjectSection(ProjectDependencies) = postProject" {
				inDeps = true
				continue
			}
			if trimmed == "EndProjectSection" {
				inDeps = false
				continue
			}
			if trimmed == "EndProject" {
				currentProject = nil
				inDeps = false
				continue
			}
			if inDeps {
				if m := depLineRe.FindStringSubmatch(line); m != nil {
					currentProject.Dependencies = append(currentProject.Dependencies, strings.ToUpper(m[1]))
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sln: %w", err)
	}

	return sol, nil
}

// CppProjects returns only C++ projects from the solution.
func (s *Solution) CppProjects() []SolutionProject {
	var result []SolutionProject
	for _, p := range s.Projects {
		if p.TypeGUID == cppProjectTypeGUID {
			result = append(result, p)
		}
	}
	return result
}

// ResolveProjectPath returns the absolute path for a project's vcxproj file.
func (s *Solution) ResolveProjectPath(proj SolutionProject) string {
	rel := filepath.FromSlash(proj.RelativePath)
	return filepath.Join(s.Dir, rel)
}

// BuildGUIDMap returns a map from GUID to project name.
func (s *Solution) BuildGUIDMap() map[string]string {
	m := make(map[string]string, len(s.Projects))
	for _, p := range s.Projects {
		m[p.GUID] = p.Name
	}
	return m
}
