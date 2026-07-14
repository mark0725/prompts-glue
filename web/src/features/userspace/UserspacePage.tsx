import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "@/api";
import { useAuthStore } from "@/store/auth";
import { useDictStore } from "@/store/dict";
import { confirmDialog, toast } from "@/store/feedback";
import { Badge, Button, Card, Input, Modal, Spinner } from "@/components/ui";
import { cn } from "@/lib/cn";
import type { AccessToken, GenerationRecord, NamespaceSummary, PackageSummary } from "@/types";

type Tab = "packages" | "stars" | "namespaces" | "history" | "tokens" | "prefs";

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
    { key: "tokens", label: "访问令牌" },
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
              <Badge color="blue">{label("account_source", user.source || "LOCAL")}</Badge>
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
        {tab === "tokens" && <AccessTokens />}
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
  const totalTokens = items.reduce((sum, r) => sum + (r.token_total ?? 0), 0);
  if (items.length === 0) return <Empty text="没有生成记录" />;
  return (
    <div className="space-y-3">
      <div className="text-xs text-muted-foreground">累计 Token 消耗：<span className="font-semibold text-foreground">{totalTokens}</span></div>
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
    </div>
  );
}

function AccessTokens() {
  const [items, setItems] = useState<AccessToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [created, setCreated] = useState<string | null>(null);

  const load = () => {
    setLoading(true);
    api.listTokens().then((r) => setItems(r.filter((t) => !t.expires_at || new Date(t.expires_at) > new Date()))).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  };
  useEffect(load, []);

  const submit = () => {
    if (!name) { toast("令牌名称必填", "error"); return; }
    api.createToken(name, expiresAt || undefined).then((r) => {
      setCreated(r.access_token);
      setName(""); setExpiresAt(""); setOpen(false); load();
      toast("令牌已创建，请立即复制保存", "success");
    }).catch((e) => toast(e.message, "error"));
  };
  const revoke = async (id: string, label: string) => {
    const ok = await confirmDialog(`确定吊销令牌 ${label}？`, "吊销令牌");
    if (!ok) return;
    api.revokeToken(id).then(() => { toast("已吊销", "success"); load(); }).catch((e) => toast(e.message, "error"));
  };
  const copy = (v: string) => { navigator.clipboard?.writeText(v).then(() => toast("已复制", "success")); };

  if (loading) return <div className="p-6 text-center"><Spinner /></div>;
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">访问令牌用于 CLI 鉴权（glue config set server.token &lt;token&gt;）。登录创建的令牌也会显示在此处。</p>
        <Button size="sm" onClick={() => setOpen(true)}>新建令牌</Button>
      </div>
      {created && (
        <div className="rounded-md border border-primary/40 bg-primary/5 p-3">
          <div className="text-xs font-medium text-foreground">令牌仅显示一次，请立即复制保存：</div>
          <div className="mt-2 flex items-center gap-2">
            <code className="flex-1 truncate rounded bg-muted px-2 py-1 text-xs">{created}</code>
            <Button size="sm" variant="outline" onClick={() => copy(created)}>复制</Button>
          </div>
        </div>
      )}
      <table className="w-full text-sm">
        <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
          <tr><th className="px-3 py-2">名称</th><th className="px-3 py-2">创建时间</th><th className="px-3 py-2">最近使用</th><th className="px-3 py-2">过期时间</th><th className="px-3 py-2 text-right">操作</th></tr>
        </thead>
        <tbody>
          {items.map((t) => (
            <tr key={t.id} className="border-b border-border/60 hover:bg-muted/40">
              <td className="px-3 py-2 font-mono text-xs">{t.name || "-"}</td>
              <td className="px-3 py-2 text-xs text-muted-foreground">{(t.created_at || "").slice(0, 19).replace("T", " ") || "-"}</td>
              <td className="px-3 py-2 text-xs text-muted-foreground">{(t.last_used_at || "").slice(0, 19).replace("T", " ") || "—"}</td>
              <td className="px-3 py-2 text-xs text-muted-foreground">{t.expires_at ? (t.expires_at).slice(0, 19).replace("T", " ") : "永不过期"}</td>
              <td className="px-3 py-2 text-right"><Button size="sm" variant="ghost" onClick={() => revoke(t.id, t.name || "令牌")}>吊销</Button></td>
            </tr>
          ))}
          {items.length === 0 && <tr><td colSpan={5} className="px-3 py-8 text-center text-muted-foreground">还没有访问令牌</td></tr>}
        </tbody>
      </table>
      <Modal open={open} onClose={() => setOpen(false)} title="新建访问令牌">
        <div className="space-y-3">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="令牌名称（如：ci-token）" />
          <div>
            <Input value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} placeholder="过期时间（可选，留空=永不过期）" type="datetime-local" />
            <p className="mt-1 text-xs text-muted-foreground">留空表示永不过期。</p>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={() => setOpen(false)}>取消</Button>
            <Button size="sm" onClick={submit}>创建</Button>
          </div>
        </div>
      </Modal>
    </div>
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
