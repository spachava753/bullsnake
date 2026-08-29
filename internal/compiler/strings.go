package compiler

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"golang.org/x/text/unicode/runenames"
)

var unicodeNameCache sync.Map

// parseStringLiteral evaluates one plain Python string or bytes token.
func parseStringLiteral(text string) (bytecode.Constant, error) {
	body, raw, bytesLiteral, err := splitStringLiteral(text)
	if err != nil {
		return bytecode.Constant{}, err
	}
	body, err = decodeStringText(body, raw, bytesLiteral)
	if err != nil {
		return bytecode.Constant{}, err
	}
	if bytesLiteral {
		return bytecode.Bytes(body), nil
	}
	return bytecode.TextString(body), nil
}

func decodeStringText(text string, raw, bytesLiteral bool) (string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if bytesLiteral {
		for _, value := range []byte(text) {
			if value >= utf8.RuneSelf {
				return "", fmt.Errorf("bytes can only contain ASCII literal characters")
			}
		}
	}
	if raw {
		return text, nil
	}
	decoder := stringDecoder{source: text, bytesLiteral: bytesLiteral}
	return decoder.decode()
}

// splitStringLiteral separates a lexer token into its prefix and body, records
// raw and bytes modes, and verifies its single or triple quote delimiter.
func splitStringLiteral(text string) (body string, raw, bytesLiteral bool, err error) {
	quoteIndex := strings.IndexAny(text, "'\"")
	if quoteIndex < 0 {
		return "", false, false, fmt.Errorf("string literal has no opening quote")
	}
	prefix := strings.ToLower(text[:quoteIndex])
	for _, character := range prefix {
		switch character {
		case 'r':
			raw = true
		case 'b':
			bytesLiteral = true
		case 'u':
		default:
			return "", false, false, fmt.Errorf("unsupported string prefix %q", text[:quoteIndex])
		}
	}
	quote := text[quoteIndex : quoteIndex+1]
	quoteSize := 1
	if strings.HasPrefix(text[quoteIndex:], quote+quote+quote) {
		quoteSize = 3
	}
	if len(text) < quoteIndex+2*quoteSize || !strings.HasSuffix(text, strings.Repeat(quote, quoteSize)) {
		return "", false, false, fmt.Errorf("string literal has no matching closing quote")
	}
	return text[quoteIndex+quoteSize : len(text)-quoteSize], raw, bytesLiteral, nil
}

type stringDecoder struct {
	source       string
	index        int
	bytesLiteral bool
	output       strings.Builder
}

func (decoder *stringDecoder) decode() (string, error) {
	for decoder.index < len(decoder.source) {
		if decoder.source[decoder.index] != '\\' {
			decoder.output.WriteByte(decoder.source[decoder.index])
			decoder.index++
			continue
		}
		if err := decoder.decodeEscape(); err != nil {
			return "", err
		}
	}
	return decoder.output.String(), nil
}

// decodeEscape consumes one backslash escape and writes its decoded value,
// preserving unknown escapes for Python's warning layer to handle later.
func (decoder *stringDecoder) decodeEscape() error {
	decoder.index++
	if decoder.index == len(decoder.source) {
		return fmt.Errorf("trailing backslash in string literal")
	}
	escape := decoder.source[decoder.index]
	decoder.index++
	if escape == '\n' {
		return nil
	}
	if value, ok := simpleEscape(escape); ok {
		decoder.appendCodePoint(value)
		return nil
	}
	if escape >= '0' && escape <= '7' {
		return decoder.decodeOctal(escape)
	}
	switch escape {
	case 'x':
		return decoder.decodeHex(2, "\\x")
	case 'u':
		if !decoder.bytesLiteral {
			return decoder.decodeHex(4, "\\u")
		}
	case 'U':
		if !decoder.bytesLiteral {
			return decoder.decodeHex(8, "\\U")
		}
	case 'N':
		if !decoder.bytesLiteral {
			return decoder.decodeNamedUnicode()
		}
	}
	decoder.output.WriteByte('\\')
	decoder.output.WriteByte(escape)
	return nil
}

// simpleEscape maps Python's one-character escape spellings to their values.
func simpleEscape(escape byte) (int64, bool) {
	switch escape {
	case '\\', '\'', '"':
		return int64(escape), true
	case 'a':
		return 0x07, true
	case 'b':
		return 0x08, true
	case 'f':
		return 0x0c, true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return 0x0b, true
	default:
		return 0, false
	}
}

// decodeOctal consumes at most three octal digits. Bytes follow CPython's
// low-byte behavior for values above 255; Unicode strings retain the rune.
func (decoder *stringDecoder) decodeOctal(first byte) error {
	value := int64(first - '0')
	for count := 1; count < 3 && decoder.index < len(decoder.source); count++ {
		digit := decoder.source[decoder.index]
		if digit < '0' || digit > '7' {
			break
		}
		value = value*8 + int64(digit-'0')
		decoder.index++
	}
	if decoder.bytesLiteral {
		value &= 0xff
	}
	decoder.appendCodePoint(value)
	return nil
}

func (decoder *stringDecoder) decodeHex(digits int, escape string) error {
	if len(decoder.source)-decoder.index < digits {
		return fmt.Errorf("truncated %s escape", escape)
	}
	text := decoder.source[decoder.index : decoder.index+digits]
	value, err := strconv.ParseInt(text, 16, 32)
	if err != nil {
		return fmt.Errorf("invalid %s escape", escape)
	}
	decoder.index += digits
	if value > unicode.MaxRune {
		return fmt.Errorf("%s escape is outside the Unicode range", escape)
	}
	decoder.appendCodePoint(value)
	return nil
}

func (decoder *stringDecoder) decodeNamedUnicode() error {
	if decoder.index == len(decoder.source) || decoder.source[decoder.index] != '{' {
		return fmt.Errorf("malformed \\N character escape")
	}
	nameStart := decoder.index + 1
	nameEnd := strings.IndexByte(decoder.source[nameStart:], '}')
	if nameEnd < 0 {
		return fmt.Errorf("malformed \\N character escape")
	}
	nameEnd += nameStart
	name := decoder.source[nameStart:nameEnd]
	character, ok := lookupUnicodeName(name)
	if !ok {
		return fmt.Errorf("unknown Unicode character name %q", name)
	}
	decoder.index = nameEnd + 1
	decoder.appendCodePoint(int64(character))
	return nil
}

func (decoder *stringDecoder) appendCodePoint(value int64) {
	if decoder.bytesLiteral {
		decoder.output.WriteByte(byte(value))
		return
	}
	if value >= 0xd800 && value <= 0xdfff {
		decoder.output.WriteByte(byte(0xe0 | value>>12))
		decoder.output.WriteByte(byte(0x80 | value>>6&0x3f))
		decoder.output.WriteByte(byte(0x80 | value&0x3f))
		return
	}
	decoder.output.WriteRune(rune(value))
}

func lookupUnicodeName(name string) (rune, bool) {
	name = strings.ToUpper(name)
	if cached, ok := unicodeNameCache.Load(name); ok {
		character := cached.(rune)
		return character, character >= 0
	}
	for character := rune(0); character <= unicode.MaxRune; character++ {
		if runenames.Name(character) == name {
			unicodeNameCache.Store(name, character)
			return character, true
		}
	}
	unicodeNameCache.Store(name, rune(-1))
	return 0, false
}

// compileStringConcat folds adjacent plain literals or joins plain and
// formatted components while rejecting Python's bytes/text mixture.
func (compiler *compilerState) compileStringConcat(expression *compilerast.StringConcatExpr) error {
	allPlain := true
	for _, part := range expression.Parts {
		if _, ok := part.(*compilerast.StringLiteral); !ok {
			allPlain = false
			break
		}
	}
	if allPlain {
		var output strings.Builder
		kind := bytecode.NoneConstant
		for _, part := range expression.Parts {
			literal := part.(*compilerast.StringLiteral)
			constant, err := parseStringLiteral(literal.Text)
			if err != nil {
				return compiler.error(literal.Span(), "%v", err)
			}
			if kind == bytecode.NoneConstant {
				kind = constant.Kind
			} else if constant.Kind != kind {
				return compiler.error(literal.Span(), "cannot mix bytes and nonbytes literals")
			}
			output.WriteString(constant.Text)
		}
		constant := bytecode.TextString(output.String())
		if kind == bytecode.BytesConstant {
			constant = bytecode.Bytes(output.String())
		}
		return compiler.emit(bytecode.LoadConst, compiler.constantIndex(constant), expression.Span())
	}

	componentCount := 0
	for _, part := range expression.Parts {
		switch part := part.(type) {
		case *compilerast.StringLiteral:
			constant, err := parseStringLiteral(part.Text)
			if err != nil {
				return compiler.error(part.Span(), "%v", err)
			}
			if constant.Kind == bytecode.BytesConstant {
				return compiler.error(part.Span(), "cannot mix bytes and nonbytes literals")
			}
			if constant.Text == "" {
				continue
			}
			if err := compiler.emit(bytecode.LoadConst, compiler.constantIndex(constant), part.Span()); err != nil {
				return err
			}
			componentCount++
		case *compilerast.FormattedStringExpr:
			if err := compiler.compileFormattedString(part); err != nil {
				return err
			}
			componentCount++
		default:
			return compiler.unsupported(part)
		}
	}
	if componentCount == 0 {
		return compiler.emit(bytecode.LoadConst, compiler.constantIndex(bytecode.TextString("")), expression.Span())
	}
	if componentCount == 1 {
		return nil
	}
	return compiler.emit(bytecode.BuildString, uint32(componentCount), expression.Span())
}
