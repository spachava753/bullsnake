package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func (compiler *compilerState) compileFormattedString(expression *compilerast.FormattedStringExpr) error {
	if expression.Template {
		return compiler.compileTemplateStrings([]*compilerast.FormattedStringExpr{expression})
	}
	return compiler.compileFormattedParts(expression.Parts, expression.Raw, expression.Span())
}

type templateInterpolation struct {
	value *compilerast.FormattedValueExpr
	raw   bool
}

// compileTemplateStrings builds the parallel string and interpolation tuples
// used by Python 3.14 template values.
func (compiler *compilerState) compileTemplateStrings(
	expressions []*compilerast.FormattedStringExpr,
) error {
	stringsValues := []string{""}
	var interpolations []templateInterpolation
	for _, expression := range expressions {
		for _, part := range expression.Parts {
			switch part := part.(type) {
			case *compilerast.StringLiteral:
				text, err := decodeStringText(part.Text, expression.Raw, false)
				if err != nil {
					return compiler.error(part.Span(), "%v", err)
				}
				stringsValues[len(stringsValues)-1] += text
			case *compilerast.FormattedValueExpr:
				if part.Debug {
					debugText, err := decodeStringText(part.DebugText, true, false)
					if err != nil {
						return compiler.error(part.Span(), "%v", err)
					}
					stringsValues[len(stringsValues)-1] += debugText
				}
				interpolations = append(interpolations, templateInterpolation{
					value: part,
					raw:   expression.Raw,
				})
				stringsValues = append(stringsValues, "")
			default:
				return compiler.unsupported(part)
			}
		}
	}
	span := expressions[0].Span()
	span.End = expressions[len(expressions)-1].Span().End
	for _, text := range stringsValues {
		if err := compiler.emit(
			bytecode.LoadConst,
			compiler.constantIndex(bytecode.TextString(text)),
			span,
		); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.BuildTuple, uint32(len(stringsValues)), span); err != nil {
		return err
	}
	for _, interpolation := range interpolations {
		if err := compiler.compileTemplateInterpolation(interpolation); err != nil {
			return err
		}
	}
	if err := compiler.emit(bytecode.BuildTuple, uint32(len(interpolations)), span); err != nil {
		return err
	}
	return compiler.emit(bytecode.BuildTemplate, 0, span)
}

// compileTemplateInterpolation evaluates one field, preserves its source text
// and conversion, and optionally computes its nested format-spec string.
func (compiler *compilerState) compileTemplateInterpolation(
	interpolation templateInterpolation,
) error {
	expression := interpolation.value
	if err := compiler.compileExpr(expression.Value); err != nil {
		return err
	}
	source, err := compiler.expressionSource(expression.Value)
	if err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.TextString(source)),
		expression.Span(),
	); err != nil {
		return err
	}
	conversion := expression.Conversion
	if conversion == "" && expression.Debug && expression.Format == nil {
		conversion = "r"
	}
	conversionOperand := uint32(0)
	if conversion != "" {
		var ok bool
		conversionOperand, ok = formattedConversion(conversion)
		if !ok {
			return compiler.error(
				expression.Span(),
				"unknown template string conversion %q",
				conversion,
			)
		}
	}
	if expression.Format != nil {
		if err := compiler.compileFormattedParts(
			expression.Format,
			interpolation.raw,
			expression.Span(),
		); err != nil {
			return err
		}
	}
	operand, ok := bytecode.PackInterpolationOperand(
		conversionOperand,
		expression.Format != nil,
	)
	if !ok {
		return compiler.error(expression.Span(), "invalid template interpolation metadata")
	}
	return compiler.emit(bytecode.BuildInterpolation, operand, expression.Span())
}

func (compiler *compilerState) expressionSource(expression compilerast.Expr) (string, error) {
	span := expression.Span()
	source := compiler.module.Source()
	if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset || span.End.Offset > len(source) {
		return "", compiler.error(span, "template interpolation span is outside source")
	}
	return source[span.Start.Offset:span.End.Offset], nil
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
