// Package importer finds and compiles Python modules outside the bytecode runtime.
package importer

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spachava753/bullsnake/host"
	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	"github.com/spachava753/bullsnake/internal/compiler/source"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

// FileSystem finds flat module files under an ordered list of roots.
type FileSystem struct {
	files host.FileSystem
	roots []string
}

// NewFileSystem constructs a loader that searches roots from first to last.
func NewFileSystem(roots ...string) *FileSystem {
	return NewFileSystemWithHost(host.Default().Files, roots...)
}

// NewFileSystemWithHost constructs a loader whose reads pass through files.
// A nil capability denies every otherwise valid module lookup.
func NewFileSystemWithHost(files host.FileSystem, roots ...string) *FileSystem {
	return &FileSystem{files: files, roots: slices.Clone(roots)}
}

// Load finds and compiles a regular package or source module under the first
// configured root that contains it.
func (loader *FileSystem) Load(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
	name := request.Name
	if name == "" || strings.ContainsAny(name, `/\\`) {
		return bullruntime.ModuleSpec{}, false, nil
	}
	parts := strings.Split(name, ".")
	for _, part := range parts {
		if part == "" {
			return bullruntime.ModuleSpec{}, false, nil
		}
	}
	roots := loader.roots
	relative := filepath.Join(parts...)
	if request.SearchLocations != nil {
		roots = request.SearchLocations
		relative = parts[len(parts)-1]
	}
	var namespaceLocations []string
	for _, root := range roots {
		packageFile := filepath.Join(root, relative, "__init__.py")
		unit, found, err := loader.readUnit(packageFile)
		if err != nil {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("load module %q: %w", name, err)
		}
		if found {
			code, err := compileUnit(unit)
			if err != nil {
				return bullruntime.ModuleSpec{}, true, fmt.Errorf("compile module %q: %w", name, err)
			}
			return bullruntime.ModuleSpec{
				Code:            code,
				IsPackage:       true,
				Origin:          packageFile,
				SearchLocations: []string{filepath.Dir(packageFile)},
			}, true, nil
		}
		packageDirectory := filepath.Join(root, relative)
		info, statErr := loader.files.Stat(packageDirectory)
		if statErr == nil && info.IsDir() {
			namespaceLocations = append(namespaceLocations, packageDirectory)
		} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("load module %q: %w", name, statErr)
		}

		moduleFile := filepath.Join(root, relative+".py")
		unit, found, err = loader.readUnit(moduleFile)
		if err != nil {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("load module %q: %w", name, err)
		}
		if found {
			code, err := compileUnit(unit)
			if err != nil {
				return bullruntime.ModuleSpec{}, true, fmt.Errorf("compile module %q: %w", name, err)
			}
			return bullruntime.ModuleSpec{Code: code, Origin: moduleFile}, true, nil
		}
	}
	if len(namespaceLocations) != 0 {
		code, err := compileUnit(source.Unit{Filename: "<namespace " + name + ">", Text: ""})
		if err != nil {
			return bullruntime.ModuleSpec{}, true, fmt.Errorf("compile module %q: %w", name, err)
		}
		return bullruntime.ModuleSpec{
			Code: code, IsPackage: true, IsNamespace: true,
			SearchLocations: namespaceLocations,
		}, true, nil
	}
	return bullruntime.ModuleSpec{}, false, nil
}

func (loader *FileSystem) readUnit(filename string) (source.Unit, bool, error) {
	unit, err := source.ReadFileWithHost(loader.files, filename)
	if errors.Is(err, fs.ErrNotExist) {
		return source.Unit{}, false, nil
	}
	if err != nil {
		return source.Unit{}, false, err
	}
	return unit, true, err
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
