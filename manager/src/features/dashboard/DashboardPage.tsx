import { useEffect, useState } from "react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Card, Spinner } from "@/components/ui";
import type { PackageSummary } from "@/types";

function Stat({ label, value, accent }: { label: string; value: number; accent?: string }) {
  return (
    <Card className="p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-bold" style={accent ? { color: accent } : undefined}>{value}</div>
    </Card>
  );
}

export function DashboardPage() {
  const label = useDictStore((s) => s.label);
  const [packages, setPackages] = useState<PackageSummary[]>([]);
  const [webUsers, setWebUsers] = useState(0);
  const [managerUsers, setManagerUsers] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.searchPackages({ page: 1, page_size: 8 }).then((r) => r.items ?? []).catch((e) => { toast(e.message, "error"); return []; }),
      api.users(undefined, undefined, 1, 1).then((r) => r.total ?? 0).catch((e) => { toast(e.message, "error"); return 0; }),
      api.managerUsers(undefined, undefined, 1, 1).then((r) => r.total ?? 0).catch((e) => { toast(e.message, "error"); return 0; }),
    ])
      .then(([pkgs, webTotal, mgrTotal]) => {
        setPackages(pkgs as PackageSummary[]);
        setWebUsers(webTotal);
        setManagerUsers(mgrTotal);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-10 text-center"><Spinner /></div>;

  const deprecated = packages.filter((p) => p.status === "DEPRECATED").length;

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-foreground">运营概览</h1>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="提示词包" value={packages.length} />
        <Stat label="已废弃版本" value={deprecated} accent="#ca8a04" />
        <Stat label="Web 用户" value={webUsers} />
        <Stat label="管理端用户" value={managerUsers} accent="#16a34a" />
      </div>

      <Card className="p-4">
        <h2 className="mb-3 text-sm font-semibold text-foreground">最近提示词包</h2>
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-3 py-2">包名</th>
              <th className="px-3 py-2">分类</th>
              <th className="px-3 py-2">状态</th>
              <th className="px-3 py-2 text-right">下载</th>
            </tr>
          </thead>
          <tbody>
            {packages.map((p) => (
              <tr key={p.name} className="border-b border-border/60 hover:bg-muted/40">
                <td className="px-3 py-2 font-mono text-xs text-primary">{p.name}</td>
                <td className="px-3 py-2">{label("prompt_category", p.category)}</td>
                <td className="px-3 py-2"><Badge color={p.status === "DEPRECATED" ? "yellow" : "green"}>{label("version_status", p.status)}</Badge></td>
                <td className="px-3 py-2 text-right text-xs text-muted-foreground">{p.download_count}</td>
              </tr>
            ))}
            {packages.length === 0 && <tr><td colSpan={4} className="px-3 py-6 text-center text-muted-foreground">暂无提示词包</td></tr>}
          </tbody>
        </table>
      </Card>
    </div>
  );
}
