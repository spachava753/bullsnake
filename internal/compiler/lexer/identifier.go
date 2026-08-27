package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type stringPrefix struct {
	raw       bool
	bytes     bool
	unicode   bool
	formatted bool
	template  bool
}

// scanIdentifierOrString distinguishes quoted and prefixed strings from names,
// then dispatches to the scanner for the recognized form.
func (lexer *Lexer) scanIdentifierOrString() (Token, error) {
	start := lexer.offset
	if lexer.source[start] == '\'' || lexer.source[start] == '"' {
		return lexer.scanPlainString(start, start, stringPrefix{})
	}

	prefix, quoteOffset, found := lexer.detectStringPrefix(start)
	if found {
		for _, pair := range incompatibleStringPrefixPairs {
			if prefix.enabled(pair[0]) && prefix.enabled(pair[1]) {
				span := lexer.span(start, quoteOffset)
				message := fmt.Sprintf("'%c' and '%c' prefixes are incompatible", pair[0], pair[1])
				return lexer.failToken(SyntaxError, message, span, false)
			}
		}
		if prefix.formatted || prefix.template {
			return lexer.scanStringStart(start, quoteOffset, prefix)
		}
		return lexer.scanPlainString(start, quoteOffset, prefix)
	}

	return lexer.scanIdentifier(start)
}

// scanIdentifier consumes an ASCII fast path, then validates every rune when
// the name contains non-ASCII source bytes.
func (lexer *Lexer) scanIdentifier(start int) (Token, error) {
	nonASCII := false
	for lexer.offset < len(lexer.source) {
		char := lexer.source[lexer.offset]
		if char < utf8.RuneSelf {
			if !isASCIIIdentifierContinue(char) {
				break
			}
			lexer.offset++
			continue
		}
		nonASCII = true
		_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
		lexer.offset += size
	}

	if !nonASCII {
		lexer.lineHasCode = true
		return lexer.token(Name, start, lexer.offset), nil
	}

	firstRune := true
	for offset := start; offset < lexer.offset; {
		r, size := utf8.DecodeRuneInString(lexer.source[offset:])
		valid := isXIDContinue(r)
		if firstRune {
			valid = isXIDStart(r)
		}
		if !valid {
			span := lexer.span(offset, offset+size)
			message := fmt.Sprintf("invalid character '%c' (U+%04X)", r, r)
			if !unicode.IsPrint(r) {
				message = fmt.Sprintf("invalid non-printable character U+%04X", r)
			}
			return lexer.failToken(SyntaxError, message, span, false)
		}
		firstRune = false
		offset += size
	}

	lexer.lineHasCode = true
	return lexer.token(Name, start, lexer.offset), nil
}

// detectStringPrefix collects distinct prefix flags through an opening quote;
// invalid candidates remain identifiers for normal tokenization.
func (lexer *Lexer) detectStringPrefix(start int) (stringPrefix, int, bool) {
	var prefix stringPrefix
	offset := start
	for offset < len(lexer.source) {
		if !prefix.accept(lexer.source[offset]) {
			return stringPrefix{}, 0, false
		}
		offset++
		if offset < len(lexer.source) && (lexer.source[offset] == '\'' || lexer.source[offset] == '"') {
			return prefix, offset, true
		}
	}
	return stringPrefix{}, 0, false
}

// accept records one case-insensitive prefix flag and rejects unknown or
// duplicate flags.
func (prefix *stringPrefix) accept(char byte) bool {
	var enabled *bool
	switch char {
	case 'b', 'B':
		enabled = &prefix.bytes
	case 'r', 'R':
		enabled = &prefix.raw
	case 'u', 'U':
		enabled = &prefix.unicode
	case 'f', 'F':
		enabled = &prefix.formatted
	case 't', 'T':
		enabled = &prefix.template
	default:
		return false
	}
	if *enabled {
		return false
	}
	*enabled = true
	return true
}

var incompatibleStringPrefixPairs = [...][2]byte{
	{'u', 'b'},
	{'u', 'r'},
	{'u', 'f'},
	{'u', 't'},
	{'b', 'f'},
	{'b', 't'},
	{'f', 't'},
}

// enabled reports whether char identifies a flag present in the prefix.
func (prefix stringPrefix) enabled(char byte) bool {
	switch char {
	case 'b':
		return prefix.bytes
	case 'r':
		return prefix.raw
	case 'u':
		return prefix.unicode
	case 'f':
		return prefix.formatted
	case 't':
		return prefix.template
	default:
		return false
	}
}

func isASCIIIdentifierStart(char byte) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func isASCIIIdentifierContinue(char byte) bool {
	return isASCIIIdentifierStart(char) || isASCIIDigit(char)
}

// isXIDStart applies NFKC identifier rules after removing code points added
// after the Unicode version used by Python 3.14.
func isXIDStart(r rune) bool {
	if inRuneRanges(r, unicode17OnlyXIDStart[:]) {
		return false
	}
	normalized := norm.NFKC.String(string(r))
	first := true
	for _, candidate := range normalized {
		if first {
			if !isIdentifierStartBase(candidate) {
				return false
			}
			first = false
			continue
		}
		if !isIdentifierContinueBase(candidate) {
			return false
		}
	}
	return !first
}

func isXIDContinue(r rune) bool {
	if inRuneRanges(r, unicode17OnlyXIDContinue[:]) {
		return false
	}
	normalized := norm.NFKC.String(string(r))
	if normalized == "" {
		return false
	}
	for _, candidate := range normalized {
		if !isIdentifierContinueBase(candidate) {
			return false
		}
	}
	return true
}

func isIdentifierStartBase(r rune) bool {
	return r == '_' || unicode.In(
		r,
		unicode.Lu,
		unicode.Ll,
		unicode.Lt,
		unicode.Lm,
		unicode.Lo,
		unicode.Nl,
		unicode.Properties["Other_ID_Start"],
	)
}

func isIdentifierContinueBase(r rune) bool {
	return isIdentifierStartBase(r) || unicode.In(
		r,
		unicode.Mn,
		unicode.Mc,
		unicode.Nd,
		unicode.Pc,
		unicode.Properties["Other_ID_Continue"],
	)
}
