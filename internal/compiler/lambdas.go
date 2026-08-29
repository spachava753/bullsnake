package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileLambdaExpression evaluates defaults in the enclosing scope, compiles
// the expression body in one child, and leaves the created function as its value.
func (compiler *compilerState) compileLambdaExpression(expression *compilerast.LambdaExpr) error {
	scope := compiler.table.ScopeFor(expression, resolver.DefinitionBody, 0)
	if scope == nil || scope.Kind != resolver.FunctionScope {
		return compiler.error(expression.Span(), "resolver has no function scope for lambda")
	}
	defaults, keywordDefaults, err := compiler.compileFunctionDefaults(
		expression.Parameters,
		expression.Span(),
	)
	if err != nil {
		return err
	}
	child := compiler.newFunctionCompiler(expression, scope, expression.Parameters, "<lambda>")
	if err := child.compileExpr(expression.Body); err != nil {
		return err
	}
	if err := child.emitTerminator(bytecode.ReturnValue, 0, expression.Body.Span()); err != nil {
		return err
	}
	code, err := child.finish()
	if err != nil {
		return err
	}
	return compiler.emitFunction(code, defaults, keywordDefaults, expression.Span())
}
