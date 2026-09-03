package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// compileWithItem nests each manager around the remaining items and suite,
// retaining the entered manager below protected body state until exit.
func (compiler *compilerState) compileWithItem(
	statement *compilerast.WithStmt,
	itemIndex int,
) error {
	item := statement.Items[itemIndex]
	baseDepth := compiler.stackDepth
	if err := compiler.compileExpr(item.Context); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, item.Range); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadAttr,
		compiler.nameIndex(withMethodName(statement.Async, "__enter__", "__aenter__")),
		item.Range,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 0, item.Range); err != nil {
		return err
	}
	if statement.Async {
		if err := compiler.emit(bytecode.AwaitValue, 0, item.Range); err != nil {
			return err
		}
	}

	handler := compiler.newLabel()
	end := compiler.newLabel()
	handlerDepth := len(compiler.activeHandlers)
	compiler.activeHandlers = append(compiler.activeHandlers, instructionExceptionHandler{
		target:     handler,
		stackDepth: baseDepth + 1,
	})
	compiler.controlCleanups = append(compiler.controlCleanups, controlCleanup{
		kind:         withControlCleanup,
		handlerDepth: handlerDepth,
		managerDepth: baseDepth,
		async:        statement.Async,
	})
	var err error
	if item.Target == nil {
		err = compiler.emit(bytecode.PopTop, 0, item.Range)
	} else {
		err = compiler.compileStore(item.Target)
	}
	if err == nil {
		if itemIndex+1 < len(statement.Items) {
			err = compiler.compileWithItem(statement, itemIndex+1)
		} else {
			err = compiler.compileStatements(statement.Body)
		}
	}
	compiler.controlCleanups = compiler.controlCleanups[:len(compiler.controlCleanups)-1]
	compiler.activeHandlers = compiler.activeHandlers[:len(compiler.activeHandlers)-1]
	if err != nil {
		return err
	}
	if compiler.reachable {
		if err := compiler.emitNormalWithExit(baseDepth, statement.Async, item.Range); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.Jump, end, item.Range); err != nil {
			return err
		}
	}

	if err := compiler.mergeLabelDepth(handler, baseDepth+2, item.Range); err != nil {
		return err
	}
	if baseDepth+2 > compiler.maxStack {
		compiler.maxStack = baseDepth + 2
	}
	if err := compiler.markLabel(handler, item.Range); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, item.Range); err != nil {
		return err
	}
	if err := compiler.emitLabelOperand(bytecode.EnterExcept, end, item.Range); err != nil {
		return err
	}
	if err := compiler.emitExceptionalWithExit(end, baseDepth, statement.Async, item.Range); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}

// emitNormalWithExit calls __exit__(None, None, None), discards its result,
// and removes the retained manager while preserving newer stack values.
func (compiler *compilerState) emitNormalWithExit(managerDepth int, async bool, span lexer.Span) error {
	depth := compiler.stackDepth - managerDepth
	if depth < 1 {
		return compiler.error(span, "context manager is no longer on the operand stack")
	}
	if err := compiler.emit(bytecode.Copy, uint32(depth), span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadAttr,
		compiler.nameIndex(withMethodName(async, "__exit__", "__aexit__")),
		span,
	); err != nil {
		return err
	}
	for range 3 {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			span,
		); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.Call, 3, span); err != nil {
		return err
	}
	if async {
		if err := compiler.emit(bytecode.AwaitValue, 0, span); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
		return err
	}
	return compiler.removeWithManager(managerDepth, span)
}

// emitExceptionalWithExit calls __exit__ with the active exception, then
// either removes retained state on suppression or reraises the same exception.
func (compiler *compilerState) emitExceptionalWithExit(
	end *jumpLabel,
	managerDepth int,
	async bool,
	span lexer.Span,
) error {
	suppressed := compiler.newLabel()
	if err := compiler.emit(bytecode.Copy, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadAttr,
		compiler.nameIndex(withMethodName(async, "__exit__", "__aexit__")),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.ExceptionType, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 3, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadAttr,
		compiler.nameIndex("__traceback__"),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 3, span); err != nil {
		return err
	}
	if async {
		if err := compiler.emit(bytecode.AwaitValue, 0, span); err != nil {
			return err
		}
	}
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, suppressed, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LeaveExcept, 0, span); err != nil {
		return err
	}
	if err := compiler.removeWithManager(managerDepth, span); err != nil {
		return err
	}
	if err := compiler.emitTerminator(bytecode.Reraise, 0, span); err != nil {
		return err
	}
	if err := compiler.markLabel(suppressed, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LeaveExcept, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
		return err
	}
	if err := compiler.removeWithManager(managerDepth, span); err != nil {
		return err
	}
	return compiler.emitJump(bytecode.Jump, end, span)
}

func withMethodName(async bool, synchronous, asynchronous string) string {
	if async {
		return asynchronous
	}
	return synchronous
}

func (compiler *compilerState) removeWithManager(managerDepth int, span lexer.Span) error {
	valuesAbove := compiler.stackDepth - managerDepth - 1
	if valuesAbove < 0 {
		return compiler.error(span, "context manager is no longer on the operand stack")
	}
	for depth := 2; depth <= valuesAbove+1; depth++ {
		if err := compiler.emit(bytecode.Swap, uint32(depth), span); err != nil {
			return err
		}
	}
	return compiler.emit(bytecode.PopTop, 0, span)
}
