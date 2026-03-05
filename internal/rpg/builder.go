package rpg

import (
	"graphdb/internal/graph"
	"sort"
	"strings"
)

type Clusterer interface {
	// Clusters nodes into named groups
	Cluster(nodes []graph.Node, domain string) (map[string][]graph.Node, error)
}

type Builder struct {
	Clusterer  Clusterer
	// GlobalClusterer enables global discovery mode (inverted flow).
	// It clusters all functions first, then grounds them to domains.
	GlobalClusterer Clusterer

	// CategoryClusterer enables 3-level hierarchy: Domain -> Category -> Feature.
	// If nil, falls back to 2-level: Domain -> Feature.
	CategoryClusterer Clusterer

	// Callbacks for progress reporting
	OnPhaseStart func(phaseName string, total int)
	OnStepStart  func(stepName string)
	OnStepEnd    func(stepName string)
}

func (b *Builder) Build(rootPath string, functions []graph.Node) ([]Feature, []graph.Edge, error) {
	if b.GlobalClusterer == nil {
		// Fallback or Error? 
		// Since we removed Directory Discovery, GlobalClusterer is mandatory.
		// However, for backward compatibility or testing, we might want a default?
		// But we don't have a default discovery anymore.
		return nil, nil, nil
	}
	return b.buildGlobal(rootPath, functions)
}

func (b *Builder) buildGlobal(rootPath string, functions []graph.Node) ([]Feature, []graph.Edge, error) {
	// 1. Global Clustering (Latent Domains)
	// We pass "root" as domain name context, though global clusterer might ignore it.
	domainMap, err := b.GlobalClusterer.Cluster(functions, "root")
	if err != nil {
		return nil, nil, err
	}

	// Sort domain keys for deterministic order
	domainKeys := make([]string, 0, len(domainMap))
	for k := range domainMap {
		domainKeys = append(domainKeys, k)
	}
	sort.Strings(domainKeys)

	if b.OnPhaseStart != nil {
		b.OnPhaseStart("Processing Global Domains", len(domainKeys))
	}

	var rootFeatures []Feature
	var allEdges []graph.Edge

	for _, originalKey := range domainKeys {
		nodes := domainMap[originalKey]
		if b.OnStepStart != nil {
			b.OnStepStart(originalKey)
		}

		// 2. Grounding (LCA)
		filePaths := make([]string, 0, len(nodes))
		for _, n := range nodes {
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
		if strings.HasPrefix(originalKey, "root-cluster-") || strings.HasPrefix(originalKey, "Feature-") {
			domainName = GenerateDomainName(lca, nodes)
		} else {
			domainName = originalKey
		}

		// Ensure unique and safe ID
		domainID := strings.ToLower(domainName)
		domainID = strings.ReplaceAll(domainID, " ", "-")
		// Remove unsafe chars
		domainID = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return -1
		}, domainID)

		if !strings.HasPrefix(domainID, "domain-") {
			domainID = "domain-" + domainID
		}

		domainFeature := Feature{
			ID:        domainID,
			Name:      domainName, // Use the semantic name (e.g., "Authentication System")
			ScopePath: lca,
			Children:  make([]*Feature, 0),
		}
		
		// In Global Mode, 'nodes' ARE the members. No filtering needed.
		domainFeature.MemberFunctions = nodes

		// 4. Standard Construction (Feature Clustering)
		if b.CategoryClusterer != nil {
			allEdges = b.buildThreeLevel(&domainFeature, nodes, domainName, lca, allEdges)
		} else {
			allEdges = b.buildTwoLevel(&domainFeature, nodes, domainName, lca, allEdges)
		}

		rootFeatures = append(rootFeatures, domainFeature)

		if b.OnStepEnd != nil {
			b.OnStepEnd(domainName)
		}
	}

	return rootFeatures, allEdges, nil
}

func (b *Builder) buildTwoLevel(domain *Feature, funcs []graph.Node, name, pathPrefix string, allEdges []graph.Edge) []graph.Edge {
	// Use domain ID as the key for clustering if name is not unique globally, but here we use name as per interface
	clusters, _ := b.Clusterer.Cluster(funcs, name)
	
	// Sort cluster names for deterministic order
	clusterNames := make([]string, 0, len(clusters))
	for k := range clusters {
		clusterNames = append(clusterNames, k)
	}
	sort.Strings(clusterNames)

	for _, clusterName := range clusterNames {
		nodes := clusters[clusterName]
		child := &Feature{
			ID:              "feat-" + clusterName, // Simple ID generation
			Name:            clusterName,
			ScopePath:       pathPrefix,
			MemberFunctions: nodes,
		}

		allEdges = append(allEdges, graph.Edge{
			SourceID: domain.ID,
			TargetID: child.ID,
			Type:     "PARENT_OF",
		})

		for _, fn := range nodes {
			allEdges = append(allEdges, graph.Edge{
				SourceID: fn.ID,
				TargetID: child.ID,
				Type:     "IMPLEMENTS",
			})
		}

		domain.Children = append(domain.Children, child)
	}
	return allEdges
}

func (b *Builder) buildThreeLevel(domain *Feature, funcs []graph.Node, name, pathPrefix string, allEdges []graph.Edge) []graph.Edge {
	// First pass: coarse clustering into categories
	categories, _ := b.CategoryClusterer.Cluster(funcs, name)
	
	catNames := make([]string, 0, len(categories))
	for k := range categories {
		catNames = append(catNames, k)
	}
	sort.Strings(catNames)

	for _, catName := range catNames {
		catNodes := categories[catName]
		category := &Feature{
			ID:              "cat-" + catName,
			Name:            catName,
			ScopePath:       pathPrefix,
			MemberFunctions: catNodes,
			Children:        make([]*Feature, 0),
		}

		allEdges = append(allEdges, graph.Edge{
			SourceID: domain.ID,
			TargetID: category.ID,
			Type:     "PARENT_OF",
		})

		// Second pass: fine-grained clustering within each category
		features, _ := b.Clusterer.Cluster(catNodes, catName)
		
		featNames := make([]string, 0, len(features))
		for k := range features {
			featNames = append(featNames, k)
		}
		sort.Strings(featNames)

		for _, featName := range featNames {
			featNodes := features[featName]
			feature := &Feature{
				ID:              "feat-" + featName,
				Name:            featName,
				ScopePath:       pathPrefix,
				MemberFunctions: featNodes,
			}

			allEdges = append(allEdges, graph.Edge{
				SourceID: category.ID,
				TargetID: feature.ID,
				Type:     "PARENT_OF",
			})

			for _, fn := range featNodes {
				allEdges = append(allEdges, graph.Edge{
					SourceID: fn.ID,
					TargetID: feature.ID,
					Type:     "IMPLEMENTS",
				})
			}

			category.Children = append(category.Children, feature)
		}

		domain.Children = append(domain.Children, category)
	}
	return allEdges
}
