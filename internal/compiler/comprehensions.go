package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

const (
	comprehensionIteratorLocal = ".0"
	comprehensionResultLocal   = ".result"
)

// compileListComprehension builds a hidden function for the comprehension body,
// then calls it with an iterator evaluated in the enclosing scope.
func (compiler *compilerState) compileListComprehension(
	expression *compilerast.ListComprehensionExpr,
) error {
	if len(expression.Clauses) == 0 {
		return compiler.error(expression.Span(), "list comprehension has no clauses")
	}
	for _, clause := range expression.Clauses {
		if clause.Async {
			return compiler.error(clause.Range, "asynchronous comprehensions are not compiled")
		}
	}

	scope := compiler.table.ScopeFor(expression, resolver.ComprehensionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
		return compiler.error(expression.Span(), "resolver has no list comprehension scope")
	}
	child := compiler.newComprehensionCompiler(expression, scope, "<listcomp>")
	if err := child.emit(bytecode.BuildList, 0, expression.Span()); err != nil {
		return err
	}
	if err := child.emit(
		bytecode.StoreFast,
		child.localIDs[comprehensionResultLocal],
		expression.Span(),
	); err != nil {
		return err
	}
	if err := child.compileListComprehensionClauses(expression, 0); err != nil {
		return err
	}
	if err := child.emit(
		bytecode.LoadFast,
		child.localIDs[comprehensionResultLocal],
		expression.Span(),
	); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, expression.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, expression.Span()); err != nil {
		return err
	}

	first := expression.Clauses[0]
	if err := compiler.compileExpr(first.Iterable); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.GetIter, 0, first.Iterable.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.Call, 1, expression.Span())
}

func (compiler *compilerState) newComprehensionCompiler(
	owner compilerast.Node,
	scope *resolver.Scope,
	codeName string,
) *compilerState {
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:            compiler.filename,
		module:              compiler.module,
		owner:               owner,
		table:               compiler.table,
		scope:               scope,
		codeName:            codeName,
		qualifiedName:       compiler.childQualifiedName(codeName),
		firstLine:           owner.Span().Start.Line,
		codeFlags:           flags,
		positionalOnlyCount: 1,
		positionalCount:     1,
		localIDs:            make(map[string]uint32),
		derefIDs:            make(map[string]uint32),
		constantIDs:         make(map[bytecode.Constant]uint32),
		nameIDs:             make(map[string]uint32),
		reachable:           true,
	}
	child.addLocal(comprehensionIteratorLocal)
	child.addLocal(comprehensionResultLocal)
	child.initializeScopeLayout(scope)
	return child
}

// compileListComprehensionClauses emits nested iterator loops recursively; each
// filter jumps to its own loop head and the innermost clause appends one value.
func (compiler *compilerState) compileListComprehensionClauses(
	expression *compilerast.ListComprehensionExpr,
	index int,
) error {
	clause := expression.Clauses[index]
	if index == 0 {
		if err := compiler.emit(
			bytecode.LoadFast,
			compiler.localIDs[comprehensionIteratorLocal],
			clause.Iterable.Span(),
		); err != nil {
			return err
		}
	} else {
		if err := compiler.compileExpr(clause.Iterable); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.GetIter, 0, clause.Iterable.Span()); err != nil {
			return err
		}
	}

	start := compiler.newLabel()
	end := compiler.newLabel()
	if err := compiler.markLabel(start, clause.Range); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.ForIter, end, clause.Range); err != nil {
		return err
	}
	if err := compiler.compileStore(clause.Target); err != nil {
		return err
	}
	for _, condition := range clause.Conditions {
		if err := compiler.compileExpr(condition); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.PopJumpIfFalse, start, condition.Span()); err != nil {
			return err
		}
	}
	if index+1 < len(expression.Clauses) {
		if err := compiler.compileListComprehensionClauses(expression, index+1); err != nil {
			return err
		}
	} else if err := compiler.appendListComprehensionElement(expression); err != nil {
		return err
	}
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, start, clause.Range); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, clause.Range)
}

func (compiler *compilerState) appendListComprehensionElement(
	expression *compilerast.ListComprehensionExpr,
) error {
	if err := compiler.emit(
		bytecode.LoadFast,
		compiler.localIDs[comprehensionResultLocal],
		expression.Element.Span(),
	); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression.Element); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.ListAppend, 0, expression.Element.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, expression.Element.Span())
}
