package promptssdk

import "strings"

type parser struct {
	tokens []Token
	pos    int
}

func (s *SDK) Parse(req ParseRequest) (ParseResult, error) {
	if req.Options.MaxExpressionLength > 0 && len(req.Expression) > req.Options.MaxExpressionLength {
		return ParseResult{}, &SDKError{Code: ErrParseFailed, Message: "expression exceeds maximum length", Position: &Position{Start: req.Options.MaxExpressionLength, End: len(req.Expression)}}
	}
	tokens, err := lex(req.Expression)
	if err != nil {
		return ParseResult{}, err
	}
	p := &parser{tokens: tokens}
	ast, err := p.parseExpression(0)
	if err != nil {
		return ParseResult{}, err
	}
	if p.peek().Kind != TokenEOF {
		pos := p.peek().Position
		return ParseResult{}, &SDKError{Code: ErrParseFailed, Message: "unexpected token after expression", Position: &pos}
	}
	if err := validate(ast, 0, req.Options.MaxDepth); err != nil {
		return ParseResult{}, err
	}
	return ParseResult{AST: ast, Tokens: tokens, Dependencies: ExtractDependencies(ast)}, nil
}

func (p *parser) parseExpression(minPrec int) (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		prec := precedence(tok.Kind)
		if prec < minPrec {
			break
		}
		p.next()
		right, err := p.parseExpression(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Position: mergePos(left.Pos(), right.Pos()), Op: tok.Literal, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parsePrimary() (Node, error) {
	tok := p.peek()
	switch tok.Kind {
	case TokenString:
		p.next()
		return &StringNode{Position: tok.Position, Value: tok.Literal}, nil
	case TokenVar:
		p.next()
		return &VariableNode{Position: tok.Position, Name: tok.Literal}, nil
	case TokenIdent:
		return p.parseIdentLike()
	case TokenLParen:
		start := p.next().Position
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		end, err := p.expect(TokenRParen, "expected closing parenthesis")
		if err != nil {
			return nil, err
		}
		return &GroupNode{Position: Position{Start: start.Start, End: end.Position.End}, Expr: expr}, nil
	default:
		pos := tok.Position
		return nil, &SDKError{Code: ErrParseFailed, Message: "expected expression", Position: &pos}
	}
}

func (p *parser) parseIdentLike() (Node, error) {
	ident := p.next()
	if p.peek().Kind == TokenLParen {
		return p.parseCall(ident)
	}
	name, version := splitPromptRef(ident.Literal)
	return &PromptRefNode{Position: ident.Position, Name: name, VersionRule: version}, nil
}

func (p *parser) parseCall(ident Token) (Node, error) {
	p.next()
	args := make([]Node, 0)
	if p.peek().Kind != TokenRParen {
		for {
			arg, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().Kind != TokenComma {
				break
			}
			p.next()
		}
	}
	end, err := p.expect(TokenRParen, "expected closing parenthesis")
	if err != nil {
		return nil, err
	}
	option := ""
	endPos := end.Position.End
	if p.peek().Kind == TokenLBrace {
		p.next()
		value, err := p.expect(TokenString, "expected string option")
		if err != nil {
			return nil, err
		}
		option = value.Literal
		close, err := p.expect(TokenRBrace, "expected closing option block")
		if err != nil {
			return nil, err
		}
		endPos = close.Position.End
	}
	return &CallNode{Position: Position{Start: ident.Position.Start, End: endPos}, Name: ident.Literal, Args: args, Option: option}, nil
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *parser) next() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(kind TokenKind, message string) (Token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		pos := tok.Position
		return Token{}, &SDKError{Code: ErrParseFailed, Message: message, Position: &pos}
	}
	return p.next(), nil
}

func precedence(kind TokenKind) int {
	switch kind {
	case TokenHash:
		return 3
	case TokenPlus:
		return 2
	case TokenPipe:
		return 1
	default:
		return -1
	}
}

func splitPromptRef(raw string) (string, string) {
	if raw == "latest" || raw == "*" {
		return raw, ""
	}
	idx := strings.LastIndex(raw, "@")
	if idx <= 0 {
		return raw, ""
	}
	if strings.HasPrefix(raw, "@") && strings.Count(raw, "@") == 1 {
		return raw, ""
	}
	return raw[:idx], raw[idx+1:]
}

func mergePos(a, b Position) Position {
	return Position{Start: a.Start, End: b.End}
}
