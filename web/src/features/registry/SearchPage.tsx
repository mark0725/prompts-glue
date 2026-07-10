import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Download, Search, User, X } from "lucide-react";
import { api } from "@/api";
import { useDictStore } from "@/store/dict";
import { toast } from "@/store/feedback";
import { Badge, Input, Spinner } from "@/components/ui";
import { Select } from "@/components/Select";
import { cn } from "@/lib/cn";
import type { PackageSummary } from "@/types";

const PAGE_SIZE = 12;
const SORTS = [
  { value: "updated", label: "最新" },
  { value: "popular", label: "热门" },
  { value: "downloads", label: "下载量" },
];

export function SearchPage() {
  const dicts = useDictStore((s) => s.dicts);
  const dictItems = (group: string) => dicts[group] ?? [];
  const label = (group: string, value?: string) => (dicts[group] ?? []).find((i) => i.value === (value ?? ""))?.label ?? (value ?? "");
  const [searchParams, setSearchParams] = useSearchParams();
  const [q, setQ] = useState(searchParams.get("q") ?? "");
  const [category, setCategory] = useState(searchParams.get("category") ?? "");
  const [type, setType] = useState("");
  const [sort, setSort] = useState("updated");
  const [items, setItems] = useState<PackageSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const pageRef = useRef(1);
  const hasMoreRef = useRef(true);
  const loadingRef = useRef(false);
  const filterRef = useRef({ q, category, type, sort });

  pageRef.current = pageRef.current; // keep
  hasMoreRef.current = hasMore;
  filterRef.current = { q, category, type, sort };

  const fetchPage = (p: number, append: boolean) => {
    loadingRef.current = true;
    if (append) setLoadingMore(true); else setLoading(true);
    const f = filterRef.current;
    api.searchPackages({ q: f.q, category: f.category, type: f.type, page: p, page_size: PAGE_SIZE })
      .then((result) => {
        const newItems = result.items ?? [];
        setItems((prev) => append ? [...prev, ...newItems] : newItems);
        setTotal(result.total ?? 0);
        setHasMore(result.has_more ?? false);
      })
      .catch((e) => toast(e.message, "error"))
      .finally(() => {
        loadingRef.current = false;
        setLoading(false);
        setLoadingMore(false);
      });
  };

  // Reset + load page 1 on filter/search/sort change
  useEffect(() => {
    pageRef.current = 1;
    fetchPage(1, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, category, type, sort]);

  // Infinite scroll observer
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting && hasMoreRef.current && !loadingRef.current) {
        const next = pageRef.current + 1;
        pageRef.current = next;
        fetchPage(next, true);
      }
    }, { rootMargin: "300px" });
    observer.observe(sentinel);
    return () => observer.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isSearching = Boolean(q || category || type);
  const applyCategory = (value: string) => {
    setCategory(value);
    const next = new URLSearchParams(searchParams);
    if (value) next.set("category", value);
    else next.delete("category");
    setSearchParams(next, { replace: true });
  };
  const reset = () => {
    setQ("");
    setCategory("");
    setType("");
    setSearchParams({}, { replace: true });
  };

  return (
    <div className="space-y-4">
      {/* 搜索栏（搜索 + 类型巧克力块 + 排序 + 清除，无外框） */}
      <div className="flex flex-wrap items-center gap-2 px-2 py-1">
        <div className="relative min-w-[200px] flex-1">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="搜索包名、说明或标签"
            className="h-9 pl-8"
            onKeyDown={(e) => { if (e.key === "Enter") setQ(e.currentTarget.value); }}
          />
        </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <button
              onClick={() => setType("")}
              className={cn("shrink-0 rounded-full border px-2.5 py-1 text-xs transition-colors", type === "" ? "border-primary bg-primary text-primary-foreground" : "border-border text-muted-foreground hover:bg-accent")}
            >
              全部类型
            </button>
            {dictItems("prompt_type").map((t) => (
              <button
                key={t.value}
                onClick={() => setType(t.value)}
                className={cn("shrink-0 rounded-full border px-2.5 py-1 text-xs transition-colors", type === t.value ? "border-primary bg-primary text-primary-foreground" : "border-border text-muted-foreground hover:bg-accent")}
              >
                {t.label}
              </button>
            ))}
          </div>
          <Select className="w-28" value={sort} onChange={setSort} options={SORTS} />
          {isSearching && (
            <button onClick={reset} className="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent"><X className="h-3.5 w-3.5" /> 清除</button>
          )}
        </div>

      {/* 分类（巧克力块，搜索框下方） */}
      <div className="flex items-center gap-2 overflow-x-auto pb-1">
        <button
          onClick={() => applyCategory("")}
          className={cn("shrink-0 rounded-full border px-3 py-1 text-sm transition-colors", category === "" ? "border-primary bg-primary text-primary-foreground" : "border-border text-muted-foreground hover:bg-accent")}
        >
          全部
        </button>
        {dictItems("prompt_category").map((c) => (
          <button
            key={c.value}
            onClick={() => applyCategory(c.value)}
            className={cn("shrink-0 rounded-full border px-3 py-1 text-sm transition-colors", category === c.value ? "border-primary bg-primary text-primary-foreground" : "border-border text-muted-foreground hover:bg-accent")}
          >
            {c.label}
          </button>
        ))}
      </div>

      {/* 视图标题 */}
      <div className="flex items-center justify-between px-1">
        <h1 className="text-base font-semibold text-foreground">{isSearching ? "搜索结果" : "Prompts"}</h1>
        <span className="text-xs text-muted-foreground">共 {total} 条</span>
      </div>

      {/* 卡片网格 */}
      {loading && items.length === 0 ? (
        <div className="p-10 text-center"><Spinner /></div>
      ) : (
        <>
          <div className="grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(240px,1fr))]">
            {items.map((pkg) => (
              <Link
                key={pkg.name}
                to={`/packages/${encodeURIComponent(pkg.name)}`}
                className="group flex flex-col gap-2 rounded-lg border bg-card p-4 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md"
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="font-mono text-sm font-medium text-foreground group-hover:text-primary">{pkg.name}</span>
                  {pkg.author && (
                    <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground" title={pkg.author}>
                      <User className="h-3 w-3" /> {pkg.author}
                    </span>
                  )}
                </div>
                <p className="line-clamp-2 text-sm text-muted-foreground">{pkg.description}</p>
                <div className="flex flex-wrap gap-1">
                  <Badge color="blue">{label("prompt_category", pkg.category)}</Badge>
                  {(pkg.tags ?? []).slice(0, 3).map((tag) => (
                    <Badge key={tag} color="slate">{tag}</Badge>
                  ))}
                </div>
                <div className="mt-auto flex items-center justify-between pt-1 text-xs text-muted-foreground">
                  <span>{pkg.latest_version ? `最新 ${pkg.latest_version}` : "暂无版本"}</span>
                  <span className="flex items-center gap-1"><Download className="h-3.5 w-3.5" /> {pkg.download_count}</span>
                </div>
              </Link>
            ))}
          </div>

          {/* 加载更多 / 哨兵 */}
          {loadingMore && <div className="py-4 text-center"><Spinner /></div>}
          {!hasMore && items.length > 0 && (
            <p className="py-4 text-center text-xs text-muted-foreground">没有更多了</p>
          )}
          {items.length === 0 && !loading && (
            <div className="rounded-lg border border-dashed p-12 text-center text-muted-foreground">
              <Search className="mx-auto mb-2 h-8 w-8 opacity-40" />
              暂无匹配的提示词包
              {isSearching && <div className="mt-3"><button onClick={reset} className="text-primary underline">清除筛选</button></div>}
            </div>
          )}
          {/* 哨兵节点触发无限滚动 */}
          <div ref={sentinelRef} className="h-1" />
        </>
      )}
    </div>
  );
}
