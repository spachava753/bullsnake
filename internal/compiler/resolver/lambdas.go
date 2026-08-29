package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectLambda resolves defaults outside and parameters and body inside a new
// function scope.
func (state *resolver) collectLambda(expression *compilerast.LambdaExpr) error {
	if err := state.collectParameterDefaults(expression.Parameters); err != nil {
		return err
	}
	flags := ScopeFlags(0)
	if state.current.Kind == ClassScope {
		flags |= Method
	}
	if expression.Parameters.VarArg != nil {
		flags |= VarArgs
	}
	if expression.Parameters.KeywordVarArg != nil {
		flags |= VarKeywords
	}
	scope := state.childScope(
		expression, DefinitionBody, 0, FunctionScope, "lambda",
		state.current.PrivateName, flags,
	)
	return state.inScope(scope, func() error {
		if err := state.collectParameters(expression.Parameters); err != nil {
			return err
		}
		return state.collectExpr(expression.Body)
	})
}
