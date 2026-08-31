package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileTypeAlias creates the hidden functions needed to construct one lazy
// alias, then binds the resulting alias in the defining scope.
func (compiler *compilerState) compileTypeAlias(statement *compilerast.TypeAliasStmt) error {
	if len(statement.TypeParameters) == 0 {
		if err := compiler.emitTypeAliasObject(statement); err != nil {
			return err
		}
		return compiler.emitNameStore(statement.Name, statement.Span())
	}
	return compiler.compileGenericTypeAlias(statement)
}

// compileGenericTypeAlias runs type-parameter creation in a hidden scope so
// the lazy value function can close over the resulting TypeVars.
func (compiler *compilerState) compileGenericTypeAlias(
	statement *compilerast.TypeAliasStmt,
) error {
	scope := compiler.table.ScopeFor(statement, resolver.TypeParameters, 0)
	if scope == nil || scope.Kind != resolver.TypeParametersScope {
		return compiler.error(
			statement.Span(),
			"resolver has no type parameter scope for %q",
			statement.Name,
		)
	}
	name := "<generic parameters of " + statement.Name + ">"
	child := compiler.newTypeParametersCompiler(statement, scope, name)
	for index, parameter := range statement.TypeParameters {
		if err := child.emit(
			bytecode.LoadConst,
			child.constantIndex(bytecode.TextString(parameter.Name)),
			parameter.Range,
		); err != nil {
			return err
		}
		makeOpcode := bytecode.MakeTypeVar
		switch parameter.Kind {
		case compilerast.TypeVariableTuple:
			makeOpcode = bytecode.MakeTypeVarTuple
		case compilerast.ParameterSpecification:
			makeOpcode = bytecode.MakeParamSpec
		}
		if err := child.emit(makeOpcode, 0, parameter.Range); err != nil {
			return err
		}
		if parameter.Bound != nil {
			if err := child.emitTypeParameterBound(statement, parameter, index); err != nil {
				return err
			}
		}
		if parameter.Default != nil {
			if err := child.emitTypeParameterEvaluator(
				statement,
				parameter,
				index,
				resolver.TypeVariableDefault,
				parameter.Default,
				"<default of "+parameter.Name+">",
				bytecode.SetTypeVarDefault,
			); err != nil {
				return err
			}
		}
		if err := child.emitNameStore(parameter.Name, parameter.Range); err != nil {
			return err
		}
	}
	for _, parameter := range statement.TypeParameters {
		if err := child.emitNameLoad(parameter.Name, parameter.Range); err != nil {
			return err
		}
	}
	if err := child.emit(
		bytecode.BuildTuple,
		uint32(len(statement.TypeParameters)),
		statement.Span(),
	); err != nil {
		return err
	}
	if err := child.emitTypeAliasObject(statement); err != nil {
		return err
	}
	if err := child.emit(bytecode.SetTypeAliasParameters, 0, statement.Span()); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, statement.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 0, statement.Span()); err != nil {
		return err
	}
	return compiler.emitNameStore(statement.Name, statement.Span())
}

// newTypeParametersCompiler creates the hidden scope that owns PEP 695 type
// parameters and any child definitions that capture them.
func (compiler *compilerState) newTypeParametersCompiler(
	owner compilerast.Node,
	scope *resolver.Scope,
	name string,
) *compilerState {
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:      compiler.filename,
		module:        compiler.module,
		owner:         owner,
		table:         compiler.table,
		scope:         scope,
		codeName:      name,
		qualifiedName: compiler.childQualifiedName(name),
		firstLine:     owner.Span().Start.Line,
		codeFlags:     flags,
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		reachable:     true,
	}
	child.initializeScopeLayout(scope)
	if scope.Flags&resolver.CanSeeClassScope != 0 {
		child.addFree("__classdict__")
	}
	return child
}

// emitTypeParameterBound creates the lazy evaluator selected by the resolver
// and attaches it to the TypeVar currently on the operand stack.
func (compiler *compilerState) emitTypeParameterBound(
	statement *compilerast.TypeAliasStmt,
	parameter compilerast.TypeParameter,
	index int,
) error {
	opcode := bytecode.SetTypeVarBound
	if _, constraints := parameter.Bound.(*compilerast.TupleExpr); constraints {
		opcode = bytecode.SetTypeVarConstraints
	}
	return compiler.emitTypeParameterEvaluator(
		statement,
		parameter,
		index,
		resolver.TypeVariableBound,
		parameter.Bound,
		"<bound of "+parameter.Name+">",
		opcode,
	)
}

// emitTypeParameterEvaluator compiles one lazy bound, constraints, or default
// child and attaches it to the TypeVar currently on the operand stack.
func (compiler *compilerState) emitTypeParameterEvaluator(
	statement *compilerast.TypeAliasStmt,
	parameter compilerast.TypeParameter,
	index int,
	purpose resolver.ScopePurpose,
	expression compilerast.Expr,
	name string,
	opcode bytecode.Opcode,
) error {
	scope := compiler.table.ScopeFor(statement, purpose, index)
	if scope == nil || scope.Kind != resolver.TypeVariableScope {
		return compiler.error(
			parameter.Range,
			"resolver has no evaluator scope for type parameter %q",
			parameter.Name,
		)
	}
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
	}
	child := &compilerState{
		filename:      compiler.filename,
		module:        compiler.module,
		owner:         statement,
		table:         compiler.table,
		scope:         scope,
		codeName:      name,
		qualifiedName: compiler.childQualifiedName(name),
		firstLine:     parameter.Range.Start.Line,
		codeFlags:     flags,
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		reachable:     true,
	}
	child.initializeScopeLayout(scope)
	if scope.Flags&resolver.CanSeeClassScope != 0 {
		child.addFree("__classdict__")
	}
	if err := child.compileExpr(expression); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, expression.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, parameter.Range); err != nil {
		return err
	}
	return compiler.emit(opcode, 0, parameter.Range)
}

// emitTypeAliasObject leaves one unevaluated alias on the operand stack.
func (compiler *compilerState) emitTypeAliasObject(statement *compilerast.TypeAliasStmt) error {
	scope := compiler.table.ScopeFor(statement, resolver.TypeAliasValue, 0)
	if scope == nil || scope.Kind != resolver.TypeAliasScope {
		return compiler.error(statement.Span(), "resolver has no type alias scope for %q", statement.Name)
	}

	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.Nested != 0 {
		flags |= bytecode.Nested
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
		codeFlags:     flags,
		localIDs:      make(map[string]uint32),
		derefIDs:      make(map[string]uint32),
		constantIDs:   make(map[bytecode.Constant]uint32),
		nameIDs:       make(map[string]uint32),
		reachable:     true,
	}
	child.initializeScopeLayout(scope)
	if scope.Flags&resolver.CanSeeClassScope != 0 {
		child.addFree("__classdict__")
	}
	if err := child.compileExpr(statement.Value); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, statement.Value.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}

	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(statement.Name)),
		statement.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emitFunction(code, false, false, false, statement.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.MakeTypeAlias, 0, statement.Span())
}
