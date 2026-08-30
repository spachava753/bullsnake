package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileYieldExpression suspends a generator with one value and leaves the
// value supplied by its next resumption as the expression result.
func (compiler *compilerState) compileYieldExpression(expression *compilerast.YieldExpr) error {
	if compiler.scope.Kind != resolver.FunctionScope ||
		compiler.scope.Flags&resolver.Generator == 0 {
		return compiler.error(expression.Span(), "yield has no enclosing generator")
	}
	if expression.From {
		return compiler.error(expression.Span(), "yield from is not compiled")
	}
	if expression.Value == nil {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			expression.Span(),
		); err != nil {
			return err
		}
	} else if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	return compiler.emit(bytecode.YieldValue, 0, expression.Span())
}
