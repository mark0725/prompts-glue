import { useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { ArrowDown, ArrowUp, CheckCircle2, Code2, Download, Plus, ShieldAlert, Sparkles, Trash2 } from "lucide-react";
import { api } from "@/api";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Button, Card, Input, Modal, Spinner, Textarea } from "@/components/ui";
import { MarkdownEditor } from "@/components/MarkdownEditor";
import { Select } from "@/components/Select";
import { cn } from "@/lib/cn";
import { copyToClipboard } from "@/lib/clipboard";
import type { ASTNode, Dependency, GenerateResponse } from "@/types";

type SourceVal = { src: "ref" | "text"; value: string; versionStrategy: string; version: string };
type Stage =
  | { id: string; kind: "transform"; instr: string; option: string; args: SourceVal[] }
  | { id: string; kind: "combine"; op: "+" | "|" | "#"; operand: SourceVal };

const OP_LABEL: Record<string, string> = { "+": "汇总 +", "|": "合并 |", "#": "标签 #" };

let idSeq = 0;
const newId = () => `b${Date.now()}_${++idSeq}`;

function bumpVersion(v: string): string {
  const parts = v.split(".");
  if (parts.length >= 3 && /^\d+$/.test(parts[2])) {
    parts[2] = String(parseInt(parts[2]) + 1);
    return parts.join(".");
  }
  return v + ".1";
}

// AST → 图形块：将解析后的表达式树映射为工作台的 Stage 列表
const OP_TO_INSTR: Record<string, string> = { "+": "SYNTHESIS", "|": "MERGE", "#": "TAGS" };
const RULE_PREFIX: Record<string, string> = { "^": "CARET", "~": "TILDE", ">=": "GTE", "<": "LT" };

function astToSourceVal(node: ASTNode): SourceVal | null {
  if (node.kind === "prompt_ref") {
    const rule = node.version_rule || "";
    let strategy = "LATEST";
    let version = "";
    if (rule && rule !== "latest" && rule !== "*") {
      version = rule;
      for (const [prefix, strat] of Object.entries(RULE_PREFIX)) {
        if (rule.startsWith(prefix)) { strategy = strat; version = rule.slice(prefix.length); break; }
      }
      if (strategy === "LATEST") { strategy = "EXACT"; }
    }
    return { src: "ref", value: node.name || "", versionStrategy: strategy, version };
  }
  if (node.kind === "string") {
    return { src: "text", value: node.value || "", versionStrategy: "LATEST", version: "" };
  }
  return null;
}

function astToBlocks(node: ASTNode): Stage[] {
  if (node.kind === "binary") {
    const instr = OP_TO_INSTR[node.op || "+"] || "SYNTHESIS";
    return astToBlocks({ kind: "call", name: instr.toLowerCase(), args: [node.left!, node.right!], option: "" });
  }
  if (node.kind === "group" && node.expr) {
    return astToBlocks(node.expr);
  }
  if (node.kind !== "call") return [];

  const args = node.args ?? [];
  let blocks: Stage[] = [];
  let argStart = 0;

  // 第一个参数若为复杂表达式（call/binary），递归解析为前置块
  if (args.length > 0 && (args[0].kind === "call" || args[0].kind === "binary")) {
    blocks.push(...astToBlocks(args[0]));
    argStart = 1;
  }

  // 剩余参数 → 显式输入
  const extraArgs = args.slice(argStart).map(astToSourceVal).filter((v): v is SourceVal => v !== null);
  blocks.push({ id: newId(), kind: "transform", instr: (node.name || "").toUpperCase(), option: node.option || "", args: extraArgs });
  return blocks;
}

function esc(s: string) {
  return '"' + s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n") + '"';
}
const VERSION_PREFIX: Record<string, string> = { EXACT: "", CARET: "^", TILDE: "~", GTE: ">=", LT: "<" };
function srcExpr(s: SourceVal): string {
  if (s.src === "text") return esc(s.value);
  if (!s.value) return s.value;
  if (!s.versionStrategy || s.versionStrategy === "LATEST" || !s.version) return s.value;
  const prefix = VERSION_PREFIX[s.versionStrategy] ?? "";
  return `${s.value}@${prefix}${s.version}`;
}
function synth(stages: Stage[]): string {
  if (stages.length === 0) return "";
  let expr = "";
  for (let i = 0; i < stages.length; i++) {
    const st = stages[i];
    if (st.kind === "transform") {
      const name = st.instr.toLowerCase();
      const args = st.args.map(srcExpr);
      const allArgs = i === 0 ? args : [expr, ...args];
      const call = `${name}(${allArgs.join(", ")})`;
      expr = st.option ? `${call}{${esc(st.option)}}` : call;
    } else if (st.kind === "combine") {
      if (i === 0) { expr = srcExpr(st.operand); }
      else { expr = `${expr} ${st.op} ${srcExpr(st.operand)}`; }
    }
  }
  return expr;
}

export function WorkbenchPage() {
  const instructions = useDictStore((s) => s.dicts["prompt_instruction"] ?? []);
  const dictItems = useDictStore((s) => s.items);
  const { user } = useAuthStore();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const editName = searchParams.get("edit");
  const editVersion = searchParams.get("version") ?? "";

  const [mode, setMode] = useState<"plain" | "composite">("plain");
  const [view, setView] = useState<"graphic" | "text">("graphic");
  const [plainContent, setPlainContent] = useState("");
  const [stages, setStages] = useState<Stage[]>([]);
  const [textExpr, setTextExpr] = useState("");
  const [publishMode, setPublishMode] = useState(true);

  const [errorStrategy, setErrorStrategy] = useState("STRICT");
  const [generating, setGenerating] = useState(false);
  const [result, setResult] = useState<GenerateResponse | null>(null);
  const [deps, setDeps] = useState<Dependency[] | null>(null);

  const [pkg, setPkg] = useState({ name: "", version: "0.1.0", description: "", category: "", tags: "", visibility: "PUBLIC" });
  const [tagInput, setTagInput] = useState("");
  const [publishing, setPublishing] = useState(false);
  const [validation, setValidation] = useState<{ ok: boolean; deps: Dependency[]; warnings?: string[] } | null>(null);
  const [picker, setPicker] = useState<{ pick: ((name: string) => void) | null }>({ pick: null });

  const graphicExpr = useMemo(() => stages.length === 0 ? (textExpr || "") : synth(stages), [stages, textExpr]);
  const expression = mode === "plain" ? esc(plainContent) : (view === "graphic" ? graphicExpr : textExpr);
  const isComposite = mode === "composite";
  const detectedType = isComposite ? "COMPOSITE" : "PLAIN";
  const typeLabel = dictItems("prompt_type").find((i) => i.value === detectedType)?.label ?? detectedType;

  function switchView(v: "graphic" | "text") {
    if (v === "text") setTextExpr(graphicExpr);
    setView(v);
  }
  function addTransform(instr: string) { setView("graphic"); setStages((s) => [...s, { id: newId(), kind: "transform", instr, option: "", args: [] }]); }
  function addCombine(op: "+" | "|" | "#") { setView("graphic"); setStages((s) => [...s, { id: newId(), kind: "combine", op, operand: { src: "text", value: "", versionStrategy: "LATEST", version: "" } }]); }
  function removeStage(id: string) { setStages((s) => s.filter((x) => x.id !== id)); }
  function move(id: string, dir: -1 | 1) {
    setStages((s) => { const i = s.findIndex((x) => x.id === id); const j = i + dir; if (i < 0 || j < 0 || j >= s.length) return s; const c = [...s]; [c[i], c[j]] = [c[j], c[i]]; return c; });
  }
  function setStage(id: string, patch: Partial<Stage>) { setStages((s) => s.map((x) => (x.id === id ? ({ ...x, ...patch } as Stage) : x))); }
  function addArg(id: string) { setStages((s) => s.map((x) => x.id === id && x.kind === "transform" ? { ...x, args: [...x.args, { src: "text", value: "", versionStrategy: "LATEST", version: "" }] } : x)); }
  function setArg(id: string, idx: number, patch: Partial<SourceVal>) {
    setStages((s) => s.map((x) => {
      if (x.id !== id || x.kind !== "transform") return x;
      const args = x.args.map((a, i) => (i === idx ? { ...a, ...patch } : a));
      return { ...x, args };
    }));
  }
  function removeArg(id: string, idx: number) {
    setStages((s) => s.map((x) => {
      if (x.id !== id || x.kind !== "transform") return x;
      return { ...x, args: x.args.filter((_, i) => i !== idx) };
    }));
  }
  function clearAll() {
    setPlainContent(""); setStages([]); setTextExpr(""); setResult(null); setDeps(null); setValidation(null);
  }

  // 编辑模式：加载已有版本内容和表达式
  useEffect(() => {
    if (!editName || !editVersion) return;
    let active = true;
    api.version(editName, editVersion).then(async (v) => {
      if (!active) return;
      // 加载内容
      try {
        const c = await api.content(editName, editVersion, true);
        if (active) setPlainContent(c.content || "");
      } catch { /* ignore */ }
      // 设置模式
      const isComp = v.package.type === "COMPOSITE";
      setMode(isComp ? "composite" : "plain");
      if (isComp && v.package.expression) {
        setTextExpr(v.package.expression);
        // 反向解析表达式为图形块
        try {
          const parsed = await api.parseExpression(v.package.expression);
          if (active && parsed.ast) {
            const blocks = astToBlocks(parsed.ast);
            if (blocks.length > 0) setStages(blocks);
          }
        } catch { /* 解析失败则保持文本模式 */ }
      }
      // 预填属性
      const next = bumpVersion(editVersion);
      setPkg({
        name: editName,
        version: next,
        description: v.package.description || "",
        category: v.package.category || "",
        tags: (v.package.tags ?? []).join(","),
        visibility: "PUBLIC",
      });
    }).catch((e) => toast(e.message, "error"));
    return () => { active = false; };
  }, [editName, editVersion]);

  async function run() {
    if (!expression.trim()) { toast("请先构建表达式", "error"); return; }
    setGenerating(true); setResult(null); setDeps(null);
    try {
      const res = await api.generate({ expression, error_strategy: errorStrategy });
      setResult(res);
      api.resolveDependencies(expression).then((d) => setDeps(d.dependencies ?? [])).catch(() => setDeps([]));
    } catch (e: any) { toast(e.message, "error"); }
    finally { setGenerating(false); }
  }

  async function publish(dryRun: boolean) {
    if (!user) { toast("请先登录", "error"); navigate("/login"); return; }
    if (!pkg.name.trim() || !pkg.version.trim() || !pkg.description.trim() || !pkg.category) { toast("请填写包名、版本、说明与分类", "error"); return; }
    if (isComposite) {
      if (!result) { toast("请先执行生成后再发布", "error"); return; }
      if (!expression.trim()) { toast("组合提示词表达式为空", "error"); return; }
    }
    setPublishing(true);
    try {
      const content = isComposite ? (result?.content || "") : plainContent;
      const meta = {
        name: pkg.name, version: pkg.version, description: pkg.description, category: pkg.category, type: detectedType,
        tags: pkg.tags.split(",").map((t) => t.trim()).filter(Boolean), author: user?.display_name || user?.username || undefined,
        visibility: pkg.visibility, expression: isComposite ? expression : "",
      };
      const res = await api.publish(meta, content, dryRun);
      if (dryRun) { setValidation({ ok: true, deps: res.dependencies, warnings: res.warnings }); toast("校验通过", "success"); }
      else { toast("发布成功", "success"); navigate(`/packages/${encodeURIComponent(pkg.name)}`); }
    } catch (e: any) { if (dryRun) setValidation({ ok: false, deps: [] }); toast(e.message, "error"); }
    finally { setPublishing(false); }
  }

  return (
    <div className={cn("space-y-4", publishMode && "lg:pr-[392px]")}>
      {/* 工具栏 */}
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-xl font-bold text-foreground">{editName ? `编辑: ${editName}` : "创建提示词"}</h1>
        {/* 模式 Tab */}
        <div className="flex rounded-md border p-0.5 text-xs">
          <button className={cn("rounded px-3 py-1", mode === "plain" ? "bg-primary text-primary-foreground" : "text-muted-foreground")} onClick={() => setMode("plain")}>普通提示词</button>
          <button className={cn("rounded px-3 py-1", mode === "composite" ? "bg-primary text-primary-foreground" : "text-muted-foreground")} onClick={() => setMode("composite")}>复合提示词</button>
        </div>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          {mode === "composite" && (
            <div className="flex rounded-md border p-0.5 text-xs">
              <button className={cn("rounded px-2 py-1", view === "graphic" ? "bg-primary text-primary-foreground" : "text-muted-foreground")} onClick={() => switchView("graphic")}>图形</button>
              <button className={cn("rounded px-2 py-1", view === "text" ? "bg-primary text-primary-foreground" : "text-muted-foreground")} onClick={() => switchView("text")}>文本</button>
            </div>
          )}
          <label className="flex items-center gap-1 text-xs text-muted-foreground">
            <input type="checkbox" checked={publishMode} onChange={(e) => setPublishMode(e.target.checked)} disabled={!user && !publishMode} /> 属性面板
          </label>
          <Button size="sm" variant="ghost" onClick={clearAll}>清空</Button>
          {mode === "composite" && <Button size="sm" onClick={run} disabled={generating}><Sparkles className="h-4 w-4" /> {generating ? "生成中…" : "生成提示词"}</Button>}
        </div>
      </div>

      {mode === "plain" ? (
        /* 普通提示词：Markdown 所见即所得编辑器 */
        <Card className="flex h-[calc(100dvh-8.5rem)] min-h-[460px] flex-col p-4">
          <div className="mb-2 flex shrink-0 items-center justify-between gap-2">
            <div className="text-xs text-muted-foreground">提示词内容（Markdown 所见即所得）</div>
            <div className="text-[11px] text-muted-foreground">支持标题、列表、代码块、表格等；左侧编辑，右侧实时预览</div>
          </div>
          <MarkdownEditor
            value={plainContent}
            onChange={setPlainContent}
            placeholder={"输入提示词 Markdown 内容…\n\n## 示例\n- 支持 **加粗** / *斜体*\n- 列表、代码块、表格与链接"}
            preview="live"
          />
        </Card>
      ) : (
        /* 复合提示词：完整编辑器 */
        <>
          <div className="grid gap-4 lg:grid-cols-[200px_1fr]">
            {/* 左栏：区块面板 + 执行配置 */}
            <div className="space-y-4 self-start lg:sticky lg:top-20">
              {view === "graphic" && (
                <Card className="p-3">
                  <Palette title="制作指令" />
                  <div className="flex flex-wrap gap-1.5">
                    {instructions.map((i) => (
                      <button key={i.value} onClick={() => addTransform(i.value)} className="rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-accent" title={i.description}>{i.label}</button>
                    ))}
                  </div>
                  <Palette title="组合" />
                  <div className="flex flex-wrap gap-1.5">
                    {(["+", "|", "#"] as const).map((op) => (
                      <button key={op} onClick={() => addCombine(op)} className="rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-accent">{OP_LABEL[op]}</button>
                    ))}
                  </div>
                </Card>
              )}
              <Card className="p-3">
                <h2 className="mb-2 text-xs font-semibold text-foreground">执行配置</h2>
                <div className="space-y-2">
                  <div>
                    <span className="mb-1 block text-xs text-muted-foreground">错误策略</span>
                    <Select value={errorStrategy} onChange={setErrorStrategy} options={dictItems("error_strategy").map((i) => ({ value: i.value, label: i.label }))} />
                  </div>
                  <p className="text-[11px] text-muted-foreground">缓存策略由服务端配置统一管理，不在界面设置。</p>
                </div>
              </Card>
            </div>

            {/* 构建区 */}
            <div className="space-y-3">
              {view === "graphic" ? (
                <Card className="p-4 min-h-[70vh]">
                  <div className="mb-2 text-xs text-muted-foreground">数据流（左 → 右）</div>
                  <div className="flex flex-wrap items-start gap-2">
                    {stages.map((st, i) => (
                      <div key={st.id} className="flex items-start gap-2">
                        {i > 0 && <span className="mt-6 text-muted-foreground">→</span>}
                        {st.kind === "transform" && (
                          <BlockCard color="purple" title={instructions.find((x) => x.value === st.instr)?.label ?? st.instr} onRemove={() => removeStage(st.id)} onUp={() => move(st.id, -1)} onDown={() => move(st.id, 1)} canUp={i > 0} canDown={i < stages.length - 1}>
                            <div className="space-y-2">
                              <Textarea rows={2} className="min-h-[56px] resize-y text-xs" placeholder="指令参数（可空）" value={st.option} onChange={(e) => setStage(st.id, { option: e.target.value } as Partial<Stage>)} />
                              <div className="text-[10px] font-medium uppercase text-muted-foreground">输入 {st.args.length > 0 ? `+${st.args.length}` : ""}</div>
                              {st.args.map((arg, ai) => (
                                <div key={ai} className="rounded-md border border-border/70 p-1.5">
                                  <div className="mb-1 flex items-center justify-between">
                                    <span className="text-[10px] text-muted-foreground">输入 {ai + 1}</span>
                                    <button onClick={() => removeArg(st.id, ai)} className="rounded p-0.5 text-muted-foreground hover:bg-black/10"><Trash2 className="h-3 w-3" /></button>
                                  </div>
                                  <SourceEditor value={arg} onChange={(v) => setArg(st.id, ai, v)} onPickRef={() => setPicker({ pick: (name) => setArg(st.id, ai, { src: "ref", value: name, versionStrategy: "LATEST" }) })} textRows={3} textPlaceholder="文本" />
                                </div>
                              ))}
                              <Button size="sm" variant="outline" className="w-full" onClick={() => addArg(st.id)}><Plus className="h-3.5 w-3.5" /> 增加输入</Button>
                            </div>
                          </BlockCard>
                        )}
                        {st.kind === "combine" && (
                          <BlockCard color="green" title={OP_LABEL[st.op]} onRemove={() => removeStage(st.id)} onUp={() => move(st.id, -1)} onDown={() => move(st.id, 1)} canUp={i > 0} canDown={i < stages.length - 1}>
                            <SourceEditor value={st.operand} onChange={(v) => setStage(st.id, { operand: v } as Partial<Stage>)} onPickRef={() => setPicker({ pick: (name) => setStage(st.id, { operand: { src: "ref", value: name, versionStrategy: "LATEST", version: "" } } as Partial<Stage>) })} textRows={3} textPlaceholder="文本" />
                          </BlockCard>
                        )}
                      </div>
                    ))}
                    {stages.length === 0 && textExpr ? (
                      <div className="space-y-3 py-6">
                        <p className="text-sm text-muted-foreground">已加载表达式（编辑模式，可切换到文本视图修改）</p>
                        <pre className="overflow-x-auto rounded-md bg-muted/60 p-3 font-mono text-xs text-foreground">{textExpr}</pre>
                        <Button variant="outline" size="sm" onClick={() => switchView("text")}>切换到文本视图编辑</Button>
                      </div>
                    ) : stages.length === 0 ? (
                      <p className="py-10 text-center text-sm text-muted-foreground">从左侧面板添加指令或组合区块</p>
                    ) : null}
                  </div>
                </Card>
              ) : (
                <Card className="p-4 min-h-[70vh]">
                  <div className="mb-2 flex items-center gap-1 text-xs text-muted-foreground"><Code2 className="h-3.5 w-3.5" /> 表达式（文本模式）</div>
                  <Textarea rows={20} className="min-h-[60vh] resize-y font-mono text-xs" placeholder='例如：summary(browser-test-prompt + "补充")' value={textExpr} onChange={(e) => setTextExpr(e.target.value)} />
                </Card>
              )}
              {/* 表达式预览 */}
              <Card className="p-3">
                <div className="mb-1 text-xs text-muted-foreground">当前表达式</div>
                <pre className="overflow-x-auto rounded-md bg-muted/60 p-2 font-mono text-xs text-foreground">{expression || "（空）"}</pre>
              </Card>
            </div>
          </div>

          {/* 结果区 */}
          <div className="grid gap-4 md:grid-cols-2">
            <Card className="p-4">
              <h2 className="mb-2 text-sm font-semibold text-foreground">生成结果</h2>
              {!result ? (
                <p className="text-sm text-muted-foreground">构建后点击「生成提示词」</p>
              ) : (
                <div className="space-y-2">
                  <div className="max-h-72 overflow-auto rounded-md bg-muted/40 p-3"><pre className="whitespace-pre-wrap break-words text-sm text-foreground">{result.content}</pre></div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <Badge color={result.calculation.cache_hit ? "green" : "slate"}>{result.calculation.cache_hit ? "缓存命中" : "实时计算"}</Badge>
                    {result.calculation.token_usage && <span>token {result.calculation.token_usage.input}/{result.calculation.token_usage.output}/{result.calculation.token_usage.total}</span>}
                    <button className="hover:text-foreground" onClick={() => copyToClipboard(result.content).then((ok) => ok && toast("已复制", "success"))}><Download className="inline h-3.5 w-3.5" /> 复制</button>
                  </div>
                  {result.warnings?.length ? <p className="text-xs text-amber-600">{result.warnings.join("；")}</p> : null}
                </div>
              )}
            </Card>
            <Card className="p-4">
              <h2 className="mb-2 text-sm font-semibold text-foreground">依赖关系图</h2>
              {!deps ? <p className="text-sm text-muted-foreground">执行后展示</p>
                : deps.length === 0 ? <p className="text-sm text-muted-foreground">无依赖</p>
                : <DepGraph deps={deps} rootLabel={`当前${typeLabel}`} />}
            </Card>
          </div>
        </>
      )}

      {/* 属性面板：右侧抽屉 */}
      {publishMode && (
        <aside className="fixed right-0 top-16 bottom-0 z-40 flex w-[380px] max-w-[92vw] flex-col border-l bg-background shadow-lg">
          <div className="flex items-center justify-between border-b p-3">
            <h2 className="text-sm font-semibold text-foreground">提示词属性</h2>
            <button onClick={() => setPublishMode(false)} className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground" title="关闭">✕</button>
          </div>
          <div className="flex-1 overflow-y-auto p-3">
            {!user ? (
              <p className="text-sm text-muted-foreground">发布需先<button className="mx-1 text-primary underline" onClick={() => navigate("/login")}>登录</button></p>
            ) : (
              <>
                <div className="mb-3 flex items-center gap-2 text-xs">
                  <span className="text-muted-foreground">类型：</span>
                  <Badge color={isComposite ? "purple" : "blue"}>{typeLabel}</Badge>
                  <span className="text-muted-foreground">{isComposite ? "按表达式计算" : "人工文本"}</span>
                </div>
                <div className="grid gap-3">
                  <Field label="包名"><Input value={pkg.name} onChange={(e) => setPkg({ ...pkg, name: e.target.value })} placeholder="my-prompt" /></Field>
                  <Field label="版本"><Input value={pkg.version} onChange={(e) => setPkg({ ...pkg, version: e.target.value })} /></Field>
                  <Field label="说明"><Input value={pkg.description} onChange={(e) => setPkg({ ...pkg, description: e.target.value })} /></Field>
                  <Field label="分类"><Select value={pkg.category} onChange={(v) => setPkg({ ...pkg, category: v })} options={[{ value: "", label: "请选择" }, ...dictItems("prompt_category").map((i) => ({ value: i.value, label: i.label }))]} /></Field>
                  <Field label="可见性"><Select value={pkg.visibility} onChange={(v) => setPkg({ ...pkg, visibility: v })} options={dictItems("visibility").map((i) => ({ value: i.value, label: i.label }))} /></Field>
                  <div>
                    <span className="mb-1 block text-xs text-muted-foreground">标签</span>
                    <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-input bg-background p-1.5">
                      {pkg.tags.split(",").map((t) => t.trim()).filter(Boolean).map((tag, i) => (
                        <span key={i} className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs">
                          {tag}
                          <button onClick={() => setPkg({ ...pkg, tags: pkg.tags.split(",").map((t) => t.trim()).filter(Boolean).filter((_, j) => j !== i).join(",") })} className="text-muted-foreground hover:text-foreground">✕</button>
                        </span>
                      ))}
                      <input
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === ",") {
                            e.preventDefault();
                            const val = tagInput.trim();
                            if (val) {
                              const current = pkg.tags.split(",").map((t) => t.trim()).filter(Boolean);
                              if (!current.includes(val)) setPkg({ ...pkg, tags: [...current, val].join(",") });
                              setTagInput("");
                            }
                          } else if (e.key === "Backspace" && !tagInput && pkg.tags) {
                            const current = pkg.tags.split(",").map((t) => t.trim()).filter(Boolean);
                            setPkg({ ...pkg, tags: current.slice(0, -1).join(",") });
                          }
                        }}
                        placeholder={pkg.tags ? "" : "输入标签后回车"}
                        className="min-w-[80px] flex-1 bg-transparent px-1 text-xs outline-none"
                      />
                    </div>
                  </div>
                </div>
                {validation && (
                  <div className="mt-3 rounded-md border p-2 text-xs">
                    {validation.ok ? (
                      <div className="flex items-center gap-1.5 text-emerald-600"><CheckCircle2 className="h-4 w-4" /> 校验通过 · 依赖 {validation.deps.length} 个{validation.warnings?.length ? <span className="ml-2 flex items-center gap-1 text-amber-600"><ShieldAlert className="h-4 w-4" /> {validation.warnings.join("；")}</span> : null}</div>
                    ) : <p className="text-red-600">校验未通过，请检查字段</p>}
                  </div>
                )}
              </>
            )}
          </div>
          {user && (
            <div className="border-t p-3 flex justify-end gap-2">
              <Button variant="outline" onClick={() => publish(true)} disabled={publishing}>发布前校验</Button>
              <Button onClick={() => publish(false)} disabled={publishing}>{publishing ? "发布中…" : "发布"}</Button>
            </div>
          )}
        </aside>
      )}

      {picker.pick && <PackagePicker onPick={(name) => { picker.pick?.(name); setPicker({ pick: null }); }} onClose={() => setPicker({ pick: null })} />}
    </div>
  );
}

function Palette({ title }: { title: string }) {
  return <h3 className="mb-1.5 mt-3 text-xs font-semibold text-muted-foreground first:mt-0">{title}</h3>;
}

const colorCls: Record<string, string> = {
  purple: "border-purple-300 bg-purple-50", green: "border-emerald-300 bg-emerald-50",
};

function BlockCard({ color, title, children, onRemove, onUp, onDown, canUp, canDown }: {
  color: keyof typeof colorCls; title: string; children: React.ReactNode;
  onRemove?: () => void; onUp?: () => void; onDown?: () => void; canUp?: boolean; canDown?: boolean;
}) {
  return (
    <div className={cn("w-72 rounded-md border p-2", colorCls[color])}>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="text-xs font-semibold text-foreground">{title}</span>
        {onRemove && (
          <div className="flex items-center gap-0.5">
            {onUp && <button disabled={!canUp} onClick={onUp} className="rounded p-0.5 text-muted-foreground hover:bg-black/5 disabled:opacity-30"><ArrowUp className="h-3 w-3" /></button>}
            {onDown && <button disabled={!canDown} onClick={onDown} className="rounded p-0.5 text-muted-foreground hover:bg-black/5 disabled:opacity-30"><ArrowDown className="h-3 w-3" /></button>}
            <button onClick={onRemove} className="rounded p-0.5 text-muted-foreground hover:bg-black/5"><Trash2 className="h-3 w-3" /></button>
          </div>
        )}
      </div>
      {children}
    </div>
  );
}

function SourceEditor({ value, onChange, onPickRef, textRows = 3, textPlaceholder = "文本内容" }: {
  value: SourceVal; onChange: (v: SourceVal) => void; onPickRef: () => void; textRows?: number; textPlaceholder?: string;
}) {
  const versionRules = useDictStore((s) => s.dicts["version_rule"] ?? []);
  const [versions, setVersions] = useState<string[]>([]);
  const strategy = value.versionStrategy || "LATEST";
  useEffect(() => {
    if (value.src === "ref" && value.value) {
      let active = true;
      api.package(value.value).then((r) => { if (active) setVersions((r.versions ?? []).map((v) => v.version)); }).catch(() => { if (active) setVersions([]); });
      return () => { active = false; };
    }
    setVersions([]);
  }, [value.src, value.value]);
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex gap-1">
        <button className={cn("rounded px-2 py-0.5 text-xs", value.src === "ref" ? "bg-primary text-primary-foreground" : "bg-muted")} onClick={() => onChange({ ...value, src: "ref", versionStrategy: strategy })}>引用</button>
        <button className={cn("rounded px-2 py-0.5 text-xs", value.src === "text" ? "bg-primary text-primary-foreground" : "bg-muted")} onClick={() => onChange({ ...value, src: "text" })}>文本</button>
      </div>
      {value.src === "ref" ? (
        <div className="space-y-1.5">
          <div className="flex items-center gap-1">
            <Input className="h-8 font-mono text-xs" placeholder="包名" value={value.value} onChange={(e) => onChange({ ...value, value: e.target.value })} />
            <Button size="sm" variant="ghost" onClick={onPickRef}><Plus className="h-3.5 w-3.5" /></Button>
          </div>
          <div>
            <span className="mb-1 block text-[10px] text-muted-foreground">版本策略</span>
            <Select value={strategy} onChange={(v) => onChange({ ...value, versionStrategy: v, version: v === "LATEST" ? "" : value.version })} options={versionRules.map((i) => ({ value: i.value, label: i.label }))} />
          </div>
          {strategy !== "LATEST" && (
            <div>
              <span className="mb-1 block text-[10px] text-muted-foreground">版本</span>
              <Select value={value.version} onChange={(v) => onChange({ ...value, version: v })} placeholder={versions.length ? "选择版本" : "无可用版本"} options={versions.map((v) => ({ value: v, label: v }))} />
            </div>
          )}
        </div>
      ) : (
        <Textarea rows={textRows} className="min-h-[88px] resize-y text-xs" placeholder={textPlaceholder} value={value.value} onChange={(e) => onChange({ ...value, value: e.target.value })} />
      )}
    </div>
  );
}

function DepGraph({ deps, rootLabel }: { deps: Dependency[]; rootLabel: string }) {
  return (
    <div className="space-y-1 text-xs">
      <div className="rounded-md border border-primary/40 bg-primary/10 px-2 py-1 font-medium text-primary">{rootLabel}（根）</div>
      {deps.map((d) => (
        <div key={d.name} className="flex items-center gap-2 pl-4">
          <span className="text-muted-foreground">└→</span>
          <span className="font-mono text-foreground">{d.name}</span>
          {d.resolved_version ? <Badge color="blue">@{d.resolved_version}</Badge> : <Badge color="slate">未解析</Badge>}
        </div>
      ))}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="mb-1 block text-xs text-muted-foreground">{label}</span>{children}</label>;
}

function PackagePicker({ onPick, onClose }: { onPick: (name: string) => void; onClose: () => void }) {
  const [q, setQ] = useState("");
  const [items, setItems] = useState<{ name: string; description: string }[]>([]);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    setLoading(true);
    const t = setTimeout(() => { api.searchPackages({ q, page: 1, page_size: 20 }).then((r) => setItems(r.items ?? [])).catch(() => setItems([])).finally(() => setLoading(false)); }, 250);
    return () => clearTimeout(t);
  }, [q]);
  return (
    <Modal open onClose={onClose} title="选择提示词">
      <div className="space-y-2">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索包名、说明或标签" autoFocus />
        <div className="max-h-72 overflow-y-auto">
          {loading ? <div className="p-4 text-center"><Spinner /></div> : items.length === 0 ? <p className="p-4 text-center text-sm text-muted-foreground">无匹配提示词</p> : items.map((it) => (
            <button key={it.name} onClick={() => onPick(it.name)} className="block w-full rounded-md px-2 py-1.5 text-left hover:bg-accent">
              <div className="font-mono text-xs text-primary">{it.name}</div>
              <div className="truncate text-xs text-muted-foreground">{it.description}</div>
            </button>
          ))}
        </div>
      </div>
    </Modal>
  );
}
