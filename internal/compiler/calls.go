package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// compileCall uses inline positional operands for the common case and builds
// tuple/map containers when unpacking or named arguments require them.
func (compiler *compilerState) compileCall(expression *compilerast.CallExpr) error {
	if err := compiler.compileExpr(expression.Function); err != nil {
		return err
	}

	hasStarred := false
	for _, argument := range expression.Arguments {
		if _, ok := argument.(*compilerast.StarredExpr); ok {
			hasStarred = true
			break
		}
	}
	if !hasStarred && len(expression.Keywords) == 0 {
		for _, argument := range expression.Arguments {
			if err := compiler.compileExpr(argument); err != nil {
				return err
			}
		}
		return compiler.emit(bytecode.Call, uint32(len(expression.Arguments)), expression.Span())
	}

	if err := compiler.compileIterableDisplay(
		expression.Arguments,
		bytecode.BuildTuple,
		bytecode.BuildList,
		bytecode.ListAppend,
		bytecode.ListExtend,
		true,
		expression.Span(),
	); err != nil {
		return err
	}
	if len(expression.Keywords) == 0 {
		return compiler.emit(bytecode.CallEx, bytecode.CallExNoKeywords, expression.Span())
	}

	if err := compiler.compileKeywordArguments(expression.Keywords, expression.Span()); err != nil {
		return err
	}
	return compiler.emit(bytecode.CallEx, bytecode.CallExWithKeywords, expression.Span())
}

// compileKeywordArguments builds one map in source order. MAP_MERGE preserves
// Python's duplicate-name failure for both named and dictionary-unpacked items.
func (compiler *compilerState) compileKeywordArguments(
	keywords []compilerast.KeywordArgument,
	span lexer.Span,
) error {
	if err := compiler.emit(bytecode.BuildMap, 0, span); err != nil {
		return err
	}
	for _, keyword := range keywords {
		if keyword.Name == "" {
			if err := compiler.compileExpr(keyword.Value); err != nil {
				return err
			}
		} else {
			if err := compiler.emit(
				bytecode.LoadConst,
				compiler.constantIndex(
					bytecode.TextString(compiler.scope.Mangle(keyword.Name)),
				),
				keyword.Range,
			); err != nil {
				return err
			}
			if err := compiler.compileExpr(keyword.Value); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.BuildMap, 1, keyword.Range); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.MapMerge, 0, keyword.Range); err != nil {
			return err
		}
	}
	return nil
}
