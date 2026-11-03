package main_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Guard the dependency direction independently of implementation tests.
func TestArchitectureBoundaries(t *testing.T) {
	for _, dir := range []string{"domain", "usecase", "repository", "api/route"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if dir == "api/route" && filepath.Base(path) != "factory.go" {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				imp, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				forbidden := false
				switch dir {
				case "domain":
					// Domain contracts depend only on the standard library.
					forbidden = strings.Contains(strings.Split(imp, "/")[0], ".") || strings.HasPrefix(imp, "shadmin/")
				case "usecase":
					forbidden = strings.HasPrefix(imp, "shadmin/") && imp != "shadmin/domain"
					forbidden = forbidden || imp == "net/http" || strings.HasPrefix(imp, "github.com/gin-gonic/") || strings.HasPrefix(imp, "entgo.io/")
				case "repository":
					forbidden = strings.HasPrefix(imp, "shadmin/api") || strings.HasPrefix(imp, "shadmin/bootstrap") || strings.HasPrefix(imp, "shadmin/usecase") || strings.HasPrefix(imp, "github.com/gin-gonic/")
				case "api/route":
					forbidden = strings.HasPrefix(imp, "shadmin/ent") || strings.HasPrefix(imp, "shadmin/repository")
				}
				if forbidden {
					t.Errorf("%s imports forbidden dependency %s", path, imp)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
