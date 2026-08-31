package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectFunction binds the definition, resolves defaults and decorators in
// the parent, then enters optional type-parameter and function scopes.
func (state *resolver) collectFunction(statement *compilerast.FunctionDefStmt) error {
	if _, err := state.bind(statement.Name, Assigned, statement.Span()); err != nil {
		return err
	}
	if err := state.collectParameterDefaults(statement.Parameters); err != nil {
		return err
	}
	for _, decorator := range statement.Decorators {
		if err := state.collectExpr(decorator); err != nil {
			return err
		}
	}

	method := state.current.Kind == ClassScope
	canSeeClass := method || state.current.Flags&CanSeeClassScope != 0
	if len(statement.TypeParameters) == 0 {
		return state.collectFunctionScopes(statement, method, canSeeClass)
	}
	return state.inTypeParameters(
		statement, statement.Name, statement.TypeParameters,
		state.current.PrivateName, canSeeClass,
		func() error {
			return state.collectFunctionScopes(statement, method, canSeeClass)
		},
	)
}

// collectFunctionScopes creates sibling annotation and body scopes after the
// caller has entered any enclosing type-parameter scope.
func (state *resolver) collectFunctionScopes(
	statement *compilerast.FunctionDefStmt,
	method bool,
	canSeeClass bool,
) error {
	annotationFlags := ScopeFlags(0)
	if statement.Returns != nil || parametersHaveAnnotations(statement.Parameters) {
		annotationFlags |= UsesAnnotations
		if state.table.Features&FutureAnnotations != 0 {
			annotationFlags |= UnevaluatedAnnotations
		} else if canSeeClass {
			annotationFlags |= CanSeeClassScope
			state.markVisibleClassNeedsDict()
		}
	}
	annotationScope := state.childScope(
		statement, Annotations, 0, AnnotationScope, "__annotate__",
		state.current.PrivateName, annotationFlags,
	)
	if err := state.inScope(annotationScope, func() error {
		if err := state.collectParameterAnnotations(statement.Parameters); err != nil {
			return err
		}
		return state.collectExpr(statement.Returns)
	}); err != nil {
		return err
	}

	functionFlags := ScopeFlags(0)
	if method {
		functionFlags |= Method
	}
	if statement.Async {
		functionFlags |= AsyncFunction | Coroutine
	}
	if statement.Parameters.VarArg != nil {
		functionFlags |= VarArgs
	}
	if statement.Parameters.KeywordVarArg != nil {
		functionFlags |= VarKeywords
	}
	functionScope := state.childScope(
		statement, DefinitionBody, 0, FunctionScope, statement.Name,
		state.current.PrivateName, functionFlags,
	)
	return state.inScope(functionScope, func() error {
		if err := state.collectParameters(statement.Parameters); err != nil {
			return err
		}
		if err := state.collectStatements(statement.Body); err != nil {
			return err
		}
		if functionScope.Flags&AsyncFunction != 0 && functionScope.Flags&Generator != 0 && functionScope.Flags&ReturnsValue != 0 {
			return state.syntaxError(functionScope.returnValueSpan, "return with value in async generator")
		}
		return nil
	})
}

// collectParameterDefaults visits positional and keyword-only defaults in the
// definition's surrounding scope.
func (state *resolver) collectParameterDefaults(parameters compilerast.Parameters) error {
	for _, parameter := range parameters.PositionalOnly {
		if err := state.collectExpr(parameter.Default); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.Positional {
		if err := state.collectExpr(parameter.Default); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.KeywordOnly {
		if err := state.collectExpr(parameter.Default); err != nil {
			return err
		}
	}
	return nil
}

// collectParameterAnnotations visits each present parameter annotation inside
// the function's annotation scope.
func (state *resolver) collectParameterAnnotations(parameters compilerast.Parameters) error {
	for _, parameter := range parameters.PositionalOnly {
		if err := state.collectExpr(parameter.Annotation); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.Positional {
		if err := state.collectExpr(parameter.Annotation); err != nil {
			return err
		}
	}
	if parameters.VarArg != nil {
		if err := state.collectExpr(parameters.VarArg.Annotation); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.KeywordOnly {
		if err := state.collectExpr(parameter.Annotation); err != nil {
			return err
		}
	}
	if parameters.KeywordVarArg != nil {
		return state.collectExpr(parameters.KeywordVarArg.Annotation)
	}
	return nil
}

// collectParameters records parameter bindings and their callable-order names
// in the function or lambda body scope.
func (state *resolver) collectParameters(parameters compilerast.Parameters) error {
	for _, parameter := range parameters.PositionalOnly {
		if err := state.collectParameter(parameter); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.Positional {
		if err := state.collectParameter(parameter); err != nil {
			return err
		}
	}
	if parameters.VarArg != nil {
		if err := state.collectParameter(*parameters.VarArg); err != nil {
			return err
		}
	}
	for _, parameter := range parameters.KeywordOnly {
		if err := state.collectParameter(parameter); err != nil {
			return err
		}
	}
	if parameters.KeywordVarArg != nil {
		if err := state.collectParameter(*parameters.KeywordVarArg); err != nil {
			return err
		}
	}
	return nil
}

func (state *resolver) collectParameter(parameter compilerast.Parameter) error {
	name, err := state.bind(parameter.Name, Parameter, parameter.Range)
	if err != nil {
		return err
	}
	state.current.Parameters = append(state.current.Parameters, name)
	return nil
}

// parametersHaveAnnotations reports whether a function needs annotation work.
func parametersHaveAnnotations(parameters compilerast.Parameters) bool {
	for _, parameter := range parameters.PositionalOnly {
		if parameter.Annotation != nil {
			return true
		}
	}
	for _, parameter := range parameters.Positional {
		if parameter.Annotation != nil {
			return true
		}
	}
	if parameters.VarArg != nil && parameters.VarArg.Annotation != nil {
		return true
	}
	for _, parameter := range parameters.KeywordOnly {
		if parameter.Annotation != nil {
			return true
		}
	}
	return parameters.KeywordVarArg != nil && parameters.KeywordVarArg.Annotation != nil
}
