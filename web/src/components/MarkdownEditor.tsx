import MDEditor from "@uiw/react-md-editor";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/cn";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";

export interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  /** 默认 live：编辑与预览并排（所见即所得）；也可 edit / preview */
  preview?: "live" | "edit" | "preview";
}

/**
 * Markdown 所见即所得编辑器。
 * 默认 live 并排：左侧编辑、右侧实时预览；支持工具栏格式化与 GFM。
 * 以 flex 方式填充父容器：父容器需为具有确定高度的纵向 flex 布局。
 */
export function MarkdownEditor({
  value,
  onChange,
  placeholder = "输入 Markdown 内容…",
  className,
  preview = "live",
}: MarkdownEditorProps) {
  return (
    <div
      className={cn(
        "markdown-editor flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border border-input bg-background",
        className,
      )}
      data-color-mode="light"
    >
      <MDEditor
        value={value}
        onChange={(v) => onChange(v ?? "")}
        className="min-h-0 flex-1"
        preview={preview}
        height="100%"
        visibleDragbar={false}
        textareaProps={{ placeholder }}
        previewOptions={{ remarkPlugins: [remarkGfm] }}
      />
    </div>
  );
}
