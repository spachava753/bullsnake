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
	if statement.Async {
		return compiler.error(statement.Span(), "async functions are not compiled")
	}
	if len(statement.TypeParameters) != 0 {
		return compiler.error(statement.Span(), "generic functions are not compiled")
	}
	if statement.Returns != nil {
		return compiler.error(statement.Returns.Span(), "function annotations are not compiled")
	}
	parameterGroups := [][]compilerast.Parameter{
		statement.Parameters.PositionalOnly,
		statement.Parameters.Positional,
		statement.Parameters.KeywordOnly,
	}
	for _, group := range parameterGroups {
		for _, parameter := range group {
			if parameter.Annotation != nil {
				return compiler.error(parameter.Range, "parameter annotations are not compiled")
			}
		}
	}
	for _, parameter := range []*compilerast.Parameter{
		statement.Parameters.VarArg,
		statement.Parameters.KeywordVarArg,
	} {
		if parameter != nil && parameter.Annotation != nil {
			return compiler.error(parameter.Range, "parameter annotations are not compiled")
		}
	}

	scope := compiler.table.ScopeFor(statement, resolver.DefinitionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
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
	qualifiedName := statement.Name
	if compiler.codeName != "<module>" {
		qualifiedName = compiler.qualifiedName + ".<locals>." + statement.Name
	}
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
	child := &compilerState{
		filename:            compiler.filename,
		module:              compiler.module,
		owner:               statement,
		table:               compiler.table,
		scope:               scope,
		codeName:            statement.Name,
		qualifiedName:       qualifiedName,
		firstLine:           statement.Span().Start.Line,
		codeFlags:           flags,
		positionalOnlyCount: len(statement.Parameters.PositionalOnly),
		positionalCount:     len(statement.Parameters.PositionalOnly) + len(statement.Parameters.Positional),
		keywordOnlyCount:    len(statement.Parameters.KeywordOnly),
		localIDs:            make(map[string]uint32),
		constantIDs:         make(map[bytecode.Constant]uint32),
		nameIDs:             make(map[string]uint32),
		reachable:           true,
	}
	for _, name := range scope.Parameters {
		child.addLocal(name)
	}
	for _, name := range scope.SymbolOrder {
		symbol := scope.Symbols[name]
		if symbol != nil && symbol.Resolution == resolver.Local {
			child.addLocal(name)
		}
	}

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
	childIndex := uint32(len(compiler.children))
	compiler.children = append(compiler.children, code)
	if err := compiler.emit(bytecode.MakeFunction, childIndex, statement.Span()); err != nil {
		return err
	}
	if keywordDefaults {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionKeywordDefaults),
			statement.Span(),
		); err != nil {
			return err
		}
	}
	if defaults {
		if err := compiler.emit(
			bytecode.SetFunctionAttribute,
			uint32(bytecode.FunctionDefaults),
			statement.Span(),
		); err != nil {
			return err
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
	return compiler.emitTerminator(bytecode.ReturnValue, 0, statement.Span())
}

func (compiler *compilerState) addLocal(name string) {
	if _, exists := compiler.localIDs[name]; exists {
		return
	}
	compiler.localIDs[name] = uint32(len(compiler.locals))
	compiler.locals = append(compiler.locals, name)
}
