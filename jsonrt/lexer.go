/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

/* Portions of this file are on derived from yajl: <https://github.com/lloyd/yajl> */
/*
 * Copyright (c) 2007-2014, Lloyd Hilaiel <me@lloyd.io>
 *
 * Permission to use, copy, modify, and/or distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package jsonrt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type FFParseState int

const (
	FFParse_map_start FFParseState = iota
	FFParse_want_key
	FFParse_want_colon
	FFParse_want_value
	FFParse_after_value
)

type FFTok int

const (
	FFTok_init          FFTok = iota
	FFTok_error         FFTok = iota
	FFTok_bool          FFTok = iota
	FFTok_colon         FFTok = iota
	FFTok_comma         FFTok = iota
	FFTok_eof           FFTok = iota
	FFTok_left_brace    FFTok = iota
	FFTok_left_bracket  FFTok = iota
	FFTok_null          FFTok = iota
	FFTok_right_brace   FFTok = iota
	FFTok_right_bracket FFTok = iota

	/* we differentiate between integers and doubles to allow the
	 * parser to interpret the number without re-scanning */
	FFTok_integer FFTok = iota
	FFTok_double  FFTok = iota

	FFTok_string FFTok = iota

	/* comment tokens are not currently returned to the parser, ever */
	FFTok_comment FFTok = iota
)

type FFErrKind int

const (
	FFErr_e_ok                           FFErrKind = iota
	FFErr_io                             FFErrKind = iota
	FFErr_string_invalid_utf8            FFErrKind = iota
	FFErr_string_invalid_escaped_char    FFErrKind = iota
	FFErr_string_invalid_json_char       FFErrKind = iota
	FFErr_string_invalid_hex_char        FFErrKind = iota
	FFErr_invalid_char                   FFErrKind = iota
	FFErr_invalid_string                 FFErrKind = iota
	FFErr_missing_integer_after_decimal  FFErrKind = iota
	FFErr_missing_integer_after_exponent FFErrKind = iota
	FFErr_missing_integer_after_minus    FFErrKind = iota
	FFErr_unallowed_comment              FFErrKind = iota
	FFErr_incomplete_comment             FFErrKind = iota
	FFErr_unexpected_token_type          FFErrKind = iota // TODO: improve this error
)

// TODO(pquerna): return line number and offset.
func (err FFErrKind) String() string {
	switch err {
	case FFErr_e_ok:
		return ""
	case FFErr_io:
		return "ffjson: IO error"
	case FFErr_string_invalid_utf8:
		return "ffjson: string with invalid UTF-8 sequence"
	case FFErr_string_invalid_escaped_char:
		return "ffjson: string with invalid escaped character"
	case FFErr_string_invalid_json_char:
		return "ffjson: string with invalid JSON character"
	case FFErr_string_invalid_hex_char:
		return "ffjson: string with invalid hex character"
	case FFErr_invalid_char:
		return "ffjson: invalid character"
	case FFErr_invalid_string:
		return "ffjson: invalid string"
	case FFErr_missing_integer_after_decimal:
		return "ffjson: missing integer after decimal"
	case FFErr_missing_integer_after_exponent:
		return "ffjson: missing integer after exponent"
	case FFErr_missing_integer_after_minus:
		return "ffjson: missing integer after minus"
	case FFErr_unallowed_comment:
		return "ffjson: unallowed comment"
	case FFErr_incomplete_comment:
		return "ffjson: incomplete comment"
	case FFErr_unexpected_token_type:
		return "ffjson: unexpected token sequence"
	}

	panic(fmt.Sprintf("unknown FFLexer error type: %v ", err))
}

type FFError struct {
	Kind FFErrKind
	Err  error
}

func (err *FFError) Error() string {
	return err.Err.Error()
}

func NewFFError(kind FFErrKind) *FFError {
	return &FFError{Kind: kind, Err: errors.New(kind.String())}
}

type FFLexer struct {
	reader          *ffReader
	Output          DecodingBuffer //Output仅仅是为了给本库调用者使用，保证  Output = outputbuf
	outputbuf       *Buffer
	Token           FFTok
	lastCurrentChar int
	buf             Buffer
}

func NewFFLexer(input []byte) *FFLexer {
	fl := &FFLexer{
		Token:     FFTok_init,
		reader:    newffReader(input),
		outputbuf: &Buffer{},
	}
	fl.Output = fl.outputbuf //very important!
	// TODO: guess size?
	//fl.outputbuf.Grow(64)
	return fl
}

type LexerError struct {
	offset int
	line   int
	char   int
	err    error
}

// Reset the Lexer and add new input.
func (ffl *FFLexer) Reset(input []byte) {
	ffl.Token = FFTok_init
	ffl.reader.Reset(input)
	ffl.lastCurrentChar = 0
	ffl.outputbuf.Reset()
}

func (le *LexerError) Error() string {
	return fmt.Sprintf(`ffjson error: (%T)%s offset=%d line=%d char=%d`,
		le.err, le.err.Error(),
		le.offset, le.line, le.char)
}

func (ffl *FFLexer) WrapErr(err error) error {
	line, char := ffl.reader.PosWithLine()
	// TOOD: calcualte lines/characters based on offset
	return &LexerError{
		offset: ffl.reader.Pos(),
		line:   line,
		char:   char,
		err:    err,
	}
}

func (ffl *FFLexer) scanReadByte(captureall bool) (byte, error) {
	var c byte
	var err error
	if captureall {
		c, err = ffl.reader.ReadByte()
	} else {
		c, err = ffl.reader.ReadByteNoWS()
	}

	if err != nil {
		return 0, NewFFError(FFErr_io)
	}

	return c, nil
}

func (ffl *FFLexer) readByte() (byte, error) {

	c, err := ffl.reader.ReadByte()
	if err != nil {
		return 0, NewFFError(FFErr_io)
	}

	return c, nil
}

func (ffl *FFLexer) unreadByte() {
	ffl.reader.UnreadByte()
}

func (ffl *FFLexer) wantBytes(want []byte, iftrue FFTok) (FFTok, error) {
	for _, b := range want {
		c, err := ffl.readByte()

		if err != nil {
			return FFTok_error, err
		}

		if c != b {
			ffl.unreadByte()
			return FFTok_error, NewFFError(FFErr_invalid_string)
		}

		ffl.outputbuf.WriteByte(c)
	}

	return iftrue, nil
}

func (ffl *FFLexer) lexComment() (FFTok, error) {
	c, err := ffl.readByte()
	if err != nil {
		return FFTok_error, err
	}

	if c == '/' {
		// a // comment, scan until line ends.
		for {
			c, err := ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}

			if c == '\n' {
				return FFTok_comment, nil
			}
		}
	} else if c == '*' {
		// a /* */ comment, scan */
		for {
			c, err := ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}

			if c == '*' {
				c, err := ffl.readByte()

				if err != nil {
					return FFTok_error, err
				}

				if c == '/' {
					return FFTok_comment, nil
				}

				return FFTok_error, NewFFError(FFErr_incomplete_comment)
			}
		}
	} else {
		return FFTok_error, NewFFError(FFErr_incomplete_comment)
	}
}

func (ffl *FFLexer) lexString(captureall bool) (FFTok, error) {
	if captureall {
		ffl.buf.Reset()
		err := ffl.reader.SliceString(&ffl.buf)

		if err != nil {
			return FFTok_error, err
		}

		WriteJson(ffl.outputbuf, ffl.buf.Bytes())

		return FFTok_string, nil
	} else {
		err := ffl.reader.SliceString(ffl.outputbuf)

		if err != nil {
			return FFTok_error, err
		}

		return FFTok_string, nil
	}
}

func (ffl *FFLexer) lexNumber() (FFTok, error) {
	var numRead int = 0
	tok := FFTok_integer

	c, err := ffl.readByte()
	if err != nil {
		return FFTok_error, err
	}

	/* optional leading minus */
	if c == '-' {
		ffl.outputbuf.WriteByte(c)
		c, err = ffl.readByte()
		if err != nil {
			return FFTok_error, err
		}
	}

	/* a single zero, or a series of integers */
	if c == '0' {
		ffl.outputbuf.WriteByte(c)
		c, err = ffl.readByte()
		if err != nil {
			return FFTok_error, err
		}
	} else if c >= '1' && c <= '9' {
		for c >= '0' && c <= '9' {
			ffl.outputbuf.WriteByte(c)
			c, err = ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}
		}
	} else {
		ffl.unreadByte()
		return FFTok_error, NewFFError(FFErr_missing_integer_after_minus)
	}

	if c == '.' {
		numRead = 0
		ffl.outputbuf.WriteByte(c)
		c, err = ffl.readByte()
		if err != nil {
			return FFTok_error, err
		}

		for c >= '0' && c <= '9' {
			ffl.outputbuf.WriteByte(c)
			numRead++
			c, err = ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}
		}

		if numRead == 0 {
			ffl.unreadByte()

			return FFTok_error, NewFFError(FFErr_missing_integer_after_decimal)
		}

		tok = FFTok_double
	}

	/* optional exponent (indicates this is floating point) */
	if c == 'e' || c == 'E' {
		numRead = 0
		ffl.outputbuf.WriteByte(c)

		c, err = ffl.readByte()
		if err != nil {
			return FFTok_error, err
		}

		/* optional sign */
		if c == '+' || c == '-' {
			ffl.outputbuf.WriteByte(c)
			c, err = ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}
		}

		for c >= '0' && c <= '9' {
			ffl.outputbuf.WriteByte(c)
			numRead++
			c, err = ffl.readByte()
			if err != nil {
				return FFTok_error, err
			}
		}

		if numRead == 0 {
			return FFTok_error, NewFFError(FFErr_missing_integer_after_exponent)
		}

		tok = FFTok_double
	}

	ffl.unreadByte()

	return tok, nil
}

var true_bytes = []byte{'t', 'r', 'u', 'e'}
var false_bytes = []byte{'f', 'a', 'l', 's', 'e'}
var true_bytes1 = []byte{'r', 'u', 'e'}
var false_bytes1 = []byte{'a', 'l', 's', 'e'}
var null_bytes = []byte{'u', 'l', 'l'}

//预期类似   : 8
func (ffl *FFLexer) ScanIntValue(bitSize int) (int64, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return 0, err
	}

	if tok != FFTok_colon { //预期是冒号
		return 0, fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return 0, err
	}

	if tok != FFTok_integer { //预期是 int
		return 0, fmt.Errorf("ffjson: wanted int value, but got token: %v", tok)
	}

	return ParseInt(ffl.Output.Bytes(), 10, bitSize)
}

func (ffl *FFLexer) ScanUintValue(bitSize int) (uint64, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return 0, err
	}

	if tok != FFTok_colon { //预期是冒号
		return 0, fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return 0, err
	}

	if tok != FFTok_integer { //预期是 int
		return 0, fmt.Errorf("ffjson: wanted uint value, but got token: %v", tok)
	}

	return ParseUint(ffl.Output.Bytes(), 10, bitSize)
}

func (ffl *FFLexer) ScanStringValue() (string, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return "", err
	}

	if tok != FFTok_colon { //预期是冒号
		return "", fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return "", err
	}

	if tok != FFTok_string { //预期是 int
		return "", fmt.Errorf("ffjson: wanted string value, but got token: %v", tok)
	}

	return string(ffl.Output.Bytes()), nil
}

func (ffl *FFLexer) ScanBoolValue(bitSize int) (bool, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return false, err
	}

	if tok != FFTok_colon { //预期是冒号
		return false, fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return false, err
	}

	if tok != FFTok_bool { //预期是 bool
		return false, fmt.Errorf("ffjson: wanted bool value, but got token: %v", tok)
	}

	if bytes.Equal(true_bytes, ffl.Output.Bytes()) {
		return true, nil
	} else if bytes.Equal(false_bytes, ffl.Output.Bytes()) {
		return false, nil
	} else {
		return false, fmt.Errorf("ffjson: unknow bool value")
	}
}

func (ffl *FFLexer) ScanFloatValue() (float64, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return 0, err
	}

	if tok != FFTok_colon { //预期是冒号
		return 0, fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return 0, err
	}

	if tok != FFTok_double { //预期是 float
		return 0, fmt.Errorf("ffjson: wanted float value, but got token: %v", tok)
	}

	return ParseFloat(ffl.Output.Bytes(), 64)
}

func (ffl *FFLexer) ScanToValue() (FFTok, error) {
	tok, err := ffl.Scan(false)
	if err != nil {
		return tok, err
	}

	if tok != FFTok_colon {
		return tok, fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", FFTok_colon, tok, ffl.Output.String())
	}

	tok, err = ffl.Scan(false)

	if err != nil {
		return tok, err
	}

	if tok == FFTok_left_brace ||
		tok == FFTok_left_bracket ||
		tok == FFTok_integer ||
		tok == FFTok_double ||
		tok == FFTok_string ||
		tok == FFTok_bool ||
		tok == FFTok_null {
		return tok, nil
	} else {
		return tok, fmt.Errorf("ffjson: wanted value token, but got token: %v", tok)
	}
}

func (ffl *FFLexer) Scan(captureall bool) (FFTok, error) {
	tok := FFTok_error
	if !captureall {
		ffl.outputbuf.Reset()
	}
	ffl.Token = FFTok_init

	var c byte
	var err error

	for {
		if captureall {
			c, err = ffl.reader.ReadByte()
		} else {
			c, err = ffl.reader.ReadByteNoWS()
		}

		if err != nil {
			if err == io.EOF {
				return FFTok_eof, err
			} else {
				return FFTok_error, err
			}
		}

		switch c {
		case '{':
			tok = FFTok_left_bracket
			if captureall {
				ffl.outputbuf.WriteByte('{')
			}
			goto lexed
		case '}':
			tok = FFTok_right_bracket
			if captureall {
				ffl.outputbuf.WriteByte('}')
			}
			goto lexed
		case '[':
			tok = FFTok_left_brace
			if captureall {
				ffl.outputbuf.WriteByte('[')
			}
			goto lexed
		case ']':
			tok = FFTok_right_brace
			if captureall {
				ffl.outputbuf.WriteByte(']')
			}
			goto lexed
		case ',':
			tok = FFTok_comma
			if captureall {
				ffl.outputbuf.WriteByte(',')
			}
			goto lexed
		case ':':
			tok = FFTok_colon
			if captureall {
				ffl.outputbuf.WriteByte(':')
			}
			goto lexed
		case '\t', '\n', '\v', '\f', '\r', ' ':
			if captureall {
				ffl.outputbuf.WriteByte(c)
			}
			break
		case 't':
			ffl.outputbuf.WriteByte('t')
			tok, err = ffl.wantBytes(true_bytes1, FFTok_bool)
			if err != nil {
				return FFTok_error, err
			}
			goto lexed
		case 'f':
			ffl.outputbuf.WriteByte('f')
			tok, err = ffl.wantBytes(false_bytes1, FFTok_bool)
			if err != nil {
				return FFTok_error, err
			}
			goto lexed
		case 'n':
			ffl.outputbuf.WriteByte('n')
			tok, err = ffl.wantBytes(null_bytes, FFTok_null)
			if err != nil {
				return FFTok_error, err
			}
			goto lexed
		case '"':
			tok, err = ffl.lexString(captureall)
			if err != nil {
				return FFTok_error, err
			} else {
				goto lexed
			}
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			ffl.unreadByte()
			tok, err = ffl.lexNumber()
			if err != nil {
				return FFTok_error, err
			}
			goto lexed
		case '/':
			tok, err = ffl.lexComment()
			if err != nil {
				return FFTok_error, err
			}
			goto lexed
		default:
			return FFTok_error, NewFFError(FFErr_invalid_char)
		}
	}

lexed:
	ffl.Token = tok
	return tok, nil
}

func (ffl *FFLexer) captureField(start FFTok) ([]byte, error) {
	switch start {
	case FFTok_left_brace,
		FFTok_left_bracket:
		{
			end := FFTok_right_brace
			if start == FFTok_left_bracket {
				end = FFTok_right_bracket
				ffl.outputbuf.WriteByte('{')
			} else {
				ffl.outputbuf.WriteByte('[')
			}

			depth := 1

			// TODO: work.
		scanloop:
			for {
				tok, err := ffl.Scan(true)
				if err != nil {
					return nil, err
				}
				//fmt.Printf("capture-token: %v end: %v depth: %v\n", tok, end, depth)
				switch tok {
				case FFTok_eof:
					return nil, errors.New("ffjson: unexpected EOF")
				case end:
					depth--
					if depth == 0 {
						break scanloop
					}
				case start:
					depth++
				}
			}

			return ffl.outputbuf.Bytes(), nil

		}
	case FFTok_bool,
		FFTok_integer,
		FFTok_null,
		FFTok_double:
		// simple value, return it.

		return ffl.outputbuf.Bytes(), nil

	case FFTok_string:
		//TODO(pquerna): so, other users expect this to be a quoted string :(

		ffl.buf.Reset()
		WriteJson(&ffl.buf, ffl.outputbuf.Bytes())
		return ffl.buf.Bytes(), nil

	default:
		return nil, fmt.Errorf("ffjson: invalid capture type: %v", start)
	}
	panic("not reached")
}

// Captures an entire field value, including recursive objects,
// and converts them to a []byte suitable to pass to a sub-object's
// UnmarshalJSON
func (ffl *FFLexer) CaptureField(start FFTok) ([]byte, error) {
	return ffl.captureField(start)
}

func (ffl *FFLexer) SkipField(start FFTok) error {
	switch start {
	case FFTok_left_brace, //'['
		FFTok_left_bracket: //'{'
		{
			end := FFTok_right_brace
			if start == FFTok_left_bracket {
				end = FFTok_right_bracket
			}

			depth := 1
		scanloop:
			for {
				tok, err := ffl.Scan(false)
				if err != nil {
					return err
				}

				switch tok {
				case FFTok_eof:
					return fmt.Errorf("ffjson: unexpected EOF")
				case end:
					depth--
					if depth == 0 {
						break scanloop
					}
				case start:
					depth++
				}
			}

			return nil
		}
	case FFTok_bool,
		FFTok_integer,
		FFTok_null,
		FFTok_double,
		FFTok_string:

		return nil
	default:
		return fmt.Errorf("ffjson: invalid capture type: %v", start)
	}

	panic("not reached")
}

func (state FFParseState) String() string {
	switch state {
	case FFParse_map_start:
		return "map:start"
	case FFParse_want_key:
		return "want_key"
	case FFParse_want_colon:
		return "want_colon"
	case FFParse_want_value:
		return "want_value"
	case FFParse_after_value:
		return "after_value"
	}

	panic(fmt.Sprintf("unknown parse state: %d", int(state)))
}

func (tok FFTok) String() string {
	switch tok {
	case FFTok_init:
		return "tok:init"
	case FFTok_bool:
		return "tok:bool"
	case FFTok_colon:
		return "tok:colon"
	case FFTok_comma:
		return "tok:comma"
	case FFTok_eof:
		return "tok:eof"
	case FFTok_error:
		return "tok:error"
	case FFTok_left_brace:
		return "tok:left_brace"
	case FFTok_left_bracket:
		return "tok:left_bracket"
	case FFTok_null:
		return "tok:null"
	case FFTok_right_brace:
		return "tok:right_brace"
	case FFTok_right_bracket:
		return "tok:right_bracket"
	case FFTok_integer:
		return "tok:integer"
	case FFTok_double:
		return "tok:double"
	case FFTok_string:
		return "tok:string"
	case FFTok_comment:
		return "comment"
	}

	panic(fmt.Sprintf("unknown token: %d", int(tok)))
}
