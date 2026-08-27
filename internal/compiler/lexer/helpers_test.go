package lexer

import (
	"errors"
	"testing"
)

func collectTokens(source string) ([]Token, error) {
	lexer, err := New(source)
	if err != nil {
		return nil, err
	}
	var tokens []Token
	for {
		token, err := lexer.Next()
		if err != nil {
			return tokens, err
		}
		tokens = append(tokens, token)
		if token.Kind == EndMarker {
			return tokens, nil
		}
	}
}

func assertErrorKind(t *testing.T, source string, want ErrorKind) {
	t.Helper()
	_, err := collectTokens(source)
	var lexErr *Error
	if !errors.As(err, &lexErr) {
		t.Fatalf("error = %#v, want *lexer.Error", err)
	}
	if lexErr.Kind != want {
		t.Fatalf("error kind = %s, want %s: %v", lexErr.Kind, want, lexErr)
	}
}
