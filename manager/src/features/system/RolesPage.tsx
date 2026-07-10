import { useEffect, useState } from "react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Card, Spinner } from "@/components/ui";
import type { DictItem, RoleSummary } from "@/types";

function RoleSection({ title, group, summary, countOf }: { title: string; group: string; summary: RoleSummary; countOf: (code: string) => number }) {
  const dictItems = useDictStore((s) => s.items);
  const items: DictItem[] = dictItems(group);
  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground">{title}</h2>
        <span className="text-xs text-muted-foreground">{items.length} 个角色</span>
      </div>
      <div className="grid gap-2 md:grid-cols-2">
        {items.map((r) => (
          <div key={r.value} className="flex items-center justify-between rounded-md border border-border/70 px-3 py-2 text-sm">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate text-foreground">{r.label}</span>
                <span className="font-mono text-xs text-muted-foreground">{r.value}</span>
              </div>
              {r.description ? <div className="truncate text-xs text-muted-foreground">{r.description}</div> : null}
            </div>
            <Badge color={countOf(r.value) > 0 ? "blue" : "slate"}>{countOf(r.value)} 人</Badge>
          </div>
        ))}
        {items.length === 0 && <div className="col-span-2 px-3 py-6 text-center text-sm text-muted-foreground">暂无角色定义</div>}
      </div>
      {/* 统计中存在但字典缺失的角色（数据兜底） */}
      {((group === "account_role" ? summary.account_roles : summary.namespace_roles) || [])
        .filter((r) => !items.some((i) => i.value === r.code))
        .map((r) => (
          <div key={r.code} className="mt-2 flex items-center justify-between rounded-md border border-dashed border-border/70 px-3 py-2 text-sm">
            <span className="font-mono text-xs text-muted-foreground">{r.code}（未定义）</span>
            <Badge color="slate">{r.count} 人</Badge>
          </div>
        ))}
    </Card>
  );
}

export function RolesPage() {
  const [summary, setSummary] = useState<RoleSummary>({ account_roles: [], namespace_roles: [] });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.roles().then(setSummary).catch((e) => toast(e.message, "error")).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-10 text-center"><Spinner /></div>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold text-foreground">角色</h1>
        <p className="text-sm text-muted-foreground">系统角色定义与使用统计（只读）。账号角色用于用户登录权限，命名空间角色用于包维护协作。</p>
      </div>
      <RoleSection title="账号角色" group="account_role" summary={summary} countOf={(c) => (summary.account_roles ?? []).find((r) => r.code === c)?.count ?? 0} />
      <RoleSection title="命名空间角色" group="namespace_role" summary={summary} countOf={(c) => (summary.namespace_roles ?? []).find((r) => r.code === c)?.count ?? 0} />
      <p className="text-xs text-muted-foreground">注：当前为角色定义与统计视图。若需完整的角色-权限维护（自定义角色、权限分配、成员管理），需要先扩展数据库设计（角色与权限表）。</p>
    </div>
  );
}
