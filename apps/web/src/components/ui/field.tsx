export function Field({
  label,
  htmlFor,
  error,
  help,
  children,
}: {
  label: string;
  htmlFor: string;
  error?: string;
  help?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-text-secondary">
        {label}
      </label>
      {help ? <p className="mb-1.5 text-xs text-text-muted">{help}</p> : null}
      {children}
      {error ? (
        <p role="alert" className="mt-1 text-xs text-status-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}
