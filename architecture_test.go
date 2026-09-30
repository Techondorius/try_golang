package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep dependency direction enforceable as the sample grows.
func TestDependencyDirection(t *testing.T) {
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		layer := strings.Split(filepath.ToSlash(path), "/")[1]
		for _, spec := range source.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			standard := !strings.Contains(strings.Split(imported, "/")[0], ".") && !strings.HasPrefix(imported, "try_golang/")
			domain := strings.HasPrefix(imported, "try_golang/internal/domain/")
			application := imported == "try_golang/internal/application"
			allowed := standard
			switch layer {
			case "domain", "config":
			case "application":
				allowed = allowed || domain
			case "transport":
				allowed = allowed || domain || application || imported == "github.com/gin-gonic/gin"
			case "infrastructure":
				allowed = allowed || domain || application || !strings.HasPrefix(imported, "try_golang/")
			default:
				t.Errorf("unknown layer in %s", path)
			}
			if !allowed {
				t.Errorf("%s: forbidden dependency %s", path, imported)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
