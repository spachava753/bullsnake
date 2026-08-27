package lexer

import "unicode/utf8"

// scanPlainString advances through escapes and physical lines until the
// matching single or triple quote, reporting incomplete triple-quoted input.
func (lexer *Lexer) scanPlainString(start, quoteOffset int, _ stringPrefix) (Token, error) {
	quote := lexer.source[quoteOffset]
	quoteSize := lexer.quoteSize(quoteOffset, quote)
	lexer.offset = quoteOffset + quoteSize

	for {
		if lexer.offset == len(lexer.source) {
			return lexer.unterminatedPlainString(start, quoteSize)
		}
		if quoteSize == 1 && newlineSize(lexer.source, lexer.offset) != 0 {
			return lexer.unterminatedPlainString(start, quoteSize)
		}
		if lexer.hasQuote(lexer.offset, quote, quoteSize) {
			lexer.offset += quoteSize
			lexer.atLineStart = false
			lexer.lineHasCode = true
			return lexer.token(String, start, lexer.offset), nil
		}
		if lexer.source[lexer.offset] == '\\' {
			lexer.scanPlainStringEscape()
			continue
		}
		if size := newlineSize(lexer.source, lexer.offset); size != 0 {
			lexer.offset += size
			continue
		}
		_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
		lexer.offset += size
	}
}

func (lexer *Lexer) unterminatedPlainString(start, quoteSize int) (Token, error) {
	span := lexer.span(start, lexer.offset)
	message := "unterminated string literal"
	incomplete := false
	if quoteSize == 3 {
		message = "unterminated triple-quoted string literal"
		incomplete = true
	}
	return lexer.failToken(SyntaxError, message, span, incomplete)
}

func (lexer *Lexer) scanPlainStringEscape() {
	lexer.offset++
	if lexer.offset == len(lexer.source) {
		return
	}
	if size := newlineSize(lexer.source, lexer.offset); size != 0 {
		lexer.offset += size
		return
	}
	_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
	lexer.offset += size
}

func (lexer *Lexer) scanStringStart(start, quoteOffset int, prefix stringPrefix) (Token, error) {
	if len(lexer.modes)+1 >= maxStringMode {
		span := lexer.span(start, quoteOffset+1)
		return lexer.failToken(SyntaxError, "too many nested f-strings or t-strings", span, false)
	}

	quote := lexer.source[quoteOffset]
	quoteSize := lexer.quoteSize(quoteOffset, quote)
	lexer.offset = quoteOffset + quoteSize
	kind := fString
	tokenKind := FStringStart
	if prefix.template {
		kind = tString
		tokenKind = TStringStart
	}
	lexer.modes = append(lexer.modes, stringMode{
		kind:           kind,
		quote:          quote,
		quoteSize:      quoteSize,
		raw:            prefix.raw,
		start:          start,
		text:           true,
		exprStartDepth: -1,
	})
	lexer.lineHasCode = true
	return lexer.token(tokenKind, start, lexer.offset), nil
}

// scanStringText emits f/t-string text until a quote, brace transition, escape,
// or error boundary changes the active scanner mode.
func (lexer *Lexer) scanStringText() (Token, error) {
	modeIndex := len(lexer.modes) - 1
	mode := &lexer.modes[modeIndex]
	start := lexer.offset

	for {
		token, done, err := lexer.scanStringTextBoundary(modeIndex, mode, start)
		if done {
			return token, err
		}
		switch lexer.source[lexer.offset] {
		case '{':
			return lexer.scanStringTextOpenBrace(mode, start)
		case '}':
			return lexer.scanStringTextCloseBrace(mode, start), nil
		case '\\':
			if lexer.scanStringTextEscape(mode.raw) {
				return lexer.stringMiddle(mode.kind, start, lexer.offset), nil
			}
			continue
		}
		if size := newlineSize(lexer.source, lexer.offset); size != 0 {
			lexer.offset += size
			continue
		}
		_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
		lexer.offset += size
	}
}

// scanStringTextBoundary closes the active string, emits pending middle text,
// or reports EOF and newline termination errors.
func (lexer *Lexer) scanStringTextBoundary(modeIndex int, mode *stringMode, start int) (Token, bool, error) {
	if lexer.hasQuote(lexer.offset, mode.quote, mode.quoteSize) {
		if start != lexer.offset {
			return lexer.stringMiddle(mode.kind, start, lexer.offset), true, nil
		}
		lexer.offset += mode.quoteSize
		kind := FStringEnd
		if mode.kind == tString {
			kind = TStringEnd
		}
		lexer.modes = lexer.modes[:modeIndex]
		lexer.atLineStart = false
		return lexer.token(kind, start, lexer.offset), true, nil
	}
	if lexer.offset == len(lexer.source) {
		token, err := lexer.unterminatedStringText(mode)
		return token, true, err
	}
	if mode.quoteSize == 1 && newlineSize(lexer.source, lexer.offset) != 0 {
		token, err := lexer.unterminatedStringText(mode)
		return token, true, err
	}
	return Token{}, false, nil
}

func (lexer *Lexer) unterminatedStringText(mode *stringMode) (Token, error) {
	span := lexer.span(mode.start, lexer.offset)
	prefix := string(mode.prefix())
	message := "unterminated " + prefix + "-string literal"
	if mode.quoteSize == 3 {
		message = "unterminated triple-quoted " + prefix + "-string literal"
	}
	return lexer.failToken(SyntaxError, message, span, mode.quoteSize == 3)
}

// scanStringTextOpenBrace handles doubled braces or switches from text into a
// nested replacement expression.
func (lexer *Lexer) scanStringTextOpenBrace(mode *stringMode, start int) (Token, error) {
	doubled := lexer.offset+1 < len(lexer.source) && lexer.source[lexer.offset+1] == '{'
	if doubled && !mode.inFormatSpec {
		lexer.offset += 2
		return lexer.stringMiddle(mode.kind, start, lexer.offset-1), nil
	}
	mode.exprStartDepth++
	if mode.exprStartDepth >= 3 {
		span := lexer.span(lexer.offset, lexer.offset+1)
		message := string(mode.prefix()) + "-string: expressions nested too deeply"
		return lexer.failToken(SyntaxError, message, span, false)
	}
	mode.text = false
	wasFormatDouble := mode.inFormatSpec && doubled
	mode.inFormatSpec = false
	lexer.atLineStart = false
	if start != lexer.offset || wasFormatDouble {
		return lexer.stringMiddle(mode.kind, start, lexer.offset), nil
	}
	return lexer.scanNormal()
}

func (lexer *Lexer) scanStringTextCloseBrace(mode *stringMode, start int) Token {
	if lexer.offset+1 < len(lexer.source) && lexer.source[lexer.offset+1] == '}' && !mode.inFormatSpec && mode.curlyDepth == 0 {
		lexer.offset += 2
		return lexer.stringMiddle(mode.kind, start, lexer.offset-1)
	}
	mode.text = false
	lexer.atLineStart = false
	return lexer.stringMiddle(mode.kind, start, lexer.offset)
}

// scanStringTextEscape consumes one escape, keeping brace escapes available for
// replacement parsing and consuming complete named Unicode escapes as text.
func (lexer *Lexer) scanStringTextEscape(raw bool) bool {
	lexer.offset++
	if lexer.offset == len(lexer.source) {
		return false
	}
	if lexer.source[lexer.offset] == '{' || lexer.source[lexer.offset] == '}' {
		return false
	}
	if !raw && lexer.source[lexer.offset] == 'N' && lexer.peekByte(lexer.offset+1) == '{' {
		return lexer.scanNamedUnicodeEscape()
	}
	if size := newlineSize(lexer.source, lexer.offset); size != 0 {
		lexer.offset += size
		return false
	}
	_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
	lexer.offset += size
	return false
}

func (lexer *Lexer) scanNamedUnicodeEscape() bool {
	lexer.offset += 2
	for lexer.offset < len(lexer.source) {
		if lexer.source[lexer.offset] == '}' {
			lexer.offset++
			return true
		}
		_, size := utf8.DecodeRuneInString(lexer.source[lexer.offset:])
		lexer.offset += size
	}
	return false
}

func (lexer *Lexer) stringMiddle(kind stringKind, start, end int) Token {
	tokenKind := FStringMiddle
	if kind == tString {
		tokenKind = TStringMiddle
	}
	lexer.atLineStart = false
	return lexer.token(tokenKind, start, end)
}

func (lexer *Lexer) quoteSize(offset int, quote byte) int {
	if offset+2 < len(lexer.source) && lexer.source[offset+1] == quote && lexer.source[offset+2] == quote {
		return 3
	}
	return 1
}

func (lexer *Lexer) hasQuote(offset int, quote byte, size int) bool {
	if offset+size > len(lexer.source) {
		return false
	}
	for index := range size {
		if lexer.source[offset+index] != quote {
			return false
		}
	}
	return true
}
