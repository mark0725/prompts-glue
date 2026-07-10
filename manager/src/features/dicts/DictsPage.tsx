import { useDictStore } from "@/store/dict";
import { Badge, Card, Spinner } from "@/components/ui";

const groupTitle: Record<string, string> = {
  prompt_type: "提示词类型",
  prompt_category: "提示词分类",
  version_status: "版本状态",
  visibility: "资源可见性",
  account_status: "账号状态",
  namespace_role: "命名空间角色",
  cache_strategy: "缓存策略",
  risk_type: "风险类型",
  risk_severity: "风险等级",
  risk_status: "风险处理状态",
  operation_result: "操作结果",
};

export function DictsPage() {
  const { dicts, loaded, error, load } = useDictStore();
  const groups = Object.keys(dicts).sort();

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold text-foreground">数据字典</h1>
        <p className="text-sm text-muted-foreground">字典由后端统一管理，管理台只读展示。共 {groups.length} 个分组。</p>
      </div>
      {!loaded ? <div className="p-10 text-center"><Spinner /></div> : null}
      {error ? <p className="text-sm text-red-600">{error} <button className="underline" onClick={load}>重试</button></p> : null}
      <div className="space-y-4">
        {groups.map((group) => (
          <Card key={group} className="p-4">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-foreground">{groupTitle[group] || group}</h2>
              <span className="text-xs text-muted-foreground">{group}</span>
            </div>
            <div className="grid gap-2 md:grid-cols-2 lg:grid-cols-3">
              {dicts[group].map((item) => (
                <div key={item.value} className="flex items-center justify-between rounded-md border border-border/70 px-3 py-2 text-sm">
                  <div className="min-w-0">
                    <div className="truncate text-foreground">{item.label}</div>
                    <div className="truncate text-xs text-muted-foreground">{item.value}</div>
                  </div>
                  <Badge color={item.enabled ? "green" : "slate"}>{item.enabled ? "启用" : "停用"}</Badge>
                </div>
              ))}
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
