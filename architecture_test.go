package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestAggregateRepositoryPlacement は、集約の保存契約がドメインに定義されることを検証します。
// 実装を移動するときにも、ドメインがapplicationやinfrastructureへ依存する形に戻さないための検査です。
func TestAggregateRepositoryPlacement(t *testing.T) {
	for _, repository := range []struct {
		path string
		name string
	}{
		{"internal/domain/account/repository.go", "IUsersRepository"},
		{"internal/domain/task/repository.go", "ITasksRepository"},
	} {
		t.Run(repository.name, func(t *testing.T) {
			source, err := parser.ParseFile(token.NewFileSet(), repository.path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range source.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range general.Specs {
					definition, ok := spec.(*ast.TypeSpec)
					if !ok || definition.Name.Name != repository.name {
						continue
					}
					if _, ok := definition.Type.(*ast.InterfaceType); !ok {
						t.Fatalf("%s must be an interface", repository.name)
					}
					return
				}
			}
			t.Fatalf("%s must be declared in %s", repository.name, repository.path)
		})
	}
}

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
			sharedDomain := imported == "try_golang/internal/domain"
			domain := sharedDomain || strings.HasPrefix(imported, "try_golang/internal/domain/")
			application := imported == "try_golang/internal/application"
			allowed := standard
			switch layer {
			case "domain":
				// 集約間の依存を増やさず、Repositoryの共通エラーだけを共有します。
				allowed = allowed || sharedDomain
			case "config":
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
