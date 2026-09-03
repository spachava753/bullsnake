package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func (compiler *compilerState) compileFormattedString(expression *compilerast.FormattedStringExpr) error {
	if expression.Template {
		if err := compiler.emit(
			bytecode.LoadGlobal,
			compiler.nameIndex("__bullsnake_template__"),
			expression.Span(),
		); err != nil {
			return err
		}
		if err := compiler.compileFormattedParts(expression.Parts, expression.Raw, expression.Span()); err != nil {
			return err
		}
		return compiler.emit(bytecode.Call, 1, expression.Span())
	}
	return compiler.compileFormattedParts(expression.Parts, expression.Raw, expression.Span())
}

// compileFormattedParts leaves one joined string on the stack after compiling
// literal segments and replacement fields in source order.
func (compiler *compilerState) compileFormattedParts(parts []compilerast.Expr, raw bool, span lexer.Span) error {
	componentCount := 0
	for _, part := range parts {
		switch part := part.(type) {
		case *compilerast.StringLiteral:
			text, err := decodeStringText(part.Text, raw, false)
			if err != nil {
				return compiler.error(part.Span(), "%v", err)
			}
			if text == "" {
				continue
			}
			if err := compiler.emit(
				bytecode.LoadConst,
				compiler.constantIndex(bytecode.TextString(text)),
				part.Span(),
			); err != nil {
				return err
			}
			componentCount++
		case *compilerast.FormattedValueExpr:
			count, err := compiler.compileFormattedValue(part, raw)
			if err != nil {
				return err
			}
			componentCount += count
		default:
			return compiler.unsupported(part)
		}
	}
	if componentCount == 0 {
		return compiler.emit(bytecode.LoadConst, compiler.constantIndex(bytecode.TextString("")), span)
	}
	if componentCount == 1 {
		return nil
	}
	return compiler.emit(bytecode.BuildString, uint32(componentCount), span)
}

// compileFormattedValue emits an optional debug prefix and one formatted
// replacement. The returned count is the number of outer string components.
func (compiler *compilerState) compileFormattedValue(expression *compilerast.FormattedValueExpr, raw bool) (int, error) {
	componentCount := 1
	if expression.Debug {
		debugText, err := decodeStringText(expression.DebugText, true, false)
		if err != nil {
			return 0, compiler.error(expression.Span(), "%v", err)
		}
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.TextString(debugText)),
			expression.Span(),
		); err != nil {
			return 0, err
		}
		componentCount++
	}
	if err := compiler.compileExpr(expression.Value); err != nil {
		return 0, err
	}
	conversion := expression.Conversion
	if conversion == "" && expression.Debug && expression.Format == nil {
		conversion = "r"
	}
	if conversion != "" {
		operand, ok := formattedConversion(conversion)
		if !ok {
			return 0, compiler.error(expression.Span(), "unknown formatted string conversion %q", conversion)
		}
		if err := compiler.emit(bytecode.ConvertValue, operand, expression.Span()); err != nil {
			return 0, err
		}
	}
	if expression.Format == nil {
		if err := compiler.emit(bytecode.FormatSimple, 0, expression.Span()); err != nil {
			return 0, err
		}
		return componentCount, nil
	}
	if err := compiler.compileFormattedParts(expression.Format, raw, expression.Span()); err != nil {
		return 0, err
	}
	if err := compiler.emit(bytecode.FormatWithSpec, 0, expression.Span()); err != nil {
		return 0, err
	}
	return componentCount, nil
}

func formattedConversion(conversion string) (uint32, bool) {
	switch conversion {
	case "s":
		return bytecode.ConversionString, true
	case "r":
		return bytecode.ConversionRepr, true
	case "a":
		return bytecode.ConversionASCII, true
	default:
		return 0, false
	}
}
