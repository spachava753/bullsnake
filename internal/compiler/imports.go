package compiler

import (
	"strconv"
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// compileImportStatement imports each module independently. Dotted aliases keep
// one base module below the selected component until the final binding is stored.
func (compiler *compilerState) compileImportStatement(statement *compilerast.ImportStmt) error {
	for _, imported := range statement.Names {
		if err := compiler.emitImportName(imported.Name, 0, nil, imported.Range); err != nil {
			return err
		}
		parts := strings.Split(imported.Name, ".")
		if imported.Alias != "" && len(parts) > 1 {
			for index, part := range parts[1:] {
				if err := compiler.emit(bytecode.ImportFrom, compiler.nameIndex(part), imported.Range); err != nil {
					return err
				}
				if index != len(parts)-2 {
					if err := compiler.emit(bytecode.Swap, 2, imported.Range); err != nil {
						return err
					}
					if err := compiler.emit(bytecode.PopTop, 0, imported.Range); err != nil {
						return err
					}
				}
			}
			if err := compiler.emitNameStore(imported.Alias, imported.Range); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.PopTop, 0, imported.Range); err != nil {
				return err
			}
			continue
		}
		binding := imported.Alias
		if binding == "" {
			binding = parts[0]
		}
		if err := compiler.emitNameStore(binding, imported.Range); err != nil {
			return err
		}
	}
	return nil
}

// compileFromImportStatement imports one module, preserves it beneath ordinary
// imported values, and consumes it after all resolver-selected stores.
func (compiler *compilerState) compileFromImportStatement(statement *compilerast.FromImportStmt) error {
	fromNames := make([]string, 0, len(statement.Names))
	if statement.Wildcard {
		fromNames = append(fromNames, "*")
	} else {
		for _, imported := range statement.Names {
			fromNames = append(fromNames, imported.Name)
		}
	}
	if err := compiler.emitImportName(statement.Module, statement.Level, fromNames, statement.Span()); err != nil {
		return err
	}
	if statement.Wildcard {
		return compiler.emit(bytecode.ImportStar, 0, statement.Span())
	}
	for _, imported := range statement.Names {
		if err := compiler.emit(bytecode.ImportFrom, compiler.nameIndex(imported.Name), imported.Range); err != nil {
			return err
		}
		binding := imported.Alias
		if binding == "" {
			binding = imported.Name
		}
		if err := compiler.emitNameStore(binding, imported.Range); err != nil {
			return err
		}
	}
	return compiler.emit(bytecode.PopTop, 0, statement.Span())
}

// emitImportName pushes the relative level and either None or a built from-list
// before emitting the shared module-name operand.
func (compiler *compilerState) emitImportName(
	module string,
	level int,
	fromNames []string,
	span lexer.Span,
) error {
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer(strconv.Itoa(level))),
		span,
	); err != nil {
		return err
	}
	if fromNames == nil {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			span,
		); err != nil {
			return err
		}
	} else {
		for _, name := range fromNames {
			if err := compiler.emit(
				bytecode.LoadConst,
				compiler.constantIndex(bytecode.TextString(name)),
				span,
			); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.BuildTuple, uint32(len(fromNames)), span); err != nil {
			return err
		}
	}
	return compiler.emit(bytecode.ImportName, compiler.nameIndex(module), span)
}
