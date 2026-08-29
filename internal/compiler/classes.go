package compiler

import (
	"strconv"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileClassDefinition evaluates decorators first, creates a namespace body
// function, calls the class builder with ordinary bases, and binds the result.
func (compiler *compilerState) compileClassDefinition(statement *compilerast.ClassDefStmt) error {
	if len(statement.TypeParameters) != 0 {
		return compiler.error(statement.Span(), "generic classes are not compiled")
	}
	scope := compiler.table.ScopeFor(statement, resolver.DefinitionBody, 0)
	if scope == nil || scope.Kind != resolver.ClassScope {
		return compiler.error(statement.Span(), "resolver has no class scope for %q", statement.Name)
	}
	for _, decorator := range statement.Decorators {
		if err := compiler.compileExpr(decorator); err != nil {
			return err
		}
	}

	child := &compilerState{
		filename:      compiler.filename,
		module:        compiler.module,
		owner:         statement,
		table:         compiler.table,
		scope:         scope,
		codeName:      statement.Name,
		qualifiedName: compiler.childQualifiedName(statement.Name),
		firstLine:     statement.Span().Start.Line,
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		reachable:     true,
	}
	needsClassClosure := scope.Flags&resolver.NeedsClassClosure != 0
	needsClassDict := scope.Flags&resolver.NeedsClassDict != 0
	if needsClassClosure {
		child.addCell("__class__")
	}
	if needsClassDict {
		child.addCell("__classdict__")
	}
	child.initializeDerefLayout(scope)
	if err := child.emitClassNamespace(statement.Span()); err != nil {
		return err
	}
	if needsClassDict {
		index, indexErr := child.derefIndex("__classdict__")
		if indexErr != nil {
			return compiler.error(statement.Span(), "%v", indexErr)
		}
		if err := child.emit(bytecode.LoadLocals, 0, statement.Span()); err != nil {
			return err
		}
		if err := child.emit(bytecode.StoreDeref, index, statement.Span()); err != nil {
			return err
		}
	}
	if err := child.compileStatements(statement.Body); err != nil {
		return err
	}
	if err := child.emitClassReturn(needsClassClosure, needsClassDict, statement.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}

	if err := compiler.emit(bytecode.LoadBuildClass, 0, statement.Span()); err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(statement.Name)),
		statement.Span(),
	); err != nil {
		return err
	}
	expanded := len(statement.Keywords) != 0
	for _, base := range statement.Bases {
		if _, starred := base.(*compilerast.StarredExpr); starred {
			expanded = true
			break
		}
	}
	if !expanded {
		for _, base := range statement.Bases {
			if err := compiler.compileExpr(base); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.Call, uint32(2+len(statement.Bases)), statement.Span()); err != nil {
			return err
		}
	} else {
		if err := compiler.emit(bytecode.BuildList, 2, statement.Span()); err != nil {
			return err
		}
		for _, base := range statement.Bases {
			if starred, ok := base.(*compilerast.StarredExpr); ok {
				if err := compiler.compileExpr(starred.Value); err != nil {
					return err
				}
				if err := compiler.emit(bytecode.ListExtend, 0, base.Span()); err != nil {
					return err
				}
			} else {
				if err := compiler.compileExpr(base); err != nil {
					return err
				}
				if err := compiler.emit(bytecode.ListAppend, 0, base.Span()); err != nil {
					return err
				}
			}
		}
		if err := compiler.emit(bytecode.ListToTuple, 0, statement.Span()); err != nil {
			return err
		}
		if len(statement.Keywords) == 0 {
			if err := compiler.emit(bytecode.CallEx, bytecode.CallExNoKeywords, statement.Span()); err != nil {
				return err
			}
		} else {
			if err := compiler.compileKeywordArguments(statement.Keywords, statement.Span()); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.CallEx, bytecode.CallExWithKeywords, statement.Span()); err != nil {
				return err
			}
		}
	}
	for index := len(statement.Decorators) - 1; index >= 0; index-- {
		decorator := statement.Decorators[index]
		if err := compiler.emit(bytecode.Call, 1, decorator.Span()); err != nil {
			return err
		}
	}
	return compiler.emitNameStore(statement.Name, statement.Span())
}

// emitClassNamespace initializes the metadata entries that Python expects in a
// fresh class namespace before compiling user statements.
func (compiler *compilerState) emitClassNamespace(span lexer.Span) error {
	if err := compiler.emit(bytecode.LoadName, compiler.nameIndex("__name__"), span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.StoreName, compiler.nameIndex("__module__"), span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(compiler.qualifiedName)),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.StoreName, compiler.nameIndex("__qualname__"), span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.Integer(strconv.Itoa(compiler.firstLine))),
		span,
	); err != nil {
		return err
	}
	return compiler.emit(bytecode.StoreName, compiler.nameIndex("__firstlineno__"), span)
}

// emitClassReturn publishes synthetic class cells and returns the `__class__`
// cell when methods need it; other reachable class bodies return None.
func (compiler *compilerState) emitClassReturn(
	needsClassCell bool,
	needsClassDict bool,
	span lexer.Span,
) error {
	if !compiler.reachable {
		return nil
	}
	if needsClassDict {
		index, err := compiler.derefIndex("__classdict__")
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		if err := compiler.emit(bytecode.LoadClosure, index, span); err != nil {
			return err
		}
		if err := compiler.emit(
			bytecode.StoreName,
			compiler.nameIndex("__classdictcell__"),
			span,
		); err != nil {
			return err
		}
	}
	if !needsClassCell {
		return compiler.emitImplicitReturn(span)
	}
	index, err := compiler.derefIndex("__class__")
	if err != nil {
		return compiler.error(span, "%v", err)
	}
	if err := compiler.emit(bytecode.LoadClosure, index, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.StoreName, compiler.nameIndex("__classcell__"), span); err != nil {
		return err
	}
	return compiler.emitTerminator(bytecode.ReturnValue, 0, span)
}
