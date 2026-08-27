package parser

import "github.com/spachava753/bullsnake/internal/compiler/lexer"

type tokenSource interface {
	Next() (lexer.Token, error)
}

type cursorItem struct {
	token lexer.Token
	err   error
}

type tokenCursor struct {
	source   tokenSource
	items    []cursorItem
	position int
	terminal bool
}

func (cursor *tokenCursor) peek(distance int) (lexer.Token, error) {
	if distance < 0 {
		panic("parser token cursor: negative lookahead")
	}
	index := cursor.position + distance
	cursor.fill(index)
	if index >= len(cursor.items) {
		index = len(cursor.items) - 1
	}
	item := cursor.items[index]
	return item.token, item.err
}

func (cursor *tokenCursor) next() (lexer.Token, error) {
	token, err := cursor.peek(0)
	if err == nil && token.Kind != lexer.EndMarker {
		cursor.position++
	}
	return token, err
}

func (cursor *tokenCursor) reset(mark int) {
	if mark < 0 || mark > len(cursor.items) {
		panic("parser token cursor: invalid mark")
	}
	cursor.position = mark
}

// fill pulls only enough significant tokens to satisfy lookahead and caches a terminal token or error.
func (cursor *tokenCursor) fill(index int) {
	for len(cursor.items) <= index && !cursor.terminal {
		token, err := cursor.source.Next()
		if err == nil && token.IsTrivia() {
			continue
		}
		cursor.items = append(cursor.items, cursorItem{token: token, err: err})
		if err != nil || token.Kind == lexer.EndMarker {
			cursor.terminal = true
		}
	}
}
