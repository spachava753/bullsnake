package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileGeneratorExpression builds a generator function whose first argument
// receives the enclosing scope's eagerly evaluated first iterable.
func (compiler *compilerState) compileGeneratorExpression(
	expression *compilerast.GeneratorExpr,
) error {
	if len(expression.Clauses) == 0 {
		return compiler.error(expression.Span(), "generator expression has no for clause")
	}
	for _, clause := range expression.Clauses {
		if clause.Async {
			return compiler.error(clause.Range, "asynchronous comprehensions are not compiled")
		}
	}
	scope := compiler.table.ScopeFor(expression, resolver.ComprehensionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
		return compiler.error(expression.Span(), "resolver has no generator expression scope")
	}
	child := compiler.newComprehensionCompiler(expression, scope, "<genexpr>")
	first := expression.Clauses[0]
	if err := child.emit(bytecode.LoadFast, 0, first.Iterable.Span()); err != nil {
		return err
	}
	if err := child.emit(bytecode.GetIter, 0, first.Iterable.Span()); err != nil {
		return err
	}
	if err := child.compileGeneratorClause(expression.Clauses, 0, expression.Element); err != nil {
		return err
	}
	if err := child.emit(
		bytecode.LoadConst,
		child.constantIndex(bytecode.None()),
		expression.Span(),
	); err != nil {
		return err
	}
	if err := child.emit(bytecode.ReturnValue, 0, expression.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, expression.Span()); err != nil {
		return err
	}
	if err := compiler.compileExpr(first.Iterable); err != nil {
		return err
	}
	return compiler.emit(bytecode.Call, 1, expression.Span())
}

// compileGeneratorClause recursively emits nested iteration and filtering,
// suspending with each innermost element while retaining every active iterator.
func (compiler *compilerState) compileGeneratorClause(
	clauses []compilerast.Comprehension,
	index int,
	value compilerast.Expr,
) error {
	clause := clauses[index]
	if index != 0 {
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
		if err := compiler.compileGeneratorClause(clauses, index+1, value); err != nil {
			return err
		}
	} else {
		if err := compiler.compileExpr(value); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.YieldValue, 0, value.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.PopTop, 0, value.Span()); err != nil {
			return err
		}
	}
	if err := compiler.emitJump(bytecode.Jump, start, clause.Range); err != nil {
		return err
	}
	return compiler.markLabel(end, clause.Range)
}

// compileCollectionComprehension evaluates the first iterable in the enclosing
// scope and runs the nested clauses in the resolver's isolated function scope.
func (compiler *compilerState) compileCollectionComprehension(
	owner compilerast.Expr,
	clauses []compilerast.Comprehension,
	codeName string,
	buildOpcode bytecode.Opcode,
	value compilerast.Expr,
	key compilerast.Expr,
) error {
	if len(clauses) == 0 {
		return compiler.error(owner.Span(), "comprehension has no for clause")
	}
	scope := compiler.table.ScopeFor(owner, resolver.ComprehensionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
		return compiler.error(owner.Span(), "resolver has no comprehension scope")
	}

	child := compiler.newComprehensionCompiler(owner, scope, codeName)
	if err := child.emit(buildOpcode, 0, owner.Span()); err != nil {
		return err
	}
	if err := child.emit(bytecode.LoadFast, 0, clauses[0].Iterable.Span()); err != nil {
		return err
	}
	firstIteratorOpcode := bytecode.GetIter
	if clauses[0].Async {
		firstIteratorOpcode = bytecode.GetAIter
	}
	if err := child.emit(firstIteratorOpcode, 0, clauses[0].Iterable.Span()); err != nil {
		return err
	}
	if err := child.compileComprehensionClause(clauses, 0, value, key); err != nil {
		return err
	}
	if err := child.emit(bytecode.ReturnValue, 0, owner.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, owner.Span()); err != nil {
		return err
	}
	if err := compiler.compileExpr(clauses[0].Iterable); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 1, owner.Span()); err != nil {
		return err
	}
	if scope.Flags&resolver.Coroutine != 0 {
		return compiler.emit(bytecode.AwaitValue, 0, owner.Span())
	}
	return nil
}

// compileComprehensionClause keeps each active iterator above the accumulator;
// collection mutation operands record that iterator depth for the VM.
func (compiler *compilerState) compileComprehensionClause(
	clauses []compilerast.Comprehension,
	index int,
	value compilerast.Expr,
	key compilerast.Expr,
) error {
	clause := clauses[index]
	if index != 0 {
		if err := compiler.compileExpr(clause.Iterable); err != nil {
			return err
		}
		opcode := bytecode.GetIter
		if clause.Async {
			opcode = bytecode.GetAIter
		}
		if err := compiler.emit(opcode, 0, clause.Iterable.Span()); err != nil {
			return err
		}
	}

	start := compiler.newLabel()
	end := compiler.newLabel()
	if err := compiler.markLabel(start, clause.Range); err != nil {
		return err
	}
	iterationOpcode := bytecode.ForIter
	if clause.Async {
		iterationOpcode = bytecode.AsyncForIter
	}
	if err := compiler.emitJump(iterationOpcode, end, clause.Range); err != nil {
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
		if err := compiler.compileComprehensionClause(clauses, index+1, value, key); err != nil {
			return err
		}
	} else {
		depth := uint32(len(clauses))
		if key != nil {
			if err := compiler.compileExpr(key); err != nil {
				return err
			}
			if err := compiler.compileExpr(value); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.MapSet, depth, value.Span()); err != nil {
				return err
			}
		} else {
			if err := compiler.compileExpr(value); err != nil {
				return err
			}
			opcode := bytecode.ListAppend
			if compiler.scope.Flags&resolver.SetComprehension != 0 {
				opcode = bytecode.SetAdd
			}
			if err := compiler.emit(opcode, depth, value.Span()); err != nil {
				return err
			}
		}
	}
	if err := compiler.emitJump(bytecode.Jump, start, clause.Range); err != nil {
		return err
	}
	return compiler.markLabel(end, clause.Range)
}

// newComprehensionCompiler installs the synthetic iterable argument before
// resolver-ordered locals so positional binding always targets local slot zero.
func (compiler *compilerState) newComprehensionCompiler(
	owner compilerast.Node,
	scope *resolver.Scope,
	codeName string,
) *compilerState {
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	if scope.Flags&resolver.Generator != 0 {
		flags |= bytecode.Generator
	}
	if scope.Flags&resolver.Coroutine != 0 {
		flags |= bytecode.Coroutine
	}
	child := &compilerState{
		filename:        compiler.filename,
		module:          compiler.module,
		owner:           owner,
		table:           compiler.table,
		scope:           scope,
		codeName:        codeName,
		qualifiedName:   compiler.childQualifiedName(codeName),
		firstLine:       owner.Span().Start.Line,
		codeFlags:       flags,
		positionalCount: 1,
		localIDs:        make(map[string]uint32),
		derefIDs:        make(map[string]uint32),
		constantIDs:     make(map[bytecode.Constant]uint32),
		nameIDs:         make(map[string]uint32),
		reachable:       true,
	}
	child.addLocal(".0")
	child.initializeScopeLayout(scope)
	return child
}
