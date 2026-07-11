import type { ChangeEvent } from "react";
import { cn } from "@/lib/cn";

export interface PackageNameInputProps {
  /** 只读前缀，通常为「分类(小写)-」；无分类时传空串 */
  prefix: string;
  value: string;
  onChange: (fullName: string) => void;
  placeholder?: string;
}

/**
 * 包名组合输入：只读前缀（分类小写 + 连字符）+ 名称后缀，自动拼成完整包名。
 * 后缀实时小写化并过滤为合法字符（小写字母、数字、连字符）。
 */
export function PackageNameInput({ prefix, value, onChange, placeholder }: PackageNameInputProps) {
  const suffix = prefix && value.startsWith(prefix) ? value.slice(prefix.length) : value;

  function handle(e: ChangeEvent<HTMLInputElement>) {
    const normalized = e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "");
    onChange(prefix ? prefix + normalized : normalized);
  }

  return (
    <div className="space-y-1">
      <div className="flex items-stretch">
        {prefix ? (
          <span className="inline-flex select-none items-center rounded-l-md border border-r-0 border-input bg-muted px-2 font-mono text-xs text-muted-foreground">
            {prefix}
          </span>
        ) : null}
        <input
          value={suffix}
          onChange={handle}
          placeholder={placeholder ?? "名称"}
          className={cn(
            "flex h-9 w-full flex-1 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1",
            prefix ? "rounded-l-none" : "",
          )}
        />
      </div>
      <p className="text-[11px] text-muted-foreground">规则：分类(小写)-名称，仅小写字母、数字、连字符。</p>
    </div>
  );
}
