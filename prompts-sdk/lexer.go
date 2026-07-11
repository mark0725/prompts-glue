package promptssdk

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenKind string

const (
	TokenEOF    TokenKind = "eof"
	TokenIdent  TokenKind = "ident"
	TokenString TokenKind = "string"
	TokenVar    TokenKind = "var"
	TokenOption TokenKind = "option"
	TokenLParen TokenKind = "("
	TokenRParen TokenKind = ")"
	TokenComma  TokenKind = ","
	TokenPlus   TokenKind = "+"
	TokenPipe   TokenKind = "|"
	TokenHash   TokenKind = "#"
)

type Token struct {
	Kind     TokenKind
	Literal  string
	Position Position
}

func lex(input string) ([]Token, error) {
	tokens := make([]Token, 0)
	for i := 0; i < len(input); {
		r := rune(input[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		start := i
		switch r {
		case '(':
			tokens = append(tokens, simpleToken(TokenLParen, input, start))
			i++
		case ')':
			tokens = append(tokens, simpleToken(TokenRParen, input, start))
			i++
		case ',':
			tokens = append(tokens, simpleToken(TokenComma, input, start))
			i++
		case '+':
			tokens = append(tokens, simpleToken(TokenPlus, input, start))
			i++
		case '|':
			tokens = append(tokens, simpleToken(TokenPipe, input, start))
			i++
		case '#':
			tokens = append(tokens, simpleToken(TokenHash, input, start))
			i++
		case '{':
			tok, next, err := lexOption(input, start)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		case '"':
			tok, next, err := lexString(input, start)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		case '$':
			tok, next, err := lexVariable(input, start)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		default:
			if isIdentStart(r) {
				tok, next := lexIdent(input, start)
				tokens = append(tokens, tok)
				i = next
				continue
			}
			return nil, &SDKError{Code: ErrParseFailed, Message: fmt.Sprintf("unexpected character %q", r), Position: &Position{Start: start, End: start + 1}}
		}
	}
	tokens = append(tokens, Token{Kind: TokenEOF, Position: Position{Start: len(input), End: len(input)}})
	return tokens, nil
}

func simpleToken(kind TokenKind, input string, start int) Token {
	return Token{Kind: kind, Literal: input[start : start+1], Position: Position{Start: start, End: start + 1}}
}

func lexString(input string, start int) (Token, int, error) {
	var b strings.Builder
	for i := start + 1; i < len(input); i++ {
		ch := input[i]
		if ch == '"' {
			return Token{Kind: TokenString, Literal: b.String(), Position: Position{Start: start, End: i + 1}}, i + 1, nil
		}
		if ch == '\\' {
			if i+1 >= len(input) {
				return Token{}, 0, &SDKError{Code: ErrParseFailed, Message: "unterminated escape sequence", Position: &Position{Start: i, End: i + 1}}
			}
			i++
			switch input[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(input[i])
			default:
				b.WriteByte(input[i])
			}
			continue
		}
		b.WriteByte(ch)
	}
	return Token{}, 0, &SDKError{Code: ErrParseFailed, Message: "unterminated string literal", Position: &Position{Start: start, End: len(input)}}
}

func lexVariable(input string, start int) (Token, int, error) {
	if start+1 >= len(input) || !isNameStart(rune(input[start+1])) {
		return Token{}, 0, &SDKError{Code: ErrParseFailed, Message: "expected variable name after $", Position: &Position{Start: start, End: start + 1}}
	}
	i := start + 2
	for i < len(input) && isNamePart(rune(input[i])) {
		i++
	}
	return Token{Kind: TokenVar, Literal: input[start+1 : i], Position: Position{Start: start, End: i}}, i, nil
}

func lexIdent(input string, start int) (Token, int) {
	i := start
	for i < len(input) && isIdentPart(rune(input[i])) {
		i++
	}
	return Token{Kind: TokenIdent, Literal: input[start:i], Position: Position{Start: start, End: i}}, i
}

// lexOption 读取 `{...}` 指令参数（用户特别提示词）的原始内部文本，直到匹配的 `}`。
// 规则：双引号内的 `}` 与 `{` 不影响匹配；非引号内按花括号深度计数（支持 `{a,b}` 这类内层花括号）。
// 字面量为原始内部文本，不做转义处理，交由 normalizeOption 归一化。
func lexOption(input string, start int) (Token, int, error) {
	depth := 1
	inString := false
	for i := start + 1; i < len(input); i++ {
		ch := input[i]
		if inString {
			if ch == '\\' {
				i++ // 跳过被转义的下一个字符，使 \" 不结束引号
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return Token{Kind: TokenOption, Literal: input[start+1 : i], Position: Position{Start: start, End: i + 1}}, i + 1, nil
			}
		}
	}
	return Token{}, 0, &SDKError{Code: ErrParseFailed, Message: "unterminated option, expected closing }", Position: &Position{Start: start, End: len(input)}}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '@' || r == '_' || r == '-' || r == '<' || r == '>' || r == '~' || r == '^' || r == '*'
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || r == '/' || r == '.' || r == '='
}

func isNameStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isNamePart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
