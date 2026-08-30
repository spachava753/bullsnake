// Package importer finds and compiles Python modules outside the bytecode runtime.
package importer

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	"github.com/spachava753/bullsnake/internal/compiler/source"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

// FileSystem finds flat module files under an ordered list of roots.
type FileSystem struct {
	roots []string
}

// NewFileSystem constructs a loader that searches roots from first to last.
func NewFileSystem(roots ...string) *FileSystem {
	return &FileSystem{roots: slices.Clone(roots)}
}

// Load finds and compiles name.py. A false result means no configured root
// contains the requested flat module.
func (loader *FileSystem) Load(name string) (bullruntime.ModuleSpec, bool, error) {
	if name == "" || strings.Contains(name, ".") || strings.ContainsAny(name, `/\\`) {
		return bullruntime.ModuleSpec{}, false, nil
	}
	for _, root := range loader.roots {
		filename := filepath.Join(root, name+".py")
		unit, err := source.ReadFile(filename)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("load module %q: %w", name, err)
		}
		code, err := compileUnit(unit)
		if err != nil {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("compile module %q: %w", name, err)
		}
		return bullruntime.ModuleSpec{Code: code, Origin: filename}, true, nil
	}
	return bullruntime.ModuleSpec{}, false, nil
}

func compileUnit(unit source.Unit) (*bytecode.Code, error) {
	module, err := parser.Parse(unit.Filename, unit.Text)
	if err != nil {
		return nil, err
	}
	table, err := resolver.Resolve(unit.Filename, module)
	if err != nil {
		return nil, err
	}
	return compiler.Compile(unit.Filename, module, table)
}
