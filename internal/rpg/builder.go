package rpg

import (
	"fmt"
	"graphdb/internal/graph"
	"strings"
)

type ClusterGroup struct {
	Name        string
	Description string
	Nodes       []graph.Node
}

type Clusterer interface {
	// Clusters nodes into named groups
	Cluster(nodes []graph.Node, domain string) ([]ClusterGroup, error)
}

type Builder struct {
	Clusterer Clusterer
	// GlobalClusterer enables global discovery mode (inverted flow).
	// It clusters all functions first, then grounds them to domains.
	GlobalClusterer Clusterer

	// Callbacks for progress reporting
	OnPhaseStart func(phaseName string, total int)
	OnStepStart  func(stepName string)
	OnStepEnd    func(stepName string)
}

func (b *Builder) Build(rootPath string, functions []graph.Node) ([]Feature, []graph.Edge, error) {
	if b.GlobalClusterer == nil {
		return nil, nil, nil
	}
	return b.buildGlobal(rootPath, functions)
}

func (b *Builder) buildGlobal(rootPath string, functions []graph.Node) ([]Feature, []graph.Edge, error) {
	// 1. Global Clustering (Latent Domains)
	// We pass "root" as domain name context, though global clusterer might ignore it.
	domainGroups, err := b.GlobalClusterer.Cluster(functions, "root")
	if err != nil {
		return nil, nil, fmt.Errorf("global clustering failed: %w", err)
	}

	if b.OnPhaseStart != nil {
		b.OnPhaseStart("Processing Global Domains", len(domainGroups))
	}

	var rootFeatures []Feature
	var allEdges []graph.Edge

	for _, group := range domainGroups {
		if b.OnStepStart != nil {
			b.OnStepStart(group.Name)
		}

		// 2. Grounding (LCA)
		filePaths := make([]string, 0, len(group.Nodes))
		for _, n := range group.Nodes {
			if p, ok := n.Properties["file"].(string); ok {
				filePaths = append(filePaths, p)
			}
		}

		lca := FindLowestCommonAncestor(filePaths)
		// If LCA is empty (no common root), we might default to rootPath or just "."
		if lca == "" {
			lca = rootPath
		}

		// 3. Identification
		// Prioritize semantic name from GlobalClusterer unless it's a raw cluster ID
		var domainName string
		if strings.HasPrefix(group.Name, "cluster-") || strings.HasPrefix(group.Name, "root-cluster-") || strings.HasPrefix(group.Name, "Feature-") {
			domainName = GenerateDomainName(lca, group.Nodes)
		} else {
			domainName = group.Name
		}

		// Ensure unique and safe ID
		domainID := "domain-" + GenerateShortUUID()

		domainFeature := Feature{
			ID:          domainID,
			Name:        domainName,
			Description: group.Description,
			ScopePath:   lca,
			Children:    make([]*Feature, 0),
		}

		// In Global Mode, 'nodes' ARE the members. No filtering needed.
		domainFeature.MemberFunctions = group.Nodes

		// 4. Standard Construction (Feature Clustering)
		var err error
		allEdges, err = b.buildTwoLevel(&domainFeature, group.Nodes, domainName, lca, allEdges)
		if err != nil {
			return nil, nil, err
		}

		rootFeatures = append(rootFeatures, domainFeature)

		if b.OnStepEnd != nil {
			b.OnStepEnd(domainName)
		}
	}

	return rootFeatures, allEdges, nil
}

func (b *Builder) buildTwoLevel(domain *Feature, funcs []graph.Node, name, pathPrefix string, allEdges []graph.Edge) ([]graph.Edge, error) {
	// Use domain ID as the key for clustering if name is not unique globally, but here we use name as per interface
	clusters, err := b.Clusterer.Cluster(funcs, name)
	if err != nil {
		return nil, fmt.Errorf("feature clustering failed for domain %s: %w", name, err)
	}

	for _, group := range clusters {
		var featureName string
		var featureID string
		if strings.HasPrefix(group.Name, "cluster-") || strings.HasPrefix(group.Name, "Feature-") {
			featureID = "feature-" + GenerateShortUUID()
			featureName = ""
		} else {
			featureID = "feature-" + group.Name // Should sanitize
			featureName = group.Name
		}

		child := &Feature{
			ID:              featureID,
			Name:            featureName,
			Description:     group.Description,
			ScopePath:       pathPrefix,
			MemberFunctions: group.Nodes,
		}

		allEdges = append(allEdges, graph.Edge{
			SourceID: domain.ID,
			TargetID: child.ID,
			Type:     "PARENT_OF",
		})

		for _, fn := range group.Nodes {
			allEdges = append(allEdges, graph.Edge{
				SourceID: fn.ID,
				TargetID: child.ID,
				Type:     "IMPLEMENTS",
			})
		}

		domain.Children = append(domain.Children, child)
	}
	return allEdges, nil
}
