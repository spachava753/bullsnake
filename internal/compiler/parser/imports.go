package parser

import (
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func (parser *parserState) parseImportStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("import", "expected 'import'")
	if err != nil {
		return nil, err
	}
	aliases, end, err := parser.parseImportAliases(true, lexer.EndMarker)
	if err != nil {
		return nil, err
	}
	return &compilerast.ImportStmt{Range: joinSpans(keyword.Span, end), Names: aliases}, nil
}

// parseFromImportStatement parses relative levels, an optional module, and
// wildcard or aliased import targets.
func (parser *parserState) parseFromImportStatement() (compilerast.Stmt, error) {
	keyword, err := parser.expectKeyword("from", "expected 'from'")
	if err != nil {
		return nil, err
	}
	level := 0
	for {
		token, err := parser.peek(0)
		if err != nil {
			return nil, err
		}
		if token.Kind == lexer.Dot {
			level++
		} else if token.Kind == lexer.Ellipsis {
			level += 3
		} else {
			break
		}
		if _, err := parser.advance(); err != nil {
			return nil, err
		}
	}

	module := ""
	token, err := parser.peek(0)
	if err != nil {
		return nil, err
	}
	if token.Kind == lexer.Name && token.Text != "import" {
		module, _, err = parser.parseDottedName()
		if err != nil {
			return nil, err
		}
	} else if level == 0 {
		return nil, parser.syntaxError(token, "expected module name")
	}
	if _, err := parser.expectKeyword("import", "expected 'import'"); err != nil {
		return nil, err
	}

	star, wildcard, err := parser.take(lexer.Star)
	if err != nil {
		return nil, err
	}
	if wildcard {
		return &compilerast.FromImportStmt{
			Range:    joinSpans(keyword.Span, star.Span),
			Module:   module,
			Level:    level,
			Wildcard: true,
		}, nil
	}

	_, parenthesized, err := parser.take(lexer.LParen)
	if err != nil {
		return nil, err
	}
	terminator := lexer.EndMarker
	if parenthesized {
		terminator = lexer.RParen
	}
	aliases, end, err := parser.parseImportAliases(false, terminator)
	if err != nil {
		return nil, err
	}
	if parenthesized {
		close, err := parser.expect(lexer.RParen, "expected ')' after import names")
		if err != nil {
			return nil, err
		}
		end = close.Span
	}
	return &compilerast.FromImportStmt{
		Range:  joinSpans(keyword.Span, end),
		Module: module,
		Names:  aliases,
		Level:  level,
	}, nil
}

// parseImportAliases parses comma-separated imported names. Dotted names are
// accepted for import statements and rejected for from-import targets.
func (parser *parserState) parseImportAliases(dotted bool, terminator lexer.Kind) ([]compilerast.ImportAlias, lexer.Span, error) {
	var aliases []compilerast.ImportAlias
	var end lexer.Span
	for {
		alias, err := parser.parseImportAlias(dotted)
		if err != nil {
			return nil, end, err
		}
		aliases = append(aliases, alias)
		end = alias.Range
		_, comma, err := parser.take(lexer.Comma)
		if err != nil {
			return nil, end, err
		}
		if !comma {
			break
		}
		token, err := parser.peek(0)
		if err != nil {
			return nil, end, err
		}
		if token.Kind == terminator || token.Kind == lexer.Newline || token.Kind == lexer.EndMarker {
			break
		}
	}
	return aliases, end, nil
}

// parseImportAlias parses one dotted or simple imported name and its optional
// as alias.
func (parser *parserState) parseImportAlias(dotted bool) (compilerast.ImportAlias, error) {
	name, span, err := parser.parseDottedName()
	if err != nil {
		return compilerast.ImportAlias{}, err
	}
	if !dotted && strings.Contains(name, ".") {
		return compilerast.ImportAlias{}, parser.errorAt(span, "from-import target must be a name", false)
	}
	alias := ""
	if _, matched, err := parser.takeKeyword("as"); err != nil {
		return compilerast.ImportAlias{}, err
	} else if matched {
		token, err := parser.expect(lexer.Name, "expected name after 'as'")
		if err != nil {
			return compilerast.ImportAlias{}, err
		}
		if isHardKeyword(token.Text) {
			return compilerast.ImportAlias{}, parser.syntaxError(token, "expected name after 'as'")
		}
		alias = token.Text
		span = joinSpans(span, token.Span)
	}
	return compilerast.ImportAlias{Range: span, Name: name, Alias: alias}, nil
}

// parseDottedName joins one or more identifier components while retaining the
// source span from the first through final name.
func (parser *parserState) parseDottedName() (string, lexer.Span, error) {
	first, err := parser.expect(lexer.Name, "expected imported name")
	if err != nil {
		return "", first.Span, err
	}
	if isHardKeyword(first.Text) {
		return "", first.Span, parser.syntaxError(first, "expected imported name")
	}
	parts := []string{first.Text}
	end := first.Span
	for {
		_, matched, err := parser.take(lexer.Dot)
		if err != nil {
			return "", end, err
		}
		if !matched {
			break
		}
		part, err := parser.expect(lexer.Name, "expected name after '.'")
		if err != nil {
			return "", end, err
		}
		if isHardKeyword(part.Text) {
			return "", end, parser.syntaxError(part, "expected name after '.'")
		}
		parts = append(parts, part.Text)
		end = part.Span
	}
	return strings.Join(parts, "."), joinSpans(first.Span, end), nil
}
