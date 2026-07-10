package promptssdk

import (
	"fmt"
	"regexp"
)

var allowedInstructions = map[string]struct{}{
	"synthesis": {},
	"keys":      {},
	"tags":      {},
	"labels":    {},
	"summary":   {},
	"merge":     {},
	"title":     {},
	"bullets":   {},
	"clean":     {},
	"dedupe":    {},
	"topic":     {},
	"outline":   {},
	"lead":      {},
	"questions": {},
	"extract":   {},
	"template":  {},
}

var packageNamePattern = regexp.MustCompile(`^(@[a-z0-9][a-z0-9_-]*/)?[a-z0-9][a-z0-9_-]*$`)
var versionRulePattern = regexp.MustCompile(`^(latest|\*|\^?[0-9]+\.[0-9]+\.[0-9]+|~[0-9]+\.[0-9]+\.[0-9]+|>=[0-9]+\.[0-9]+\.[0-9]+|<[0-9]+\.[0-9]+\.[0-9]+)$`)

func validate(node Node, depth int, maxDepth int) error {
	if node == nil {
		return &SDKError{Code: ErrSemanticInvalid, Message: "empty expression"}
	}
	if maxDepth > 0 && depth > maxDepth {
		pos := node.Pos()
		return &SDKError{Code: ErrRuntimeBudgetExceeded, Message: "expression exceeds maximum depth", Position: &pos}
	}
	switch n := node.(type) {
	case *PromptRefNode:
		if !packageNamePattern.MatchString(n.Name) {
			pos := n.Pos()
			return &SDKError{Code: ErrSemanticInvalid, Message: fmt.Sprintf("invalid package name %q", n.Name), Position: &pos}
		}
		if n.VersionRule != "" && !versionRulePattern.MatchString(n.VersionRule) {
			pos := n.Pos()
			return &SDKError{Code: ErrSemanticInvalid, Message: fmt.Sprintf("invalid version rule %q", n.VersionRule), Position: &pos}
		}
	case *CallNode:
		if _, ok := allowedInstructions[n.Name]; !ok {
			pos := n.Pos()
			return &SDKError{Code: ErrSemanticInvalid, Message: fmt.Sprintf("unknown instruction %q", n.Name), Position: &pos}
		}
		for _, arg := range n.Args {
			if err := validate(arg, depth+1, maxDepth); err != nil {
				return err
			}
		}
	case *BinaryNode:
		if n.Op != "+" && n.Op != "|" && n.Op != "#" {
			pos := n.Pos()
			return &SDKError{Code: ErrSemanticInvalid, Message: fmt.Sprintf("unknown operator %q", n.Op), Position: &pos}
		}
		if err := validate(n.Left, depth+1, maxDepth); err != nil {
			return err
		}
		if err := validate(n.Right, depth+1, maxDepth); err != nil {
			return err
		}
	case *GroupNode:
		return validate(n.Expr, depth+1, maxDepth)
	case *StringNode, *VariableNode:
		return nil
	default:
		return &SDKError{Code: ErrSemanticInvalid, Message: "unknown expression node"}
	}
	return nil
}

func ExtractDependencies(node Node) []DependencyRef {
	deps := make([]DependencyRef, 0)
	walkDependencies(node, &deps)
	return deps
}

func walkDependencies(node Node, deps *[]DependencyRef) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *PromptRefNode:
		*deps = append(*deps, DependencyRef{Name: n.Name, VersionRule: n.VersionRule, Position: n.Position, Required: true})
	case *CallNode:
		for _, arg := range n.Args {
			walkDependencies(arg, deps)
		}
	case *BinaryNode:
		walkDependencies(n.Left, deps)
		walkDependencies(n.Right, deps)
	case *GroupNode:
		walkDependencies(n.Expr, deps)
	}
}
