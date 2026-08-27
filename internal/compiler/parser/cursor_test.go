package parser

import (
	"errors"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func TestLookaheadIsLazyAndRewindable(t *testing.T) {
	source := &scriptedTokenSource{items: []cursorItem{
		{token: lexer.Token{Kind: lexer.Comment, Text: "# comment"}},
		{token: lexer.Token{Kind: lexer.NL, Text: "\n"}},
		{token: lexer.Token{Kind: lexer.Name, Text: "value"}},
		{token: lexer.Token{Kind: lexer.Plus, Text: "+"}},
		{token: lexer.Token{Kind: lexer.Number, Text: "1"}},
		{token: lexer.Token{Kind: lexer.EndMarker}},
	}}
	cursor := &tokenCursor{source: source}

	assertCursorToken(t, cursor, 0, lexer.Name, "value")
	if source.calls != 3 {
		t.Fatalf("source calls after first token = %d, want 3", source.calls)
	}
	assertCursorToken(t, cursor, 1, lexer.Plus, "+")
	if source.calls != 4 {
		t.Fatalf("source calls after lookahead = %d, want 4", source.calls)
	}

	mark := cursor.position
	if _, err := cursor.next(); err != nil {
		t.Fatal(err)
	}
	assertCursorToken(t, cursor, 1, lexer.Number, "1")
	if source.calls != 5 {
		t.Fatalf("source calls after second lookahead = %d, want 5", source.calls)
	}
	cursor.reset(mark)
	assertCursorToken(t, cursor, 0, lexer.Name, "value")
	if source.calls != 5 {
		t.Fatalf("rewind reread source: calls = %d, want 5", source.calls)
	}

	for _, want := range []lexer.Kind{lexer.Name, lexer.Plus, lexer.Number, lexer.EndMarker, lexer.EndMarker} {
		token, err := cursor.next()
		if err != nil {
			t.Fatal(err)
		}
		if token.Kind != want {
			t.Fatalf("next token = %s, want %s", token.Kind, want)
		}
	}
	if source.calls != len(source.items) {
		t.Fatalf("source calls at EOF = %d, want %d", source.calls, len(source.items))
	}
}

func TestLexicalFailureIsStableAcrossRewind(t *testing.T) {
	wantErr := errors.New("token failure")
	source := &scriptedTokenSource{items: []cursorItem{
		{token: lexer.Token{Kind: lexer.Name, Text: "value"}},
		{err: wantErr},
	}}
	cursor := &tokenCursor{source: source}

	for range 2 {
		_, err := cursor.peek(1)
		if !errors.Is(err, wantErr) {
			t.Fatalf("peek error = %v, want %v", err, wantErr)
		}
	}
	if source.calls != 2 {
		t.Fatalf("source calls = %d, want 2", source.calls)
	}

	if _, err := cursor.next(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, err := cursor.next()
		if !errors.Is(err, wantErr) {
			t.Fatalf("next error = %v, want %v", err, wantErr)
		}
	}
	if source.calls != 2 {
		t.Fatalf("error reread source: calls = %d, want 2", source.calls)
	}
}

func TestCursorPositionGuards(t *testing.T) {
	cursor := &tokenCursor{source: &scriptedTokenSource{}}
	assertPanics(t, func() { _, _ = cursor.peek(-1) })
	assertPanics(t, func() { cursor.reset(-1) })
	assertPanics(t, func() { cursor.reset(1) })
}

type scriptedTokenSource struct {
	items []cursorItem
	calls int
}

func (source *scriptedTokenSource) Next() (lexer.Token, error) {
	if source.calls >= len(source.items) {
		source.calls++
		return lexer.Token{Kind: lexer.EndMarker}, nil
	}
	item := source.items[source.calls]
	source.calls++
	return item.token, item.err
}

func assertCursorToken(t *testing.T, cursor *tokenCursor, distance int, kind lexer.Kind, text string) {
	t.Helper()
	token, err := cursor.peek(distance)
	if err != nil {
		t.Fatal(err)
	}
	if token.Kind != kind || token.Text != text {
		t.Fatalf("peek(%d) = %s %q, want %s %q", distance, token.Kind, token.Text, kind, text)
	}
}

func assertPanics(t *testing.T, function func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("function did not panic")
		}
	}()
	function()
}
