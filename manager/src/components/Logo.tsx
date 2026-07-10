// prompts-glue 系统图标：渐变圆角徽标 + 提示词括号 </> + AI 火花，表达"提示词组合生成"。
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 40 40" className={className} fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <defs>
        <linearGradient id="pg-logo-grad" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
          <stop stopColor="#6366f1" />
          <stop offset="1" stopColor="#8b5cf6" />
        </linearGradient>
      </defs>
      <rect width="40" height="40" rx="10" fill="url(#pg-logo-grad)" />
      <path d="M14 13.5L8.5 20L14 26.5" stroke="#ffffff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M26 13.5L31.5 20L26 26.5" stroke="#ffffff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M20 10.5L21.7 18.3L29.5 20L21.7 21.7L20 29.5L18.3 21.7L10.5 20L18.3 18.3Z" fill="#fde68a" />
    </svg>
  );
}
