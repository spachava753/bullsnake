package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

// compileAwaitExpression delegates to one native coroutine and leaves its
// return value as the expression result.
func (compiler *compilerState) compileAwaitExpression(expression *compilerast.AwaitExpr) error {
	if compiler.scope.Kind != resolver.FunctionScope ||
		compiler.scope.Flags&resolver.Coroutine == 0 {
		return compiler.error(expression.Span(), "await has no enclosing coroutine")
	}
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	return compiler.compileAwaitStackTop(
		expression.Span(),
		bytecode.AwaitExpression,
	)
}

// compileAwaitStackTop converts the top value to an awaitable and emits the
// shared send/yield loop, leaving the awaitable's return value on the stack.
func (compiler *compilerState) compileAwaitStackTop(span lexer.Span, context uint32) error {
	if err := compiler.emit(bytecode.GetAwaitable, context, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.None()),
		span,
	); err != nil {
		return err
	}

	send := compiler.newLabel()
	exit := compiler.newLabel()
	if err := compiler.markLabel(send, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Send, exit, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.YieldValue, 0, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Jump, send, span); err != nil {
		return err
	}
	return compiler.markLabel(exit, span)
}
