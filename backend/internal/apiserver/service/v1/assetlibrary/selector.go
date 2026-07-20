package assetlibrary

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type selectorParser struct {
	input      string
	position   int
	predicates int
}

func parseAssetSelector(input string) (*iapiserver.AssetSelectorExpression, string, error) {
	if len(input) > 4096 {
		return nil, "", fmt.Errorf("selector exceeds 4096 bytes")
	}
	parser := &selectorParser{input: input}
	expression, err := parser.parseOr(0)
	if err != nil {
		return nil, "", err
	}
	parser.skipSpace()
	if parser.position != len(parser.input) {
		return nil, "", parser.fail("end of selector")
	}
	return expression, strings.TrimSpace(input), nil
}

func (p *selectorParser) parseOr(depth int) (*iapiserver.AssetSelectorExpression, error) {
	left, err := p.parseAnd(depth)
	if err != nil {
		return nil, err
	}
	children := []*iapiserver.AssetSelectorExpression{left}
	for {
		p.skipSpace()
		if !p.consume(';') {
			break
		}
		right, err := p.parseAnd(depth)
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}
	if len(children) == 1 {
		return left, nil
	}
	return &iapiserver.AssetSelectorExpression{Operator: "or", Children: children}, nil
}
func (p *selectorParser) parseAnd(depth int) (*iapiserver.AssetSelectorExpression, error) {
	left, err := p.parseFactor(depth)
	if err != nil {
		return nil, err
	}
	children := []*iapiserver.AssetSelectorExpression{left}
	for {
		p.skipSpace()
		if !p.consume(',') {
			break
		}
		right, err := p.parseFactor(depth)
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}
	if len(children) == 1 {
		return left, nil
	}
	return &iapiserver.AssetSelectorExpression{Operator: "and", Children: children}, nil
}
func (p *selectorParser) parseFactor(depth int) (*iapiserver.AssetSelectorExpression, error) {
	p.skipSpace()
	if p.consume('(') {
		if depth >= 8 {
			return nil, fmt.Errorf("selector nesting exceeds 8")
		}
		expr, err := p.parseOr(depth + 1)
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume(')') {
			return nil, p.fail(")")
		}
		return expr, nil
	}
	predicate, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	p.predicates++
	if p.predicates > 100 {
		return nil, fmt.Errorf("selector predicate count exceeds 100")
	}
	return &iapiserver.AssetSelectorExpression{Operator: "predicate", Predicate: predicate}, nil
}

func (p *selectorParser) parsePredicate() (*iapiserver.AssetSelectorPredicate, error) {
	p.skipSpace()
	if p.position >= len(p.input) {
		return nil, p.fail("predicate")
	}
	if p.consume('#') {
		negative := p.consume('-')
		value, err := p.parseValue("tag")
		if err != nil {
			return nil, err
		}
		action := "eq"
		if negative {
			action = "neq"
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "tag", Action: action, Values: []string{value}}, nil
	}
	if strings.HasPrefix(p.input[p.position:], "@group") {
		p.position += len("@group")
		p.skipSpace()
		if !p.consume('=') {
			return nil, p.fail("=")
		}
		value, err := p.parseValue("group")
		if err != nil {
			return nil, err
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "group", Action: "eq", Values: []string{value}}, nil
	}
	negativeExists := p.consume('!')
	key := p.parseIdentifier()
	if key == "" {
		return nil, p.fail("label key")
	}
	if negativeExists {
		return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "not_exists"}, nil
	}
	p.skipSpace()
	if p.consumeString("notin") {
		values, err := p.parseSet()
		if err != nil {
			return nil, err
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "notin", Values: values}, nil
	}
	if p.consumeString("in") {
		values, err := p.parseSet()
		if err != nil {
			return nil, err
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "in", Values: values}, nil
	}
	if p.consumeToken("!=") {
		value, err := p.parseValue("label value")
		if err != nil {
			return nil, err
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "neq", Values: []string{value}}, nil
	}
	if p.consume('=') {
		value, err := p.parseValue("label value")
		if err != nil {
			return nil, err
		}
		return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "eq", Values: []string{value}}, nil
	}
	return &iapiserver.AssetSelectorPredicate{Kind: "label", Key: key, Action: "exists"}, nil
}

func (p *selectorParser) parseSet() ([]string, error) {
	p.skipSpace()
	if !p.consume('(') {
		return nil, p.fail("(")
	}
	values := make([]string, 0)
	for {
		value, err := p.parseValue("set value")
		if err != nil {
			return nil, err
		}
		if value == "" {
			return nil, fmt.Errorf("selector set values must be non-empty")
		}
		values = append(values, value)
		if len(values) > 100 {
			return nil, fmt.Errorf("selector set exceeds 100 values")
		}
		p.skipSpace()
		if p.consume(')') {
			break
		}
		if !p.consume(',') {
			return nil, p.fail(", or )")
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("selector set must not be empty")
	}
	return values, nil
}
func (p *selectorParser) parseValue(expected string) (string, error) {
	p.skipSpace()
	if p.position >= len(p.input) {
		return "", p.fail(expected)
	}
	if p.input[p.position] == '"' {
		start := p.position
		p.position++
		escaped := false
		for p.position < len(p.input) {
			char := p.input[p.position]
			p.position++
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				raw := p.input[start:p.position]
				value, err := strconv.Unquote(raw)
				if err != nil {
					return "", p.fail("valid JSON string")
				}
				return value, nil
			}
		}
		return "", p.fail("closing quote")
	}
	start := p.position
	for p.position < len(p.input) {
		char := p.input[p.position]
		if char == ',' || char == ';' || char == ')' || unicode.IsSpace(rune(char)) {
			break
		}
		p.position++
	}
	if start == p.position {
		return "", p.fail(expected)
	}
	return p.input[start:p.position], nil
}
func (p *selectorParser) parseIdentifier() string {
	p.skipSpace()
	start := p.position
	for p.position < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.position:])
		if unicode.IsSpace(r) || strings.ContainsRune(",;()=!#@", r) {
			break
		}
		p.position += size
	}
	return p.input[start:p.position]
}
func (p *selectorParser) skipSpace() {
	for p.position < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.position:])
		if !unicode.IsSpace(r) {
			return
		}
		p.position += size
	}
}
func (p *selectorParser) consume(expected byte) bool {
	if p.position < len(p.input) && p.input[p.position] == expected {
		p.position++
		return true
	}
	return false
}
func (p *selectorParser) consumeString(expected string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.input[p.position:], expected) {
		end := p.position + len(expected)
		if end == len(p.input) || unicode.IsSpace(rune(p.input[end])) || p.input[end] == '(' {
			p.position = end
			return true
		}
	}
	return false
}
func (p *selectorParser) consumeToken(expected string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.input[p.position:], expected) {
		p.position += len(expected)
		return true
	}
	return false
}
func (p *selectorParser) fail(expected string) error {
	actual := "EOF"
	if p.position < len(p.input) {
		actual = string(p.input[p.position])
	}
	return fmt.Errorf("selector syntax error at %d: got %q, expected %s", p.position, actual, expected)
}
