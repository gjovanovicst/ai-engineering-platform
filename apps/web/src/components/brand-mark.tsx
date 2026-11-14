type BrandMarkProps = { className?: string };

// Single source of truth for the product mark; mirrors site/assets/favicon.svg.
export function BrandMark({ className = "brand-mark" }: BrandMarkProps) {
  return (
    <svg className={className} viewBox="0 0 32 32" role="img" aria-label="AI Engineering Platform">
      <rect width="32" height="32" rx="8" fill="#080b12" />
      <path d="M7 24 16 5l9 19h-4.2l-1.6-3.7h-6.4L11.2 24H7Zm6.9-7h4.2L16 11.6 13.9 17Z" fill="#7c9cff" />
      <circle cx="16" cy="27" r="1.8" fill="#7c9cff" />
    </svg>
  );
}
