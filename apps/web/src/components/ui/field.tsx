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
    // The error is absolute: it must not change the field's height, or an
    // items-end action row would move its button down when the error appears.
    <div className="relative pb-4">
      <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-text-secondary">
        {label}
      </label>
      {help ? <p className="mb-1.5 text-xs text-text-muted">{help}</p> : null}
      {children}
      {error ? (
        <p role="alert" className="absolute left-0 top-full -mt-3.5 text-xs text-status-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}
