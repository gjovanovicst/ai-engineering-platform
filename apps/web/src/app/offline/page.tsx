export default function OfflinePage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col items-center justify-center gap-3 p-6 text-center">
      <h1 className="text-xl font-semibold">You are offline</h1>
      <p className="text-sm text-text-secondary">
        The shell is cached. Reconnect to load runs and approvals.
      </p>
    </main>
  );
}
