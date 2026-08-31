package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileFunctionDefinition validates the required-parameter subset, compiles
// one independent child code object, and binds the resulting function.
func (compiler *compilerState) compileFunctionDefinition(statement *compilerast.FunctionDefStmt) error {
	if statement.Async && len(statement.TypeParameters) != 0 {
		return compiler.error(statement.Span(), "generic async functions are not compiled")
	}
	if len(statement.TypeParameters) != 0 {
		return compiler.compileGenericFunctionDefinition(statement)
	}

	scope := compiler.table.ScopeFor(statement, resolver.DefinitionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
		return compiler.error(statement.Span(), "resolver has no function scope for %q", statement.Name)
	}
	if statement.Async && scope.Flags&resolver.Generator != 0 {
		return compiler.error(statement.Span(), "async generators are not compiled")
	}
	for _, decorator := range statement.Decorators {
		if err := compiler.compileExpr(decorator); err != nil {
			return err
		}
	}
	defaults, keywordDefaults, err := compiler.compileFunctionDefaults(
		statement.Parameters,
		statement.Span(),
	)
	if err != nil {
		return err
	}
	annotations, err := compiler.compileFunctionAnnotations(
		statement,
		compiler.childQualifiedName(statement.Name)+".__annotate__",
	)
	if err != nil {
		return err
	}
	child := compiler.newFunctionCompiler(statement, scope, statement.Parameters, statement.Name)

	if err := child.compileStatements(statement.Body); err != nil {
		return err
	}
	position := statement.Span().End
	if err := child.emitImplicitReturn(lexer.Span{Start: position, End: position}); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(
		code,
		defaults,
		keywordDefaults,
		annotations,
		statement.Span(),
	); err != nil {
		return err
	}
	for index := len(statement.Decorators) - 1; index >= 0; index-- {
		decorator := statement.Decorators[index]
		if err := compiler.emit(bytecode.Call, 1, decorator.Span()); err != nil {
			return err
		}
	}
	return compiler.emitNameStore(statement.Name, statement.Span())
}

// compileGenericFunctionDefinition runs TypeVar creation in a hidden scope and
// returns the function whose body closes over those parameters.
func (compiler *compilerState) compileGenericFunctionDefinition(
	statement *compilerast.FunctionDefStmt,
) error {
	typeScope := compiler.table.ScopeFor(statement, resolver.TypeParameters, 0)
	if typeScope == nil || typeScope.Kind != resolver.TypeParametersScope {
		return compiler.error(
			statement.Span(),
			"resolver has no type parameter scope for %q",
			statement.Name,
		)
	}
	functionScope := compiler.table.ScopeFor(statement, resolver.DefinitionBody, 0)
	if functionScope == nil || functionScope.Kind != resolver.FunctionScope {
		return compiler.error(statement.Span(), "resolver has no function scope for %q", statement.Name)
	}
	for _, decorator := range statement.Decorators {
		if err := compiler.compileExpr(decorator); err != nil {
			return err
		}
	}
	defaults, keywordDefaults, err := compiler.compileFunctionDefaults(
		statement.Parameters,
		statement.Span(),
	)
	if err != nil {
		return err
	}
	payloadNames := make([]string, 0, 2)
	if defaults {
		payloadNames = append(payloadNames, ".defaults")
	}
	if keywordDefaults {
		payloadNames = append(payloadNames, ".kwdefaults")
	}

	name := "<generic parameters of " + statement.Name + ">"
	generic := compiler.newTypeParametersCompiler(statement, typeScope, name, payloadNames)
	if err := generic.emitTypeParameters(statement, statement.TypeParameters); err != nil {
		return err
	}
	if err := generic.emitTypeParameterTuple(statement.TypeParameters, statement.Span()); err != nil {
		return err
	}
	for index := range payloadNames {
		if err := generic.emit(bytecode.LoadFast, uint32(index), statement.Span()); err != nil {
			return err
		}
	}
	annotations, err := generic.compileFunctionAnnotations(
		statement,
		compiler.childQualifiedName(statement.Name)+".__annotate__",
	)
	if err != nil {
		return err
	}

	function := generic.newFunctionCompiler(
		statement,
		functionScope,
		statement.Parameters,
		statement.Name,
	)
	// The hidden type-parameter scope does not appear in Python's qualified name.
	function.qualifiedName = compiler.childQualifiedName(statement.Name)
	if err := function.compileStatements(statement.Body); err != nil {
		return err
	}
	position := statement.Span().End
	if err := function.emitImplicitReturn(lexer.Span{Start: position, End: position}); err != nil {
		return err
	}
	functionCode, err := function.finish()
	if err != nil {
		return err
	}
	if err := generic.emitFunction(
		functionCode,
		defaults,
		keywordDefaults,
		annotations,
		statement.Span(),
	); err != nil {
		return err
	}
	if err := generic.emit(bytecode.SetFunctionTypeParameters, 0, statement.Span()); err != nil {
		return err
	}
	if err := generic.emitTerminator(bytecode.ReturnValue, 0, statement.Span()); err != nil {
		return err
	}
	genericCode, err := generic.finish()
	if err != nil {
		return err
	}
	if err := compiler.emitFunction(
		genericCode,
		false,
		false,
		false,
		statement.Span(),
	); err != nil {
		return err
	}
	for depth := len(payloadNames) + 1; depth >= 2; depth-- {
		if err := compiler.emit(bytecode.Swap, uint32(depth), statement.Span()); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.Call, uint32(len(payloadNames)), statement.Span()); err != nil {
		return err
	}
	for index := len(statement.Decorators) - 1; index >= 0; index-- {
		decorator := statement.Decorators[index]
		if err := compiler.emit(bytecode.Call, 1, decorator.Span()); err != nil {
			return err
		}
	}
	return compiler.emitNameStore(statement.Name, statement.Span())
}

// newFunctionCompiler creates one child with callable metadata and resolver-
// ordered locals, cells, and free variables.
func (compiler *compilerState) newFunctionCompiler(
	owner compilerast.Node,
	scope *resolver.Scope,
	parameters compilerast.Parameters,
	codeName string,
) *compilerState {
	qualifiedName := compiler.childQualifiedName(codeName)
	flags := bytecode.Optimized | bytecode.NewLocals
	if scope.Flags&resolver.VarArgs != 0 {
		flags |= bytecode.VarArgs
	}
	if scope.Flags&resolver.VarKeywords != 0 {
		flags |= bytecode.VarKeywords
	}
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
		filename:            compiler.filename,
		module:              compiler.module,
		owner:               owner,
		table:               compiler.table,
		scope:               scope,
		codeName:            codeName,
		qualifiedName:       qualifiedName,
		firstLine:           owner.Span().Start.Line,
		codeFlags:           flags,
		positionalOnlyCount: len(parameters.PositionalOnly),
		positionalCount:     len(parameters.PositionalOnly) + len(parameters.Positional),
		keywordOnlyCount:    len(parameters.KeywordOnly),
		localIDs:            make(map[string]uint32),
		derefIDs:            make(map[string]uint32),
		constantIDs:         make(map[bytecode.Constant]uint32),
		nameIDs:             make(map[string]uint32),
		reachable:           true,
	}
	child.initializeScopeLayout(scope)
	return child
}

func (compiler *compilerState) childQualifiedName(codeName string) string {
	if compiler.codeName == "<module>" {
		return codeName
	}
	if compiler.scope.Kind == resolver.ClassScope {
		return compiler.qualifiedName + "." + codeName
	}
	return compiler.qualifiedName + ".<locals>." + codeName
}

// emitFunction captures a child's free cells, creates the function, and
// consumes closure, annotation, and default payloads in reverse stack order.
func (compiler *compilerState) emitFunction(
	code *bytecode.Code,
	defaults bool,
	keywordDefaults bool,
	annotations bool,
	span lexer.Span,
) error {
	freeVars := code.FreeVars()
	for _, name := range freeVars {
		index, err := compiler.derefIndex(name)
		if err != nil {
			return compiler.error(span, "%v", err)
		}
		if err := compiler.emit(bytecode.LoadClosure, index, span); err != nil {
			return err
		}
	}
	closure := len(freeVars) != 0
	if closure {
		if err := compiler.emit(bytecode.BuildTuple, uint32(len(freeVars)), span); err != nil {
			return err
		}
	}
	childIndex := uint32(len(compiler.children))
	compiler.children = append(compiler.children, code)
	if err := compiler.emit(bytecode.MakeFunction, childIndex, span); err != nil {
		return err
	}
	if closure {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionClosure),
			span,
		); err != nil {
			return err
		}
	}
	if annotations {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionAnnotate),
			span,
		); err != nil {
			return err
		}
	}
	if keywordDefaults {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionKeywordDefaults),
			span,
		); err != nil {
			return err
		}
	}
	if defaults {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionDefaults),
			span,
		); err != nil {
			return err
		}
	}
	return nil
}

// compileFunctionDefaults emits enclosing-scope defaults in Python evaluation
// order. Positional values become one tuple, followed by one sparse keyword map;
// callers must attach the map before the tuple to consume the stack in reverse.
func (compiler *compilerState) compileFunctionDefaults(
	parameters compilerast.Parameters,
	span lexer.Span,
) (bool, bool, error) {
	positionalCount := 0
	for _, group := range [][]compilerast.Parameter{
		parameters.PositionalOnly,
		parameters.Positional,
	} {
		for _, parameter := range group {
			if parameter.Default == nil {
				continue
			}
			if err := compiler.compileExpr(parameter.Default); err != nil {
				return false, false, err
			}
			positionalCount++
		}
	}
	if positionalCount != 0 {
		if err := compiler.emit(bytecode.BuildTuple, uint32(positionalCount), span); err != nil {
			return false, false, err
		}
	}

	keywordCount := 0
	for _, parameter := range parameters.KeywordOnly {
		if parameter.Default == nil {
			continue
		}
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.TextString(parameter.Name)),
			parameter.Range,
		); err != nil {
			return false, false, err
		}
		if err := compiler.compileExpr(parameter.Default); err != nil {
			return false, false, err
		}
		keywordCount++
	}
	if keywordCount != 0 {
		if err := compiler.emit(bytecode.BuildMap, uint32(keywordCount), span); err != nil {
			return false, false, err
		}
	}
	return positionalCount != 0, keywordCount != 0, nil
}

// compileReturnStatement evaluates the result before unwinding lexical cleanup
// blocks, then removes lower loop state while preserving the result.
func (compiler *compilerState) compileReturnStatement(statement *compilerast.ReturnStmt) error {
	if compiler.scope.Kind != resolver.FunctionScope {
		return compiler.error(statement.Span(), "return has no enclosing function")
	}
	if statement.Value == nil {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			statement.Span(),
		); err != nil {
			return err
		}
	} else if err := compiler.compileExpr(statement.Value); err != nil {
		return err
	}
	cleanupState, err := compiler.emitControlCleanupsFrom(0, statement.Span())
	if err != nil {
		compiler.restoreControlCleanups(cleanupState)
		return err
	}
	if compiler.reachable {
		for compiler.stackDepth > 1 {
			if err := compiler.emit(bytecode.Swap, 2, statement.Span()); err != nil {
				compiler.restoreControlCleanups(cleanupState)
				return err
			}
			if err := compiler.emit(bytecode.PopTop, 0, statement.Span()); err != nil {
				compiler.restoreControlCleanups(cleanupState)
				return err
			}
		}
		err = compiler.emitTerminator(bytecode.ReturnValue, 0, statement.Span())
	}
	compiler.restoreControlCleanups(cleanupState)
	return err
}

func (compiler *compilerState) addLocal(name string) {
	if _, exists := compiler.localIDs[name]; exists {
		return
	}
	compiler.localIDs[name] = uint32(len(compiler.locals))
	compiler.locals = append(compiler.locals, name)
}

// initializeScopeLayout assigns deterministic indexes in resolver order. Cells
// precede frees so every dereference opcode uses one stable index space.
func (compiler *compilerState) initializeScopeLayout(scope *resolver.Scope) {
	for _, name := range scope.Parameters {
		compiler.addLocal(name)
	}
	for _, name := range scope.SymbolOrder {
		if symbol := scope.Symbols[name]; symbol != nil && symbol.Resolution == resolver.Local {
			compiler.addLocal(name)
		}
	}
	compiler.initializeDerefLayout(scope)
}

// initializeDerefLayout assigns cells before free variables so every closure
// operation shares one deterministic resolver-ordered index space.
func (compiler *compilerState) initializeDerefLayout(scope *resolver.Scope) {
	for _, name := range scope.SymbolOrder {
		if symbol := scope.Symbols[name]; symbol != nil && symbol.Resolution == resolver.Cell {
			compiler.addCell(name)
		}
	}
	for _, name := range scope.SymbolOrder {
		if symbol := scope.Symbols[name]; symbol != nil && symbol.Resolution == resolver.Free {
			compiler.addFree(name)
		}
	}
}

func (compiler *compilerState) addCell(name string) {
	if _, exists := compiler.derefIDs[name]; exists {
		return
	}
	compiler.derefIDs[name] = uint32(len(compiler.cells))
	compiler.cells = append(compiler.cells, name)
}

func (compiler *compilerState) addFree(name string) {
	if _, exists := compiler.derefIDs[name]; exists {
		return
	}
	compiler.derefIDs[name] = uint32(len(compiler.cells) + len(compiler.freeVars))
	compiler.freeVars = append(compiler.freeVars, name)
}
