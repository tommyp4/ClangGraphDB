package ingest

import (
	"context"
	"graphdb/internal/embedding"
	"graphdb/internal/storage"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/monochromegane/go-gitignore"
)

type Walker struct {
	WorkerPool *WorkerPool
}

func NewWalker(workers int, embedder embedding.Embedder, emitter storage.Emitter) *Walker {
	return &Walker{
		WorkerPool: NewWorkerPool(workers, embedder, emitter),
	}
}

func (w *Walker) Run(ctx context.Context, dirPath string) error {
	w.WorkerPool.Start()
	defer w.WorkerPool.Stop()

	return w.Walk(ctx, dirPath, func(path string, d fs.DirEntry) error {
		if !d.IsDir() {
			w.WorkerPool.Submit(path)
		}
		return nil
	})
}

func (w *Walker) Count(ctx context.Context, dirPath string) (int64, error) {
	var count int64
	err := w.Walk(ctx, dirPath, func(path string, d fs.DirEntry) error {
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

func (w *Walker) Walk(ctx context.Context, dirPath string, visitor func(path string, d fs.DirEntry) error) error {
	// Check for .gitignore in the root directory
	gitignorePath := filepath.Join(dirPath, ".gitignore")
	var ignoreMatcher gitignore.IgnoreMatcher

	// Check if file exists
	if _, err := os.Stat(gitignorePath); err == nil {
		matcher, err := gitignore.NewGitIgnore(gitignorePath)
		if err == nil {
			ignoreMatcher = matcher
		}
	}

	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Always ignore .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check .gitignore
		if ignoreMatcher != nil {
			// Calculate relative path for matching
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}

			// "." means root, which we don't want to skip
			if relPath == "." {
				return nil
			}

			if ignoreMatcher.Match(path, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil // Skip file
			}
		}

		return visitor(path, d)
	})
}
