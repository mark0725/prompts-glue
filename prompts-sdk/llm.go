package promptssdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LLMConfig 描述 SDK 内置 LLM 执行器的连接与行为配置。
type LLMConfig struct {
	Model         string
	APIBase       string
	APIKey        string
	Proxy         string
	Timeout       time.Duration
	SystemPrompts map[string]string // 按指令名（小写）覆盖内置系统提示词
}

// NewLLMExecutor 构建一个调用 OpenAI 兼容 /chat/completions 的指令执行器。
// Model 与 APIBase 必填；APIKey 为空时不带鉴权头（适配无需鉴权的本地模型服务）。
func NewLLMExecutor(cfg LLMConfig) (InstructionExecutor, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("llm model is required")
	}
	if strings.TrimSpace(cfg.APIBase) == "" {
		return nil, fmt.Errorf("llm api_base is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	transport := &http.Transport{}
	if strings.TrimSpace(cfg.Proxy) != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid llm proxy: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	return &llmExecutor{cfg: cfg, client: client}, nil
}

type llmExecutor struct {
	cfg    LLMConfig
	client *http.Client
}

func (e *llmExecutor) ExecuteInstruction(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
	payload, err := json.Marshal(map[string]any{
		"model":  e.cfg.Model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": e.systemPrompt(req.Name)},
			{"role": "user", "content": buildLLMUserMessage(req.Option, req.Inputs)},
		},
	})
	if err != nil {
		return InstructionResult{}, err
	}

	endpoint := strings.TrimRight(e.cfg.APIBase, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return InstructionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if e.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return InstructionResult{}, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return InstructionResult{}, fmt.Errorf("llm returned status %d", resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return InstructionResult{}, fmt.Errorf("llm response parse failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return InstructionResult{}, fmt.Errorf("llm returned no choices")
	}
	return InstructionResult{
		Content: strings.TrimSpace(parsed.Choices[0].Message.Content),
		TokenUsage: TokenUsage{
			Input:  parsed.Usage.PromptTokens,
			Output: parsed.Usage.CompletionTokens,
			Total:  parsed.Usage.TotalTokens,
		},
	}, nil
}

// globalSystemPromptPrefix 统一前置指令：要求 LLM 直接输出结果内容，不附加解释/描述/前后缀，
// 便于结果作为下游工具的输入直接使用。应用于所有制作指令的 LLM 调用。
const globalSystemPromptPrefix = "本输出作为其他工具的输入，直接输出结果内容，不要描述。\n\n"

func (e *llmExecutor) systemPrompt(name string) string {
	base := ""
	if p, ok := e.cfg.SystemPrompts[name]; ok && p != "" {
		base = p
	} else if p, ok := defaultSystemPrompts[name]; ok && p != "" {
		base = p
	} else {
		base = "你是一名提示词处理助手，请按指令要求处理输入内容，保留关键约束和事实依据，不得编造输入中不存在的重要信息。"
	}
	return globalSystemPromptPrefix + base
}

// buildLLMUserMessage 组装用户消息：用户特别提示词（指令 {} 内容）作为额外要求前缀，随后拼接输入内容。
func buildLLMUserMessage(option string, inputs []string) string {
	joined := strings.Join(nonEmpty(inputs), "\n")
	if option == "" {
		return joined
	}
	return "额外要求：" + option + "\n\n" + joined
}

// defaultSystemPrompts 为各制作指令内置的系统提示词默认值，取自 docs/design/prompts.md。
// 按指令名（小写）索引，调用方可通过 LLMConfig.SystemPrompts 覆盖。
var defaultSystemPrompts = map[string]string{
	"synthesis": "你是提示词内容整合助手。请综合所有输入内容，保留关键约束、目标、角色、流程和边界，去除冲突与重复，输出结构清晰、可直接使用的完整提示词。不得添加输入中没有依据的重要事实。",
	"keys":      "你是关键词提取助手。请从输入内容中提取最能代表主题、对象、动作、技术和约束的关键词，优先保留原文中的术语，去除泛化词和重复词，输出简洁关键词列表。",
	"tags":      "你是提示词标签生成助手。请根据输入内容生成适合检索、分类和推荐的标签，标签应短小、稳定、可复用，避免过细、重复或带有临时上下文的信息。",
	"labels":    "你是内容标注助手。请为输入内容增加清晰标注，标明主题、用途、适用场景、关键约束和注意事项。标注应帮助后续理解和检索，不改变原文核心含义。",
	"summary":   "你是摘要生成助手。请压缩输入内容并保留核心目标、关键规则、限制条件和结论。摘要应准确、简洁、层次清楚，不引入原文之外的信息。",
	"merge":     "你是提示词合并助手。请按输入顺序合并内容，保留各部分有效信息，消除重复表达，处理明显冲突并说明取舍，输出连贯一致的提示词内容。保持原来的排版结构。",
	"title":     "你是标题生成助手。请为输入内容生成简短、准确、可检索的标题。标题应体现核心主题和用途，避免夸张、含糊或过长。",
	"bullets":   "你是要点整理助手。请将输入内容整理为层次清楚的要点，保留关键动作、约束、决策和注意事项。要点应简洁、可执行、避免重复。",
	"clean":     "你是文本清洗助手。请清理输入中的噪声、无效格式、重复符号和明显无关内容，保留有业务意义的信息。不得改变原文的关键意图和约束。",
	"dedupe":    "你是内容去重助手。请识别完全重复和语义近似重复的内容，保留表达最完整、最准确的一项。输出应保持原有逻辑顺序，并尽量减少信息损失。",
	"topic":     "你是主题提炼助手。请从输入内容中提炼核心主题，主题应准确反映主要对象、目标和适用范围，避免使用过宽泛的概念。",
	"outline":   "你是大纲生成助手。请将输入内容整理为层级清晰的大纲，体现主题、模块、步骤、规则和依赖关系。大纲应便于后续扩写为完整提示词。",
	"lead":      "你是导语生成助手。请为输入内容生成简短导语，说明背景、目标和使用方式。导语应自然、准确，并为后续正文建立清晰上下文。",
	"questions": "你是问题生成助手。请基于输入内容生成有助于澄清需求、发现风险或推进设计的问题。问题应具体、可回答，避免泛泛而谈。",
	"extract":   "你是要素抽取助手。请根据指定要素要求，从输入内容中抽取对应信息。输出应保持要素名称清晰、值准确；缺失要素应明确标记为未提供，不得编造。",
	"template":  "你是提示词模板化助手。请按照指定要求组织输入内容，保留关键约束和语义关系。缺少的信息应保持空缺或标注未提供，不得虚构内容。",
	"format":    "你是内容排版助手。请统一标题层级、列表、段落、代码块和间距，修正格式错误并保持结构清晰。不得改变原文语义、技术术语和关键约束。",
	"translate": "你是专业翻译助手。请将输入内容准确翻译为指定目标语言，保留术语一致性、语义和关键约束；目标语言由指令参数给出。不得增删重要信息或改变原意。",
	"redact":    "你是内容脱敏助手。请识别输入中的个人身份信息、账号、证件、密钥和联系方式等敏感信息并替换为通用占位符。不得保留可还原的原文敏感内容，也不得改变非敏感部分的语义。",
	"refine":    "你是内容提炼助手。请在保留关键约束、目标和事实依据的前提下，提炼输入的核心价值并精炼表达，去除冗余与次要细节。不得编造输入中不存在的重要信息。",
}
