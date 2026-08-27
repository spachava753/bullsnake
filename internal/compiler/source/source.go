// Package source loads Python source bytes into immutable UTF-8 text.
package source

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

const utf8BOM = "\xef\xbb\xbf"

var (
	codingCookie = regexp.MustCompile(`^[ \t\f]*#.*?coding[:=][ \t]*([A-Za-z0-9_.-]+)`)

	pythonEncodingAliases = map[string]string{
		"646":            "US-ASCII",
		"ansi_x3_4_1968": "US-ASCII",
		"ascii":          "US-ASCII",
		"cp1250":         "windows-1250",
		"cp1251":         "windows-1251",
		"cp1252":         "windows-1252",
		"cp1253":         "windows-1253",
		"cp1254":         "windows-1254",
		"cp1255":         "windows-1255",
		"cp1256":         "windows-1256",
		"cp1257":         "windows-1257",
		"cp1258":         "windows-1258",
		"cp874":          "windows-874",
		"cp932":          "Shift_JIS",
		"cp936":          "GBK",
		"cp949":          "EUC-KR",
		"cp950":          "Big5",
		"euc_jp":         "EUC-JP",
		"euc_kr":         "EUC-KR",
		"eucjp":          "EUC-JP",
		"euckr":          "EUC-KR",
		"hz":             "HZ-GB-2312",
		"iso2022_jp":     "ISO-2022-JP",
		"korean":         "EUC-KR",
		"ms932":          "Shift_JIS",
		"ms_kanji":       "Shift_JIS",
		"mskanji":        "Shift_JIS",
		"shift_jis":      "Shift_JIS",
		"shiftjis":       "Shift_JIS",
		"sjis":           "Shift_JIS",
		"u8":             "UTF-8",
		"utf":            "UTF-8",
		"utf8":           "UTF-8",
	}
)

// Unit is one completely loaded and decoded Python source unit.
type Unit struct {
	Filename string
	Encoding string
	Text     string
}

// Error reports source-encoding detection or decoding failure. Line is zero
// when the loader cannot identify a physical source line.
type Error struct {
	Filename string
	Line     int
	Message  string
	Err      error
}

func (err *Error) Error() string {
	location := err.Filename
	if err.Line != 0 {
		if location == "" {
			location = fmt.Sprintf("line %d", err.Line)
		} else {
			location = fmt.Sprintf("%s:%d", location, err.Line)
		}
	}
	if location == "" {
		return fmt.Sprintf("source encoding: %s", err.Message)
	}
	return fmt.Sprintf("%s: source encoding: %s", location, err.Message)
}

// Unwrap returns the codec or transformation error behind err, when present.
func (err *Error) Unwrap() error {
	return err.Err
}

// Decode detects the encoding of data and returns immutable UTF-8 source text.
func Decode(filename string, data []byte) (Unit, error) {
	name, bom, err := detectEncoding(data)
	if err != nil {
		err.Filename = filename
		return Unit{}, err
	}
	if bom {
		data = data[len(utf8BOM):]
	}

	text, line, decodeErr := decode(name, data)
	if decodeErr != nil {
		return Unit{}, &Error{
			Filename: filename,
			Line:     line,
			Message:  fmt.Sprintf("source is not valid %s", name),
			Err:      decodeErr,
		}
	}
	return Unit{Filename: filename, Encoding: name, Text: text}, nil
}

// Read reads all bytes from reader before detecting and decoding the source.
func Read(filename string, reader io.Reader) (Unit, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		if filename == "" {
			return Unit{}, fmt.Errorf("read source: %w", err)
		}
		return Unit{}, fmt.Errorf("read source %q: %w", filename, err)
	}
	return Decode(filename, data)
}

// ReadFile reads and decodes one source file.
func ReadFile(filename string) (Unit, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Unit{}, err
	}
	return Decode(filename, data)
}

// detectEncoding applies BOM and first-two-line cookie precedence without
// decoding bytes that may belong to a legacy source encoding.
func detectEncoding(data []byte) (name string, bom bool, err *Error) {
	bom = bytes.HasPrefix(data, []byte(utf8BOM))
	if bom {
		data = data[len(utf8BOM):]
	}

	first, next := firstPhysicalLine(data, 0)
	if cookie := findCookie(first); cookie != "" {
		name, err = resolveCookie(cookie, 1, bom)
		return name, bom, err
	}
	if !blankOrComment(first) || next == len(data) {
		return "utf-8", bom, nil
	}

	second, _ := firstPhysicalLine(data, next)
	if cookie := findCookie(second); cookie != "" {
		name, err = resolveCookie(cookie, 2, bom)
		return name, bom, err
	}
	return "utf-8", bom, nil
}

// resolveCookie normalizes and validates a declared encoding, including its
// consistency with a leading UTF-8 BOM.
func resolveCookie(cookie string, line int, bom bool) (string, *Error) {
	name := normalizeEncodingName(cookie)
	if bom && name != "utf-8" {
		return "", &Error{Line: line, Message: fmt.Sprintf("encoding %q conflicts with UTF-8 BOM", name)}
	}
	codec, nativeUTF8, lookupErr := lookupEncoding(name)
	if lookupErr != nil {
		return "", &Error{Line: line, Message: fmt.Sprintf("unknown or unsupported source encoding %q", name), Err: lookupErr}
	}
	if !nativeUTF8 && !asciiCompatible(codec) {
		return "", &Error{Line: line, Message: fmt.Sprintf("source encoding %q is not ASCII-compatible", name)}
	}
	return name, nil
}

func findCookie(line []byte) string {
	match := codingCookie.FindSubmatch(line)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func blankOrComment(line []byte) bool {
	for _, char := range line {
		switch char {
		case ' ', '\t', '\f':
			continue
		case '#':
			return true
		default:
			return false
		}
	}
	return true
}

// firstPhysicalLine slices one CR, LF, or CRLF terminated line and returns the
// offset immediately after its line ending.
func firstPhysicalLine(data []byte, start int) ([]byte, int) {
	end := start
	for end < len(data) && data[end] != '\r' && data[end] != '\n' {
		end++
	}
	next := end
	if next < len(data) {
		if data[next] == '\r' && next+1 < len(data) && data[next+1] == '\n' {
			next += 2
		} else {
			next++
		}
	}
	return data[start:end], next
}

// normalizeEncodingName follows CPython's special UTF-8 and Latin-1 cookie
// normalization while retaining other declared spellings.
func normalizeEncodingName(name string) string {
	prefix := name
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	prefix = strings.ToLower(strings.ReplaceAll(prefix, "_", "-"))
	if prefix == "utf-8" || strings.HasPrefix(prefix, "utf-8-") {
		return "utf-8"
	}
	if prefix == "latin-1" || prefix == "iso-8859-1" || prefix == "iso-latin-1" ||
		strings.HasPrefix(prefix, "latin-1-") || strings.HasPrefix(prefix, "iso-8859-1-") ||
		strings.HasPrefix(prefix, "iso-latin-1-") {
		return "iso-8859-1"
	}
	return name
}

// decode validates UTF-8 directly and transcodes supported legacy encodings,
// rejecting replacement output caused by malformed source bytes.
func decode(name string, data []byte) (text string, line int, err error) {
	codec, nativeUTF8, err := lookupEncoding(name)
	if err != nil {
		return "", 0, err
	}
	if nativeUTF8 {
		if offset := firstInvalidUTF8(data); offset >= 0 {
			return "", physicalLineAt(data, offset), errors.New("invalid UTF-8")
		}
		return string(data), 0, nil
	}
	if !asciiCompatible(codec) {
		return "", 0, errors.New("source encoding is not ASCII-compatible")
	}

	decoded, consumed, err := transform.Bytes(codec.NewDecoder(), data)
	if err != nil {
		return "", physicalLineAt(data, consumed), err
	}
	if replacement := bytes.Index(decoded, []byte("\ufffd")); replacement >= 0 {
		encoded, encodeErr := codec.NewEncoder().Bytes(decoded)
		if encodeErr != nil || !bytes.Equal(encoded, data) {
			return "", physicalLineAt(decoded, replacement), errors.New("source encoding contains an invalid byte sequence")
		}
	}
	return string(decoded), 0, nil
}

// lookupEncoding resolves Python aliases before consulting the IANA registry
// implemented by golang.org/x/text.
func lookupEncoding(name string) (codec encoding.Encoding, nativeUTF8 bool, err error) {
	alias := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if replacement, ok := pythonEncodingAliases[alias]; ok {
		name = replacement
	} else if suffix, ok := strings.CutPrefix(alias, "iso8859_"); ok {
		name = "ISO-8859-" + suffix
	} else if suffix, ok := strings.CutPrefix(alias, "iso_8859_"); ok {
		name = "ISO-8859-" + suffix
	}
	if strings.EqualFold(name, "UTF-8") {
		return nil, true, nil
	}

	codec, err = ianaindex.IANA.Encoding(name)
	if err != nil && strings.Contains(name, "_") {
		codec, err = ianaindex.IANA.Encoding(strings.ReplaceAll(name, "_", "-"))
	}
	if err != nil {
		return nil, false, err
	}
	if codec == nil {
		return nil, false, errors.New("encoding is recognized but not implemented")
	}
	return codec, false, nil
}

func asciiCompatible(codec encoding.Encoding) bool {
	const probe = " \t\f# coding: utf-8\n"
	decoded, err := codec.NewDecoder().Bytes([]byte(probe))
	return err == nil && string(decoded) == probe
}

func firstInvalidUTF8(data []byte) int {
	for offset := 0; offset < len(data); {
		_, size := utf8.DecodeRune(data[offset:])
		if size == 1 && data[offset] >= utf8.RuneSelf {
			return offset
		}
		offset += size
	}
	return -1
}

// physicalLineAt converts a raw byte offset to a one-based physical line while
// treating CRLF as one line ending.
func physicalLineAt(data []byte, target int) int {
	line := 1
	for offset := 0; offset < target && offset < len(data); {
		switch data[offset] {
		case '\r':
			line++
			offset++
			if offset < target && offset < len(data) && data[offset] == '\n' {
				offset++
			}
		case '\n':
			line++
			offset++
		default:
			offset++
		}
	}
	return line
}
