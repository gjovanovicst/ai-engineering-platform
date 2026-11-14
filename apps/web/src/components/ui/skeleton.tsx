export function Skeleton({ className = "" }: { className?: string }) {
  return <div aria-hidden="true" className={`animate-pulse rounded-md bg-bg-hover ${className}`} />;
}

export function SkeletonText({ lines = 3 }: { lines?: number }) {
  return (
    <div className="space-y-2" aria-hidden="true">
      {Array.from({ length: lines }, (_, index) => (
        <div key={index} className="h-3 animate-pulse rounded bg-bg-hover" style={{ width: `${90 - index * 12}%` }} />
      ))}
    </div>
  );
}
