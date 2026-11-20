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
    // The error is absolute and occupies no layout height: an action row is
    // items-start, so the button never moves when the error appears.
    <div className="relative mb-4">
      <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-text-secondary">
        {label}
      </label>
      {help ? <p className="mb-1.5 text-xs text-text-muted">{help}</p> : null}
      {children}
      {error ? (
        <p role="alert" className="absolute left-0 top-full text-xs text-status-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}
