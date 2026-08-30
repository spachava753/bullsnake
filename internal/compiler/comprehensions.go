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

// compileEagerComprehension builds a hidden function for the comprehension
// body, then calls it with an iterator evaluated in the enclosing scope.
func (compiler *compilerState) compileEagerComprehension(
	owner compilerast.Expr,
	clauses []compilerast.Comprehension,
	codeName string,
	scopeFlag resolver.ScopeFlags,
	buildOpcode bytecode.Opcode,
	appendValue func(*compilerState) error,
) error {
	if len(clauses) == 0 {
		return compiler.error(owner.Span(), "%s has no clauses", codeName)
	}
	for _, clause := range clauses {
		if clause.Async {
			return compiler.error(clause.Range, "asynchronous comprehensions are not compiled")
		}
	}

	scope := compiler.table.ScopeFor(owner, resolver.ComprehensionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope || scope.Flags&scopeFlag == 0 {
		return compiler.error(owner.Span(), "resolver has no %s scope", codeName)
	}
	child := compiler.newComprehensionCompiler(owner, scope, codeName)
	if err := child.emit(buildOpcode, 0, owner.Span()); err != nil {
		return err
	}
	if err := child.emit(
		bytecode.StoreFast,
		child.localIDs[comprehensionResultLocal],
		owner.Span(),
	); err != nil {
		return err
	}
	if err := child.compileComprehensionClauses(clauses, 0, appendValue); err != nil {
		return err
	}
	if err := child.emit(
		bytecode.LoadFast,
		child.localIDs[comprehensionResultLocal],
		owner.Span(),
	); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, owner.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, owner.Span()); err != nil {
		return err
	}

	first := clauses[0]
	if err := compiler.compileExpr(first.Iterable); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.GetIter, 0, first.Iterable.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.Call, 1, owner.Span())
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

// compileComprehensionClauses emits nested iterator loops recursively; each
// filter jumps to its own loop head and the innermost clause adds one value.
func (compiler *compilerState) compileComprehensionClauses(
	clauses []compilerast.Comprehension,
	index int,
	appendValue func(*compilerState) error,
) error {
	clause := clauses[index]
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
	if index+1 < len(clauses) {
		if err := compiler.compileComprehensionClauses(
			clauses,
			index+1,
			appendValue,
		); err != nil {
			return err
		}
	} else if err := appendValue(compiler); err != nil {
		return err
	}
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, start, clause.Range); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, clause.Range)
}

func (compiler *compilerState) appendComprehensionValue(
	expression compilerast.Expr,
	opcode bytecode.Opcode,
) error {
	if err := compiler.emit(
		bytecode.LoadFast,
		compiler.localIDs[comprehensionResultLocal],
		expression.Span(),
	); err != nil {
		return err
	}
	if err := compiler.compileExpr(expression); err != nil {
		return err
	}
	if err := compiler.emit(opcode, 0, expression.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, expression.Span())
}

func (compiler *compilerState) appendDictComprehensionEntry(
	key compilerast.Expr,
	value compilerast.Expr,
) error {
	if err := compiler.emit(
		bytecode.LoadFast,
		compiler.localIDs[comprehensionResultLocal],
		key.Span(),
	); err != nil {
		return err
	}
	if err := compiler.compileExpr(key); err != nil {
		return err
	}
	if err := compiler.compileExpr(value); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.MapSet, 0, value.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, value.Span())
}
