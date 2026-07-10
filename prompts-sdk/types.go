package promptssdk

import (
	"context"
	"fmt"
	"strings"
)

type NodeKind string

const (
	NodePromptRef NodeKind = "prompt_ref"
	NodeString    NodeKind = "string"
	NodeCall      NodeKind = "call"
	NodeBinary    NodeKind = "binary"
	NodeVariable  NodeKind = "variable"
	NodeGroup     NodeKind = "group"
)

type Position struct {
	Start int
	End   int
}

type Node interface {
	Kind() NodeKind
	Pos() Position
}

type PromptRefNode struct {
	Position    Position
	Name        string
	VersionRule string
}

func (n *PromptRefNode) Kind() NodeKind { return NodePromptRef }
func (n *PromptRefNode) Pos() Position  { return n.Position }

type StringNode struct {
	Position Position
	Value    string
}

func (n *StringNode) Kind() NodeKind { return NodeString }
func (n *StringNode) Pos() Position  { return n.Position }

type VariableNode struct {
	Position Position
	Name     string
}

func (n *VariableNode) Kind() NodeKind { return NodeVariable }
func (n *VariableNode) Pos() Position  { return n.Position }

type CallNode struct {
	Position Position
	Name     string
	Args     []Node
	Option   string
}

func (n *CallNode) Kind() NodeKind { return NodeCall }
func (n *CallNode) Pos() Position  { return n.Position }

type BinaryNode struct {
	Position Position
	Op       string
	Left     Node
	Right    Node
}

func (n *BinaryNode) Kind() NodeKind { return NodeBinary }
func (n *BinaryNode) Pos() Position  { return n.Position }

type GroupNode struct {
	Position Position
	Expr     Node
}

func (n *GroupNode) Kind() NodeKind { return NodeGroup }
func (n *GroupNode) Pos() Position  { return n.Position }

type DependencyRef struct {
	Name        string
	VersionRule string
	Position    Position
	Required    bool
}

type SDKErrorCode string

const (
	ErrParseFailed           SDKErrorCode = "SDK_PARSE_FAILED"
	ErrSemanticInvalid       SDKErrorCode = "SDK_SEMANTIC_INVALID"
	ErrDependencyLoadFailed  SDKErrorCode = "SDK_DEPENDENCY_LOAD_FAILED"
	ErrDependencyCycle       SDKErrorCode = "SDK_DEPENDENCY_CYCLE"
	ErrContextMissing        SDKErrorCode = "SDK_CONTEXT_MISSING"
	ErrInstructionFailed     SDKErrorCode = "SDK_INSTRUCTION_FAILED"
	ErrRuntimeBudgetExceeded SDKErrorCode = "SDK_RUNTIME_BUDGET_EXCEEDED"
	ErrCacheUnavailable      SDKErrorCode = "SDK_CACHE_UNAVAILABLE"
)

type SDKError struct {
	Code     SDKErrorCode
	Message  string
	Position *Position
}

func (e *SDKError) Error() string {
	if e == nil {
		return ""
	}
	if e.Position != nil {
		return fmt.Sprintf("%s: %s at %d", e.Code, e.Message, e.Position.Start)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type ParseOptions struct {
	MaxExpressionLength int
	MaxDepth            int
}

type ParseRequest struct {
	Expression string
	Options    ParseOptions
}

type ParseResult struct {
	AST          Node
	Tokens       []Token
	Dependencies []DependencyRef
	Warnings     []string
}

type Content struct {
	Name    string
	Version string
	Text    string
}

type ContentLoader interface {
	LoadPrompt(ctx context.Context, ref DependencyRef) (Content, error)
}

type ContentLoaderFunc func(ctx context.Context, ref DependencyRef) (Content, error)

func (f ContentLoaderFunc) LoadPrompt(ctx context.Context, ref DependencyRef) (Content, error) {
	return f(ctx, ref)
}

type InstructionExecutor interface {
	ExecuteInstruction(ctx context.Context, req InstructionRequest) (InstructionResult, error)
}

type InstructionExecutorFunc func(ctx context.Context, req InstructionRequest) (InstructionResult, error)

func (f InstructionExecutorFunc) ExecuteInstruction(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
	return f(ctx, req)
}

type InstructionRequest struct {
	Name   string
	Inputs []string
	Option string
}

type InstructionResult struct {
	Content    string
	TokenUsage TokenUsage
	Warnings   []string
}

type TokenUsage struct {
	Input  int
	Output int
	Total  int
}

type ErrorStrategy string

const (
	ErrorStrict  ErrorStrategy = "STRICT"
	ErrorLenient ErrorStrategy = "LENIENT"
)

type OutputForm string

const (
	OutputText     OutputForm = "TEXT"
	OutputMessages OutputForm = "MESSAGES"
	OutputTrace    OutputForm = "TRACE"
	OutputAST      OutputForm = "AST"
)

type RuntimeOptions struct {
	ErrorStrategy ErrorStrategy
	Output        OutputForm
	MaxDepth      int
	MaxSteps      int
}

type RunRequest struct {
	Expression          string
	AST                 Node
	Context             map[string]string
	ContentLoader       ContentLoader
	InstructionExecutor InstructionExecutor
	Options             RuntimeOptions
}

type TraceStep struct {
	Kind   NodeKind
	Name   string
	Output string
}

type Calculation struct {
	TokenUsage TokenUsage
	Steps      int
}

type RunResult struct {
	Content      string
	Dependencies []DependencyRef
	Trace        []TraceStep
	Calculation  Calculation
	Warnings     []string
}

func New() *SDK {
	return &SDK{}
}

type SDK struct{}

func tokenEstimate(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return len(strings.Fields(text))
}
