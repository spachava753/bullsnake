package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

const (
	contextEnterName = "__enter__"
	contextExitName  = "__exit__"
)

func (compiler *compilerState) compileWithStatement(statement *compilerast.WithStmt) error {
	if statement.Async {
		return compiler.error(statement.Span(), "async with is not compiled")
	}
	if len(statement.Items) == 0 {
		return compiler.error(statement.Span(), "with statement has no context managers")
	}
	return compiler.compileWithItem(statement, 0)
}

// compileWithItem nests each comma-separated manager so later entry and exit
// operations remain protected by every manager that has already entered.
func (compiler *compilerState) compileWithItem(
	statement *compilerast.WithStmt,
	index int,
) error {
	item := statement.Items[index]
	baseDepth := compiler.stackDepth
	if err := compiler.compileContextEntry(item); err != nil {
		return err
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
		context: contextManagerCleanup{
			baseDepth: baseDepth,
			span:      item.Range,
		},
	})
	var err error
	if item.Target == nil {
		err = compiler.emit(bytecode.PopTop, 0, item.Range)
	} else {
		err = compiler.compileStore(item.Target)
	}
	if err == nil {
		if index+1 < len(statement.Items) {
			err = compiler.compileWithItem(statement, index+1)
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
		if err := compiler.emitContextExit(item.Range); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.Jump, end, statement.Span()); err != nil {
			return err
		}
	}
	if err := compiler.compileExceptionalContextExit(handler, end, baseDepth, item.Range); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}

// compileContextEntry leaves the bound exit method below the enter result. The
// protected range starts only after __enter__ returns successfully.
func (compiler *compilerState) compileContextEntry(item compilerast.WithItem) error {
	if err := compiler.compileExpr(item.Context); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, item.Context.Span()); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadSpecial,
		compiler.nameIndex(contextExitName),
		item.Context.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Swap, 2, item.Context.Span()); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadSpecial,
		compiler.nameIndex(contextEnterName),
		item.Context.Span(),
	); err != nil {
		return err
	}
	return compiler.emit(bytecode.Call, 0, item.Context.Span())
}

func (compiler *compilerState) emitContextExit(span lexer.Span) error {
	if err := compiler.emitContextExitArguments(span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 3, span); err != nil {
		return err
	}
	return compiler.emit(bytecode.PopTop, 0, span)
}

// emitPreservedContextExit calls a manager during return or loop transfer while
// leaving the original exit method for the transfer's stack unwinding.
func (compiler *compilerState) emitPreservedContextExit(
	cleanup contextManagerCleanup,
) error {
	depth := compiler.stackDepth - cleanup.baseDepth
	if depth < 1 {
		return compiler.error(cleanup.span, "context exit is missing from the operand stack")
	}
	if err := compiler.emit(bytecode.Copy, uint32(depth), cleanup.span); err != nil {
		return err
	}
	return compiler.emitContextExit(cleanup.span)
}

func (compiler *compilerState) emitContextExitArguments(span lexer.Span) error {
	for range 3 {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.None()),
			span,
		); err != nil {
			return err
		}
	}
	return nil
}

// compileExceptionalContextExit activates the pending exception, calls
// __exit__(type, value, None), and either suppresses or reraises it.
func (compiler *compilerState) compileExceptionalContextExit(
	handler *jumpLabel,
	end *jumpLabel,
	baseDepth int,
	span lexer.Span,
) error {
	if err := compiler.mergeLabelDepth(handler, baseDepth+2, span); err != nil {
		return err
	}
	if baseDepth+2 > compiler.maxStack {
		compiler.maxStack = baseDepth + 2
	}
	if err := compiler.markLabel(handler, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 1, span); err != nil {
		return err
	}
	if err := compiler.emitLabelOperand(bytecode.EnterExcept, end, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadHandledExceptionType, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Copy, 3, span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.None()),
		span,
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 3, span); err != nil {
		return err
	}

	suppressed := compiler.newLabel()
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, suppressed, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LeaveExcept, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Swap, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
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
	return compiler.emit(bytecode.PopTop, 0, span)
}
