package ingest

import (
	"fmt"
	"clang-graphdb/internal/analysis"
	"clang-graphdb/internal/graph"
	"clang-graphdb/internal/storage"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Job struct {
	Root string
	Path string
}

type WorkerPool struct {
	workers    int
	emitter    storage.Emitter
	jobChan    chan Job
	wg         sync.WaitGroup
	OnProgress func()
}

func NewWorkerPool(workers int, emitter storage.Emitter) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		emitter:  emitter,
		jobChan:  make(chan Job, 100),
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for job := range wp.jobChan {
		if err := wp.processFile(job); err != nil {
			log.Printf("Error processing file %s: %v", job.Path, err)
		}
		if wp.OnProgress != nil {
			wp.OnProgress()
		}
	}
}

func (wp *WorkerPool) processFile(job Job) error {
	path := job.Path
	ext := filepath.Ext(path)
	parser, ok := analysis.GetParser(ext)
	if !ok {
		return nil // Not supported
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	relPath, err := filepath.Rel(job.Root, path)
	if err != nil {
		// Fallback to absolute path if relative path calculation fails
		relPath = path
	}
	relPath = filepath.ToSlash(relPath)

	nodes, edges, err := parser.Parse(relPath, content)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Create File Node
	fileNode := &graph.Node{
		ID:    relPath,
		Label: "File",
		Properties: map[string]interface{}{
			"file": relPath,
			"name": relPath,
		},
	}

	isTestFile := analysis.IsTestFile(relPath)
	if isTestFile {
		fileNode.Properties["is_test"] = true
		for _, node := range nodes {
			if node.Label == "Function" || node.Label == "Method" {
				if node.Properties == nil {
					node.Properties = make(map[string]interface{})
				}
				node.Properties["is_test"] = true
			}
		}
	}

	// Link nodes to File
	var definedInEdges []*graph.Edge
	for _, node := range nodes {
		if node.Label == "Function" || node.Label == "Method" || node.Label == "Class" {
			definedInEdges = append(definedInEdges, &graph.Edge{
				SourceID: node.ID,
				TargetID: fileNode.ID,
				Type:     "DEFINED_IN",
			})
		}
	}

	// Emit
	if err := wp.emitter.EmitNode(fileNode); err != nil {
		return fmt.Errorf("failed to emit file node: %w", err)
	}
	for _, edge := range definedInEdges {
		if err := wp.emitter.EmitEdge(edge); err != nil {
			return fmt.Errorf("failed to emit defined_in edge: %w", err)
		}
	}
	for _, node := range nodes {
		if err := wp.emitter.EmitNode(node); err != nil {
			return fmt.Errorf("failed to emit node: %w", err)
		}
	}
	for _, edge := range edges {
		if err := wp.emitter.EmitEdge(edge); err != nil {
			return fmt.Errorf("failed to emit edge: %w", err)
		}
	}

	return nil
}

func (wp *WorkerPool) Submit(root string, filePath string) {
	wp.jobChan <- Job{Root: root, Path: filePath}
}

func (wp *WorkerPool) Stop() {
	close(wp.jobChan)
	wp.wg.Wait()
}
