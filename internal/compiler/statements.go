package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

func (compiler *compilerState) compileStatements(statements []compilerast.Stmt) error {
	for _, statement := range statements {
		if !compiler.reachable {
			break
		}
		if err := compiler.compileStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

// compileStatement emits one supported statement while preserving Python's
// value-before-target assignment order.
func (compiler *compilerState) compileStatement(statement compilerast.Stmt) error {
	switch statement := statement.(type) {
	case *compilerast.PassStmt:
		return compiler.emit(bytecode.Nop, 0, statement.Span())
	case *compilerast.ExprStmt:
		if err := compiler.compileExpr(statement.Value); err != nil {
			return err
		}
		return compiler.emit(bytecode.PopTop, 0, statement.Span())
	case *compilerast.AssignStmt:
		if err := compiler.compileExpr(statement.Value); err != nil {
			return err
		}
		for index, target := range statement.Targets {
			if index != len(statement.Targets)-1 {
				if err := compiler.emit(bytecode.Copy, 1, statement.Span()); err != nil {
					return err
				}
			}
			if err := compiler.compileStore(target); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.AnnAssignStmt:
		return compiler.compileAnnotatedAssignment(statement)
	case *compilerast.AugAssignStmt:
		return compiler.compileAugmentedAssignment(statement)
	case *compilerast.DeleteStmt:
		return compiler.compileDeleteStatement(statement)
	case *compilerast.RaiseStmt:
		return compiler.compileRaiseStatement(statement)
	case *compilerast.AssertStmt:
		return compiler.compileAssertStatement(statement)
	case *compilerast.ReturnStmt:
		return compiler.compileReturnStatement(statement)
	case *compilerast.GlobalStmt, *compilerast.NonlocalStmt:
		return nil
	case *compilerast.ImportStmt:
		return compiler.compileImportStatement(statement)
	case *compilerast.FromImportStmt:
		return compiler.compileFromImportStatement(statement)
	case *compilerast.FunctionDefStmt:
		return compiler.compileFunctionDefinition(statement)
	case *compilerast.ClassDefStmt:
		return compiler.compileClassDefinition(statement)
	case *compilerast.IfStmt:
		return compiler.compileIfStatement(statement)
	case *compilerast.WhileStmt:
		return compiler.compileWhileStatement(statement)
	case *compilerast.ForStmt:
		return compiler.compileForStatement(statement)
	case *compilerast.WithStmt:
		return compiler.compileWithStatement(statement)
	case *compilerast.TryStmt:
		if len(statement.Finally) != 0 {
			return compiler.compileTryFinally(statement)
		}
		return compiler.compileTryExcept(statement)
	case *compilerast.BreakStmt:
		if len(compiler.loops) == 0 {
			return compiler.error(statement.Span(), "break has no enclosing loop")
		}
		loop := compiler.loops[len(compiler.loops)-1]
		return compiler.compileLoopTransfer(
			loop,
			loop.breakLabel,
			loop.breakDepth,
			statement.Span(),
		)
	case *compilerast.ContinueStmt:
		if len(compiler.loops) == 0 {
			return compiler.error(statement.Span(), "continue has no enclosing loop")
		}
		loop := compiler.loops[len(compiler.loops)-1]
		return compiler.compileLoopTransfer(
			loop,
			loop.continueLabel,
			loop.continueDepth,
			statement.Span(),
		)
	default:
		return compiler.unsupported(statement)
	}
}
