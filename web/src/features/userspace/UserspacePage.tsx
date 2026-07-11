import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "@/api";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Button, Card, Spinner } from "@/components/ui";
import { cn } from "@/lib/cn";
import type { GenerationRecord, NamespaceSummary, PackageSummary } from "@/types";

type Tab = "packages" | "stars" | "namespaces" | "history" | "prefs";

export function UserspacePage() {
  const { user, logout } = useAuthStore();
  const dicts = useDictStore((s) => s.dicts);
  const label = (group: string, value?: string) => (dicts[group] ?? []).find((i) => i.value === (value ?? ""))?.label ?? (value ?? "");
  const [tab, setTab] = useState<Tab>("packages");

  if (!user) return null;
  const initial = (user.display_name || user.username || "U").slice(0, 1).toUpperCase();

  const tabs: { key: Tab; label: string }[] = [
    { key: "packages", label: "我发布的" },
    { key: "stars", label: "我的收藏" },
    { key: "namespaces", label: "我的命名空间" },
    { key: "history", label: "生成历史" },
    { key: "prefs", label: "偏好设置" },
  ];

  return (
    <div className="space-y-4">
      <Card className="p-5">
        <div className="flex items-center gap-4">
          <span className="flex h-14 w-14 items-center justify-center rounded-full bg-primary/15 text-xl font-semibold text-primary ring-1 ring-primary/20">
            {initial}
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-bold text-foreground">{user.display_name || user.username}</h1>
              <Badge color="blue">{label("account_role", user.role)}</Badge>
              <Badge color={user.status === "ACTIVE" ? "green" : "yellow"}>{label("account_status", user.status)}</Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">@{user.username}</p>
            <p className="text-xs text-muted-foreground">最近登录 {(user.last_login_at || "").slice(0, 16).replace("T", " ") || "—"}</p>
          </div>
          <Button variant="outline" size="sm" onClick={() => { logout(); toast("已退出登录", "info"); }}>退出登录</Button>
        </div>
      </Card>

      <div className="flex gap-1 border-b">
        {tabs.map((t) => (
          <button key={t.key} onClick={() => setTab(t.key)} className={cn("border-b-2 px-4 py-2 text-sm font-medium transition-colors", tab === t.key ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")}>
            {t.label}
          </button>
        ))}
      </div>

      <Card className="p-4 min-h-[300px]">
        {tab === "packages" && <MyPackages label={label} />}
        {tab === "stars" && <MyStars label={label} />}
        {tab === "namespaces" && <MyNamespaces label={label} />}
        {tab === "history" && <MyHistory />}
        {tab === "prefs" && <Prefs />}
      </Card>
    </div>
  );
}

function MyPackages({ label }: { label: (group: string, value?: string) => string }) {
  const [items, setItems] = useState<PackageSummary[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.myPackages(1, 50).then((r) => setItems(r.items ?? [])).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  }, []);
  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  if (items.length === 0) return <Empty text="还没有发布过提示词包" />;
  return (
    <table className="w-full text-sm">
      <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
        <tr><th className="px-3 py-2">包名</th><th className="px-3 py-2">分类</th><th className="px-3 py-2">状态</th><th className="px-3 py-2">最新版本</th><th className="px-3 py-2 text-right">引用</th><th className="px-3 py-2 text-right">下载</th></tr>
      </thead>
      <tbody>
        {items.map((p) => (
          <tr key={p.name} className="border-b border-border/60 hover:bg-muted/40">
            <td className="px-3 py-2"><Link to={`/packages/${encodeURIComponent(p.name)}`} className="font-mono text-xs text-primary hover:underline">{p.name}</Link></td>
            <td className="px-3 py-2">{label("prompt_category", p.category)}</td>
            <td className="px-3 py-2"><Badge color={p.status === "DEPRECATED" ? "yellow" : "green"}>{label("version_status", p.status)}</Badge></td>
            <td className="px-3 py-2 text-xs">{p.latest_version || "-"}</td>
            <td className="px-3 py-2 text-right text-xs">{p.reference_count}</td>
            <td className="px-3 py-2 text-right text-xs">{p.download_count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function MyStars({ label }: { label: (group: string, value?: string) => string }) {
  const [items, setItems] = useState<PackageSummary[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.myStars(1, 50).then((r) => setItems(r.items ?? [])).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  }, []);
  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  if (items.length === 0) return <Empty text="还没有收藏的提示词包" />;
  return (
    <table className="w-full text-sm">
      <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
        <tr><th className="px-3 py-2">包名</th><th className="px-3 py-2">分类</th><th className="px-3 py-2">状态</th><th className="px-3 py-2">最新版本</th><th className="px-3 py-2 text-right">引用</th><th className="px-3 py-2 text-right">下载</th></tr>
      </thead>
      <tbody>
        {items.map((p) => (
          <tr key={p.name} className="border-b border-border/60 hover:bg-muted/40">
            <td className="px-3 py-2"><Link to={`/packages/${encodeURIComponent(p.name)}`} className="font-mono text-xs text-primary hover:underline">{p.name}</Link></td>
            <td className="px-3 py-2">{label("prompt_category", p.category)}</td>
            <td className="px-3 py-2"><Badge color={p.status === "DEPRECATED" ? "yellow" : "green"}>{label("version_status", p.status)}</Badge></td>
            <td className="px-3 py-2 text-xs">{p.latest_version || "-"}</td>
            <td className="px-3 py-2 text-right text-xs">{p.reference_count}</td>
            <td className="px-3 py-2 text-right text-xs">{p.download_count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function MyNamespaces({ label }: { label: (group: string, value?: string) => string }) {
  const [items, setItems] = useState<NamespaceSummary[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.myNamespaces().then(setItems).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  }, []);
  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  if (items.length === 0) return <Empty text="没有命名空间" />;
  return (
    <div className="space-y-2">
      {items.map((ns) => (
        <div key={ns.id} className="flex items-center justify-between rounded-md border border-border/70 px-3 py-2">
          <div>
            <span className="font-mono text-sm text-foreground">{ns.name}</span>
            {ns.display_name && <span className="ml-2 text-xs text-muted-foreground">{ns.display_name}</span>}
          </div>
          <div className="flex items-center gap-2">
            <Badge color="slate">{label("visibility", ns.visibility)}</Badge>
            <Badge color="blue">{label("namespace_role", ns.role)}</Badge>
          </div>
        </div>
      ))}
    </div>
  );
}

function MyHistory() {
  const [items, setItems] = useState<GenerationRecord[]>([]);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    api.myGenerations(1, 50).then((r) => setItems(r.items ?? [])).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  }, []);
  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  if (items.length === 0) return <Empty text="没有生成记录" />;
  return (
    <table className="w-full text-sm">
      <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
        <tr><th className="px-3 py-2">时间</th><th className="px-3 py-2">表达式</th><th className="px-3 py-2">缓存</th><th className="px-3 py-2 text-right">Token</th><th className="px-3 py-2 text-right">耗时</th></tr>
      </thead>
      <tbody>
        {items.map((r) => (
          <tr key={r.id} className="border-b border-border/60 hover:bg-muted/40">
            <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">{(r.created_at || "").slice(0, 19).replace("T", " ")}</td>
            <td className="max-w-[280px] truncate px-3 py-2 font-mono text-xs" title={r.expression}>{r.expression || "-"}</td>
            <td className="px-3 py-2"><Badge color={r.cache_hit ? "green" : "slate"}>{r.cache_hit ? "命中" : "未命中"}</Badge></td>
            <td className="px-3 py-2 text-right text-xs">{r.token_total}</td>
            <td className="px-3 py-2 text-right text-xs">{r.duration_ms != null ? `${r.duration_ms}ms` : "-"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Prefs() {
  const [compact, setCompact] = useState(localStorage.getItem("pg.pref.compact") === "1");
  const toggle = () => {
    const next = !compact;
    setCompact(next);
    localStorage.setItem("pg.pref.compact", next ? "1" : "0");
    toast("偏好已保存（下次加载生效）", "success");
  };
  return (
    <div className="space-y-3">
      <h2 className="text-sm font-semibold text-foreground">界面偏好</h2>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={compact} onChange={toggle} /> 紧凑模式
      </label>
      <p className="text-xs text-muted-foreground">偏好保存在本地浏览器，不影响其他设备。</p>
    </div>
  );
}

function Empty({ text }: { text: string }) {
  return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">{text}</div>;
}
