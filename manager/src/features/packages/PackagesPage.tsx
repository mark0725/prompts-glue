import { useEffect, useState } from "react";
import { RefreshCw, Search } from "lucide-react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast, confirmDialog } from "@/store/feedback";
import { Badge, Button, Card, Input, Spinner } from "@/components/ui";
import { Select } from "@/components/Select";
import { Pager } from "@/components/Pager";
import type { PackageSummary } from "@/types";

export function PackagesPage() {
  const label = useDictStore((s) => s.label);
  const dictItems = useDictStore((s) => s.items);
  const [list, setList] = useState<PackageSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [f, setF] = useState({ q: "", category: "", status: "" });

  const load = (p = page, ps = pageSize) => {
    setLoading(true);
    api.searchPackages({ q: f.q, category: f.category, status: f.status, page: p, page_size: ps })
      .then((d) => { setList(d.items ?? []); setTotal(d.total ?? 0); setPage(d.page ?? p); })
      .catch((e) => toast(e.message, "error"))
      .finally(() => setLoading(false));
  };
  useEffect(() => { load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const search = () => load(1);
  const reset = () => { setF({ q: "", category: "", status: "" }); setTimeout(() => load(1), 0); };

  const deprecate = async (pkg: PackageSummary) => {
    if (!pkg.latest_version) return;
    if (!(await confirmDialog(`确认废弃 ${pkg.name}@${pkg.latest_version}？废弃不会删除历史内容。`, "废弃版本"))) return;
    try {
      await api.deprecate(pkg.name, pkg.latest_version, "管理员废弃");
      toast("已废弃", "success");
      load();
    } catch (e: any) { toast(e.message, "error"); }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-foreground">提示词包</h1>
        <Button variant="outline" size="sm" onClick={() => load()}><RefreshCw size={14} /></Button>
      </div>

      <Card className="p-2">
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={f.q}
              onChange={(e) => setF({ ...f, q: e.target.value })}
              placeholder="包名 / 说明"
              className="h-8 w-52 pl-8 text-xs"
              onKeyDown={(e) => { if (e.key === "Enter") search(); }}
            />
          </div>
          <Select className="w-28" value={f.category} onChange={(v) => setF({ ...f, category: v })} options={[{ value: "", label: "全部分类" }, ...dictItems("prompt_category").map((i) => ({ value: i.value, label: i.label }))]} />
          <Select className="w-28" value={f.status} onChange={(v) => setF({ ...f, status: v })} options={[{ value: "", label: "全部状态" }, ...dictItems("version_status").map((i) => ({ value: i.value, label: i.label }))]} />
          <Select className="w-28" value={f.status} onChange={(v) => setF({ ...f, status: v })} options={[{ value: "", label: "全部状态" }, ...dictItems("version_status").map((i) => ({ value: i.value, label: i.label }))]} />
          <Button size="sm" onClick={search}>查询</Button>
          <Button size="sm" variant="ghost" onClick={reset}>重置</Button>
        </div>
      </Card>

      <Card>
        {loading ? <div className="p-10 text-center"><Spinner /></div> : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2">包名</th>
                <th className="px-3 py-2">分类</th>
                <th className="px-3 py-2">最新版本</th>
                <th className="px-3 py-2">状态</th>
                <th className="px-3 py-2 text-right">下载</th>
                <th className="px-3 py-2 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {list.map((p) => (
                <tr key={p.name} className="border-b border-border/60 hover:bg-muted/40">
                  <td className="px-3 py-2 font-mono text-xs text-primary">{p.name}</td>
                  <td className="px-3 py-2">{label("prompt_category", p.category)}</td>
                  <td className="px-3 py-2 text-xs">{p.latest_version || "-"}</td>
                  <td className="px-3 py-2"><Badge color={p.status === "DEPRECATED" ? "yellow" : "green"}>{label("version_status", p.status)}</Badge></td>
                  <td className="px-3 py-2 text-right text-xs">{p.download_count}</td>
                  <td className="px-3 py-2 text-right">
                    <Button size="sm" variant="ghost" disabled={!p.latest_version || p.status === "DEPRECATED"} onClick={() => deprecate(p)}>废弃</Button>
                  </td>
                </tr>
              ))}
              {list.length === 0 && <tr><td colSpan={6} className="px-3 py-8 text-center text-muted-foreground">暂无提示词包</td></tr>}
            </tbody>
          </table>
        )}
      </Card>

      <Pager page={page} total={total} pageSize={pageSize} onPageChange={(p) => load(p)} onPageSizeChange={(s) => { setPageSize(s); load(1, s); }} />
    </div>
  );
}
