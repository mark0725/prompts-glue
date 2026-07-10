import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ChevronDown, Copy, Download, Pencil } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api } from "@/api";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Button, Card, Spinner } from "@/components/ui";
import { Select } from "@/components/Select";
import { cn } from "@/lib/cn";
import { copyToClipboard } from "@/lib/clipboard";
import type { ContentResponse, PackageDetail, VersionDetail, VersionEvent } from "@/types";

type Tab = "content" | "expression" | "deps" | "events";

function packageFileName(name: string, version: string): string {
  return `${name.replace(/[@/]/g, "-")}-${version}.json`;
}

function contentApiPath(name: string, version: string, query = ""): string {
  const base = `/api/registry/packages/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}/content`;
  return query ? `${base}?${query}` : base;
}

function absoluteApiUrl(path: string): string {
  if (typeof window === "undefined") return path;
  return `${window.location.origin}${path}`;
}

export function PackageDetailPage() {
  const { name = "" } = useParams();
  const decodedName = decodeURIComponent(name);
  const { label } = useDictStore();
  const { user } = useAuthStore();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<PackageDetail | null>(null);
  const [version, setVersion] = useState<VersionDetail | null>(null);
  const [content, setContent] = useState<ContentResponse | null>(null);
  const [contentLoading, setContentLoading] = useState(false);
  const [tab, setTab] = useState<Tab>("content");
  const [loading, setLoading] = useState(true);
  const [downloadOpen, setDownloadOpen] = useState(false);
  const downloadRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setContent(null);
    setDownloadOpen(false);
    api
      .package(decodedName)
      .then((result) => {
        if (!active) return;
        setDetail(result);
        const target = result.latest_version ?? result.versions[0]?.version;
        if (target) api.version(decodedName, target).then(setVersion).catch(() => undefined);
      })
      .catch((e) => { if (active) toast(e.message, "error"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [decodedName]);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (downloadRef.current && !downloadRef.current.contains(e.target as Node)) {
        setDownloadOpen(false);
      }
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  function loadVersion(selected: string) {
    setDownloadOpen(false);
    api.version(decodedName, selected).then(setVersion).catch((e) => toast(e.message, "error"));
  }

  // 切换版本时若当前 Tab 对类型不适用，回落到内容预览
  useEffect(() => {
    const isComposite = version?.package.type === "COMPOSITE" && !!version?.package.expression;
    if (tab === "expression" && !isComposite) {
      setTab("content");
    }
  }, [version?.package.type, version?.package.expression, tab]);

  // 自动加载版本内容（Markdown 预览）
  useEffect(() => {
    const ver = version?.package.version;
    if (!ver) return;
    let active = true;
    setContent(null);
    setContentLoading(true);
    api.content(decodedName, ver, true)
      .then((r) => { if (active) setContent(r); })
      .catch(() => {})
      .finally(() => { if (active) setContentLoading(false); });
    return () => { active = false; };
  }, [decodedName, version?.package.version]);

  function loadContent() {
    if (!version) return;
    api.content(decodedName, version.package.version).then(setContent).catch((e) => toast(e.message, "error"));
  }

  function copyInstall() {
    copyToClipboard(`glue add ${decodedName}`).then((ok) => ok ? toast("已复制安装命令", "success") : toast("复制失败", "error"));
  }

  async function copyText(text: string, successMessage: string) {
    const ok = await copyToClipboard(text);
    toast(ok ? successMessage : "复制失败", ok ? "success" : "error");
    if (ok) setDownloadOpen(false);
  }

  if (loading) return <div className="p-10 text-center"><Spinner /></div>;
  if (!detail) return <Card className="p-10 text-center text-muted-foreground">未找到提示词包</Card>;

  const selectedVersion = version?.package.version ?? detail.latest_version ?? "";
  const deprecated = version?.package.status === "DEPRECATED";
  const packageUrl = selectedVersion ? absoluteApiUrl(contentApiPath(decodedName, selectedVersion)) : "";
  const rawContentUrl = selectedVersion ? absoluteApiUrl(contentApiPath(decodedName, selectedVersion, "format=raw")) : "";
  const curlCommand = selectedVersion
    ? `curl -sS -o ${packageFileName(decodedName, selectedVersion)} "${packageUrl}"`
    : "";

  return (
    <div className="space-y-4">
      {/* 面包屑 */}
      <nav className="flex items-center gap-1 text-sm text-muted-foreground">
        <Link to="/" className="hover:text-foreground">首页</Link>
        <span>/</span>
        <Link to={`/?category=${detail.category}`} className="hover:text-foreground">{label("prompt_category", detail.category)}</Link>
        <span>/</span>
        <span className="text-foreground">{detail.name}</span>
      </nav>

      {/* 包头 */}
      <Card className="p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="font-mono text-xl font-bold text-foreground">{detail.name}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{detail.description}</p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Badge color="blue">{label("prompt_category", detail.category)}</Badge>
              <Badge color="slate">{label("visibility", detail.visibility)}</Badge>
              {detail.tags?.map((t) => <Badge key={t} color="slate">{t}</Badge>)}
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>下载 {detail.versions.length} 个版本</span>
              {detail.versions[0]?.published_at && <span>· 最近发布 {detail.versions[0].published_at.slice(0, 10)}</span>}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onClick={copyInstall}><Copy className="h-4 w-4" /> 安装命令</Button>
            {user && selectedVersion && (
              <Button variant="outline" size="sm" onClick={() => navigate(`/workbench?edit=${encodeURIComponent(decodedName)}&version=${selectedVersion}`)}>
                <Pencil className="h-4 w-4" /> 编辑 / 升级版本
              </Button>
            )}
            <div className="relative" ref={downloadRef}>
              <Button size="sm" onClick={() => setDownloadOpen((open) => !open)} disabled={!selectedVersion}>
                <Download className="h-4 w-4" /> 下载内容
                <ChevronDown className={cn("h-3.5 w-3.5 transition-transform", downloadOpen && "rotate-180")} />
              </Button>
              {downloadOpen && selectedVersion && (
                <div className="absolute right-0 z-[60] mt-1 w-[min(28rem,calc(100vw-2rem))] rounded-md border bg-popover p-3 text-popover-foreground shadow-xl">
                  <div className="space-y-3">
                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-xs font-medium text-foreground">curl 下载提示词包</p>
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => copyText(curlCommand, "已复制 curl 命令")}
                        >
                          <Copy className="h-3.5 w-3.5" /> 复制
                        </button>
                      </div>
                      <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-muted/60 px-2.5 py-2 font-mono text-[11px] leading-relaxed text-foreground">{curlCommand}</pre>
                    </div>
                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-xs font-medium text-foreground">提示内容 URL</p>
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
                          onClick={() => copyText(rawContentUrl, "已复制内容 URL")}
                        >
                          <Copy className="h-3.5 w-3.5" /> 复制
                        </button>
                      </div>
                      <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-muted/60 px-2.5 py-2 font-mono text-[11px] leading-relaxed text-foreground">{rawContentUrl}</pre>
                      <p className="text-[11px] text-muted-foreground">仅返回提示词正文（Markdown），不含 API 外壳与包元数据。</p>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </Card>

      {/* 版本选择 */}
      <div className="flex items-center gap-3">
        <span className="text-sm text-muted-foreground">版本</span>
        <Select
          className="w-48"
          value={selectedVersion}
          onChange={loadVersion}
          options={detail.versions.map((v) => ({ value: v.version, label: `${v.version} · ${label("version_status", v.status)}` }))}
        />
      </div>

      {/* 废弃提示 */}
      {deprecated && (
        <div className="flex items-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <span>该版本已废弃。</span>
          {version?.package.version && <span>原因：请联系维护者。</span>}
        </div>
      )}

      {/* 版本详情 */}
      {version && (
        <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
          {/* 元信息：仅短字段，长文本内容不放左侧 */}
          <Card className="h-fit p-4">
            <h2 className="mb-3 text-sm font-semibold text-foreground">版本信息</h2>
            <dl className="space-y-2 text-sm">
              <Row k="类型" v={label("prompt_type", version.package.type)} />
              <Row k="版本" v={version.package.version} mono />
              <Row k="状态" v={<Badge color={deprecated ? "yellow" : "green"}>{label("version_status", version.package.status)}</Badge>} />
              <Row k="发布者" v={version.published_by || "-"} />
              <Row k="发布时间" v={(version.published_at || "").slice(0, 16).replace("T", " ") || "-"} />
            </dl>
          </Card>

          {/* 内容/表达式/依赖/事件 */}
          <Card className="flex min-h-[300px] flex-col">
            <div className="flex gap-1 border-b px-2">
              {([
                ["content", "内容预览"],
                ...(version.package.type === "COMPOSITE" && version.package.expression ? [["expression", "表达式"] as const] : []),
                ["deps", "依赖"],
                ["events", "事件"],
              ] as const).map(([k, l]) => (
                <button key={k} onClick={() => setTab(k)} className={cn("border-b-2 px-3 py-2 text-sm font-medium transition-colors", tab === k ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")}>{l}</button>
              ))}
              <div className="ml-auto flex items-center gap-1 p-1">
                {tab === "content" && content && (
                  <>
                    <button onClick={() => copyToClipboard(content.content).then((ok) => ok && toast("已复制", "success"))} className="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground" title="复制"><Copy className="h-4 w-4" /></button>
                    <a href={`data:text/plain;charset=utf-8,${encodeURIComponent(content.content)}`} download={`${decodedName}-${version.package.version}.md`} className="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground" title="下载"><Download className="h-4 w-4" /></a>
                  </>
                )}
                {tab === "expression" && version.package.expression && (
                  <button onClick={() => copyToClipboard(version.package.expression || "").then((ok) => ok && toast("已复制表达式", "success"))} className="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground" title="复制表达式"><Copy className="h-4 w-4" /></button>
                )}
              </div>
            </div>
            <div className="min-h-0 flex-1 overflow-auto p-3">
              {tab === "content" && (
                content ? (
                  <div className="prose prose-sm max-w-none text-foreground">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{content.content}</ReactMarkdown>
                  </div>
                ) : contentLoading ? (
                  <div className="flex h-full items-center justify-center"><Spinner /></div>
                ) : (
                  <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                    <Button variant="outline" size="sm" onClick={loadContent}><Download className="h-4 w-4" /> 加载内容</Button>
                  </div>
                )
              )}
              {tab === "expression" && (
                version.package.expression ? (
                  <pre className="overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted/60 px-3 py-3 font-mono text-xs leading-relaxed text-foreground">{version.package.expression}</pre>
                ) : <Empty text="无表达式" />
              )}
              {tab === "deps" && (
                version.dependencies.length ? (
                  <ul className="space-y-1 text-sm">
                    {version.dependencies.map((dep) => (
                      <li key={dep.name} className="flex items-center justify-between">
                        <Link to={`/packages/${encodeURIComponent(dep.name)}`} className="font-mono text-primary hover:underline">{dep.name}</Link>
                        <span className="text-xs text-muted-foreground">{dep.resolved_version ? `@ ${dep.resolved_version}` : "未解析"}</span>
                      </li>
                    ))}
                  </ul>
                ) : <Empty text="无依赖" />
              )}
              {tab === "events" && (
                version.events && version.events.length > 0 ? (
                  <EventTimeline events={version.events} />
                ) : <Empty text="暂无版本事件" />
              )}
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}

function Row({ k, v, mono }: { k: string; v: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-muted-foreground">{k}</dt>
      <dd className={cn("text-right text-foreground", mono && "font-mono text-xs")}>{v}</dd>
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">{text}</div>;
}

const EVENT_LABEL: Record<string, string> = {
  CREATED: "创建", UPDATED: "更新", PUBLISHED: "发布", DEPRECATED: "废弃", COMPUTED: "计算", CACHE_INVALIDATED: "缓存失效",
};

function EventTimeline({ events }: { events: VersionEvent[] }) {
  return (
    <div className="space-y-3">
      {events.map((ev, i) => (
        <div key={i} className="flex gap-3">
          <div className="flex flex-col items-center">
            <span className="h-2 w-2 rounded-full bg-primary" />
            {i < events.length - 1 && <span className="w-px flex-1 bg-border" />}
          </div>
          <div className="pb-1">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-foreground">{EVENT_LABEL[ev.event_type] || ev.event_type}</span>
              {ev.actor_name && <span className="text-xs text-muted-foreground">{ev.actor_name}</span>}
            </div>
            {ev.message && <p className="text-xs text-muted-foreground">{ev.message}</p>}
            <p className="text-[10px] text-muted-foreground">{(ev.created_at || "").slice(0, 19).replace("T", " ")}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
