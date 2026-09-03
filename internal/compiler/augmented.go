package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileAugmentedAssignment retains a complex target's address across one
// in-place operation so object and index expressions run exactly once.
func (compiler *compilerState) compileAugmentedAssignment(statement *compilerast.AugAssignStmt) error {
	operand, ok := binaryOperand(statement.Op)
	if !ok {
		return compiler.error(statement.Span(), "unknown augmented operator %s", statement.Op)
	}

	switch target := statement.Target.(type) {
	case *compilerast.Name:
		if target.Context != compilerast.Store {
			return compiler.error(target.Span(), "augmented name target is not a store")
		}
		if err := compiler.emitNameLoad(target.ID, target.Span()); err != nil {
			return err
		}
	case *compilerast.AttributeExpr:
		if target.Context != compilerast.Store {
			return compiler.error(target.Span(), "augmented attribute target is not a store")
		}
		if err := compiler.compileExpr(target.Value); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Copy, 1, target.Span()); err != nil {
			return err
		}
		if err := compiler.emit(
			bytecode.LoadAttr,
			compiler.nameIndex(compiler.mangleName(target.Name)),
			target.Span(),
		); err != nil {
			return err
		}
	case *compilerast.SubscriptExpr:
		if target.Context != compilerast.Store {
			return compiler.error(target.Span(), "augmented subscript target is not a store")
		}
		if err := compiler.compileExpr(target.Value); err != nil {
			return err
		}
		if err := compiler.compileExpr(target.Index); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Copy, 2, target.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Copy, 2, target.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.BinarySubscript, 0, target.Span()); err != nil {
			return err
		}
	default:
		return compiler.error(statement.Target.Span(), "invalid augmented assignment target")
	}

	if err := compiler.compileExpr(statement.Value); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.InplaceOp, operand, statement.Span()); err != nil {
		return err
	}

	switch target := statement.Target.(type) {
	case *compilerast.Name:
		return compiler.emitNameStore(target.ID, target.Span())
	case *compilerast.AttributeExpr:
		if err := compiler.emit(bytecode.Swap, 2, target.Span()); err != nil {
			return err
		}
		return compiler.emit(
			bytecode.StoreAttr,
			compiler.nameIndex(compiler.mangleName(target.Name)),
			target.Span(),
		)
	case *compilerast.SubscriptExpr:
		if err := compiler.emit(bytecode.Swap, 3, target.Span()); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Swap, 2, target.Span()); err != nil {
			return err
		}
		return compiler.emit(bytecode.StoreSubscript, 0, target.Span())
	default:
		return compiler.error(statement.Target.Span(), "invalid augmented assignment target")
	}
}
