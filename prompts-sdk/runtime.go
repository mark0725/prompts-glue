package promptssdk

import (
	"context"
	"fmt"
	"strings"
)

func (s *SDK) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	ast := req.AST
	if ast == nil {
		parsed, err := s.Parse(ParseRequest{Expression: req.Expression})
		if err != nil {
			return RunResult{}, err
		}
		ast = parsed.AST
	}
	if req.Options.ErrorStrategy == "" {
		req.Options.ErrorStrategy = ErrorStrict
	}
	r := &runner{sdk: s, req: req, ctx: ctx}
	content, err := r.eval(ast, 0)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Content: content, Dependencies: r.dependencies, Trace: r.trace, Calculation: Calculation{TokenUsage: r.tokens, Steps: r.steps}, Warnings: r.warnings}, nil
}

type runner struct {
	sdk          *SDK
	req          RunRequest
	ctx          context.Context
	dependencies []DependencyRef
	trace        []TraceStep
	warnings     []string
	tokens       TokenUsage
	steps        int
}

func (r *runner) eval(node Node, depth int) (string, error) {
	if r.req.Options.MaxDepth > 0 && depth > r.req.Options.MaxDepth {
		pos := node.Pos()
		return "", &SDKError{Code: ErrRuntimeBudgetExceeded, Message: "runtime exceeds maximum depth", Position: &pos}
	}
	if r.req.Options.MaxSteps > 0 && r.steps >= r.req.Options.MaxSteps {
		pos := node.Pos()
		return "", &SDKError{Code: ErrRuntimeBudgetExceeded, Message: "runtime exceeds maximum steps", Position: &pos}
	}
	r.steps++
	switch n := node.(type) {
	case *StringNode:
		r.addTrace(n.Kind(), "", n.Value)
		return n.Value, nil
	case *VariableNode:
		value, ok := r.req.Context[n.Name]
		if !ok {
			if r.req.Options.ErrorStrategy == ErrorLenient {
				r.warnings = append(r.warnings, fmt.Sprintf("missing context variable %s", n.Name))
				return "", nil
			}
			pos := n.Pos()
			return "", &SDKError{Code: ErrContextMissing, Message: fmt.Sprintf("missing context variable %s", n.Name), Position: &pos}
		}
		r.addTrace(n.Kind(), n.Name, value)
		return value, nil
	case *PromptRefNode:
		if r.req.ContentLoader == nil {
			pos := n.Pos()
			return "", &SDKError{Code: ErrDependencyLoadFailed, Message: "content loader is required", Position: &pos}
		}
		ref := DependencyRef{Name: n.Name, VersionRule: n.VersionRule, Position: n.Position, Required: true}
		content, err := r.req.ContentLoader.LoadPrompt(r.ctx, ref)
		if err != nil {
			pos := n.Pos()
			return "", &SDKError{Code: ErrDependencyLoadFailed, Message: err.Error(), Position: &pos}
		}
		r.dependencies = append(r.dependencies, ref)
		r.addTrace(n.Kind(), n.Name, content.Text)
		return content.Text, nil
	case *CallNode:
		inputs := make([]string, 0, len(n.Args))
		for _, arg := range n.Args {
			value, err := r.eval(arg, depth+1)
			if err != nil {
				return "", err
			}
			inputs = append(inputs, value)
		}
		result, err := r.executeInstruction(n, inputs)
		if err != nil {
			return "", err
		}
		r.addTrace(n.Kind(), n.Name, result)
		return result, nil
	case *BinaryNode:
		left, err := r.eval(n.Left, depth+1)
		if err != nil {
			return "", err
		}
		right, err := r.eval(n.Right, depth+1)
		if err != nil {
			return "", err
		}
		result, err := r.executeBinary(n, left, right)
		if err != nil {
			return "", err
		}
		r.addTrace(n.Kind(), n.Op, result)
		return result, nil
	case *GroupNode:
		return r.eval(n.Expr, depth+1)
	default:
		return "", &SDKError{Code: ErrSemanticInvalid, Message: "unknown expression node"}
	}
}

func (r *runner) executeInstruction(node *CallNode, inputs []string) (string, error) {
	if r.req.InstructionExecutor != nil {
		res, err := r.req.InstructionExecutor.ExecuteInstruction(r.ctx, InstructionRequest{Name: node.Name, Inputs: inputs, Option: node.Option})
		if err != nil {
			pos := node.Pos()
			return "", &SDKError{Code: ErrInstructionFailed, Message: err.Error(), Position: &pos}
		}
		r.tokens.Input += res.TokenUsage.Input
		r.tokens.Output += res.TokenUsage.Output
		r.tokens.Total += res.TokenUsage.Total
		r.warnings = append(r.warnings, res.Warnings...)
		return res.Content, nil
	}
	result := defaultInstruction(node.Name, inputs, node.Option)
	inputTokens := 0
	for _, input := range inputs {
		inputTokens += tokenEstimate(input)
	}
	outputTokens := tokenEstimate(result)
	r.tokens.Input += inputTokens
	r.tokens.Output += outputTokens
	r.tokens.Total += inputTokens + outputTokens
	return result, nil
}

func (r *runner) executeBinary(node *BinaryNode, left string, right string) (string, error) {
	switch node.Op {
	case "|":
		return defaultInstruction("merge", []string{left, right}, ""), nil
	case "+":
		return defaultInstruction("synthesis", []string{left, right}, ""), nil
	case "#":
		return defaultInstruction("tags", []string{left, right}, ""), nil
	default:
		pos := node.Pos()
		return "", &SDKError{Code: ErrSemanticInvalid, Message: fmt.Sprintf("unknown operator %s", node.Op), Position: &pos}
	}
}

func defaultInstruction(name string, inputs []string, option string) string {
	joined := strings.TrimSpace(strings.Join(nonEmpty(inputs), "\n"))
	switch name {
	case "merge", "synthesis":
		return joined
	case "summary":
		if option != "" {
			return strings.TrimSpace(option + "\n" + joined)
		}
		return joined
	case "clean":
		return strings.Join(strings.Fields(joined), " ")
	case "dedupe":
		seen := map[string]struct{}{}
		lines := make([]string, 0)
		for _, line := range strings.Split(joined, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	case "template":
		if option == "" {
			return joined
		}
		return strings.ReplaceAll(option, "{{content}}", joined)
	case "keys", "tags", "labels", "title", "bullets", "topic", "outline", "lead", "questions", "extract":
		if option != "" {
			return strings.TrimSpace(option + "\n" + joined)
		}
		return joined
	default:
		return joined
	}
}

func nonEmpty(inputs []string) []string {
	out := make([]string, 0, len(inputs))
	for _, input := range inputs {
		input = strings.TrimSpace(input)
		if input != "" {
			out = append(out, input)
		}
	}
	return out
}

func (r *runner) addTrace(kind NodeKind, name string, output string) {
	if len(output) > 120 {
		output = output[:120]
	}
	r.trace = append(r.trace, TraceStep{Kind: kind, Name: name, Output: output})
}
