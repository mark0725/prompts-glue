import type { MouseEvent, ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { Star } from "lucide-react";
import { api } from "@/api";
import { useAuthStore } from "@/store/auth";
import { toast } from "@/store/feedback";
import { cn } from "@/lib/cn";

export interface StarButtonProps {
  name: string;
  starred: boolean;
  count: number;
  /** 主要用于详情页：带边框的次级按钮形态；默认为内联文本样式（卡片） */
  variant?: "inline" | "outline";
  size?: "sm" | "md";
  /** 收藏状态变化回调，回调参数为服务端返回的最新状态 */
  onChange?: (starred: boolean, count: number) => void;
  children?: ReactNode;
}

/**
 * 收藏按钮：点击在 star/unstar 间切换；未登录引导登录。
 * 乐观更新：先回调本地状态，失败时由调用方回滚（onChange 传入服务端最新值）。
 */
export function StarButton({ name, starred, count, variant = "inline", size = "sm", onChange, children }: StarButtonProps) {
  const { user } = useAuthStore();
  const navigate = useNavigate();

  async function toggle(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (!user) {
      toast("请先登录后再收藏", "info");
      navigate("/login");
      return;
    }
    const next = !starred;
    onChange?.(next, Math.max(0, count + (next ? 1 : -1)));
    try {
      const res = next ? await api.star(name) : await api.unstar(name);
      onChange?.(res.starred, res.star_count);
    } catch (err: any) {
      onChange?.(starred, count);
      toast(err?.message || "操作失败", "error");
    }
  }

  const iconCls = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  if (variant === "outline") {
    return (
      <button
        type="button"
        onClick={toggle}
        title={starred ? "取消收藏" : "收藏"}
        className={cn(
          "inline-flex items-center justify-center gap-1.5 rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          size === "sm" ? "h-8 px-3 text-xs" : "h-9 px-4 text-sm",
          starred
            ? "border border-amber-300 bg-amber-50 text-amber-600 hover:bg-amber-100"
            : "border border-input bg-background text-muted-foreground hover:bg-accent hover:text-foreground",
        )}
      >
        <Star className={cn(iconCls, starred && "fill-current")} />
        {children ?? <span>{starred ? "已收藏" : "收藏"}</span>}
        <span className="tabular-nums">{count}</span>
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={toggle}
      title={starred ? "取消收藏" : "收藏"}
      className={cn(
        "inline-flex items-center gap-1 rounded text-xs transition-colors tabular-nums",
        starred ? "text-amber-500 hover:text-amber-600" : "text-muted-foreground hover:text-foreground",
      )}
    >
      <Star className={cn(iconCls, starred && "fill-current")} />
      <span>{count}</span>
    </button>
  );
}
