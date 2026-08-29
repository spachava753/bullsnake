package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// compileIterableDisplay uses one count-based build when every element is
// ordinary. Starred displays use a mutable accumulator in source order.
func (compiler *compilerState) compileIterableDisplay(
	elements []compilerast.Expr,
	directBuild, unpackBuild, add, extend bytecode.Opcode,
	finishTuple bool,
	span lexer.Span,
) error {
	hasStarred := false
	for _, element := range elements {
		if _, ok := element.(*compilerast.StarredExpr); ok {
			hasStarred = true
			break
		}
	}
	if !hasStarred {
		for _, element := range elements {
			if err := compiler.compileExpr(element); err != nil {
				return err
			}
		}
		return compiler.emit(directBuild, uint32(len(elements)), span)
	}

	if err := compiler.emit(unpackBuild, 0, span); err != nil {
		return err
	}
	for _, element := range elements {
		if starred, ok := element.(*compilerast.StarredExpr); ok {
			if starred.Context != compilerast.Load {
				return compiler.error(starred.Span(), "starred display expression is not a load")
			}
			if err := compiler.compileExpr(starred.Value); err != nil {
				return err
			}
			if err := compiler.emit(extend, 0, starred.Span()); err != nil {
				return err
			}
			continue
		}
		if err := compiler.compileExpr(element); err != nil {
			return err
		}
		if err := compiler.emit(add, 0, element.Span()); err != nil {
			return err
		}
	}
	if finishTuple {
		return compiler.emit(bytecode.ListToTuple, 0, span)
	}
	return nil
}

// compileDictDisplay builds key/value pairs directly unless a dictionary
// unpack requires a mutable map accumulator.
func (compiler *compilerState) compileDictDisplay(expression *compilerast.DictExpr) error {
	if len(expression.Keys) != len(expression.Values) {
		return compiler.error(expression.Span(), "dictionary key/value count mismatch")
	}
	hasUnpack := false
	for _, key := range expression.Keys {
		if key == nil {
			hasUnpack = true
			break
		}
	}
	if !hasUnpack {
		for index, key := range expression.Keys {
			if err := compiler.compileExpr(key); err != nil {
				return err
			}
			if err := compiler.compileExpr(expression.Values[index]); err != nil {
				return err
			}
		}
		return compiler.emit(bytecode.BuildMap, uint32(len(expression.Keys)), expression.Span())
	}

	if err := compiler.emit(bytecode.BuildMap, 0, expression.Span()); err != nil {
		return err
	}
	for index, key := range expression.Keys {
		value := expression.Values[index]
		if key == nil {
			if err := compiler.compileExpr(value); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.MapUpdate, 0, value.Span()); err != nil {
				return err
			}
			continue
		}
		if err := compiler.compileExpr(key); err != nil {
			return err
		}
		if err := compiler.compileExpr(value); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.MapSet, 0, value.Span()); err != nil {
			return err
		}
	}
	return nil
}
