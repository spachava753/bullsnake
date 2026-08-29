package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileNamedExpression(expression *compilerast.NamedExpr) error {
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, expression.Span()); err != nil {
		return err
	}
	return compiler.compileStore(expression.Target)
}
