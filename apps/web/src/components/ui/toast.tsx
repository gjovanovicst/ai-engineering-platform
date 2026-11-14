"use client";

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

type ToastVariant = "info" | "success" | "danger";
type ToastItem = { id: number; message: string; variant: ToastVariant };

type ToastAPI = { show: (message: string, variant?: ToastVariant) => void };

const ToastContext = createContext<ToastAPI | null>(null);

const styles: Record<ToastVariant, string> = {
  info: "border-brand/30 bg-bg-surface text-text-primary",
  success: "border-status-success/30 bg-status-success/10 text-status-success",
  danger: "border-status-danger/30 bg-status-danger/10 text-status-danger",
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const show = useCallback((message: string, variant: ToastVariant = "info") => {
    const id = Date.now() + Math.random();
    setItems((current) => [...current, { id, message, variant }]);
    setTimeout(() => setItems((current) => current.filter((item) => item.id !== id)), 4000);
  }, []);

  const api = useMemo(() => ({ show }), [show]);

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2">
        {items.map((item) => (
          <div key={item.id} className={`pointer-events-auto rounded-md border px-3 py-2 text-sm shadow-lg ${styles[item.variant]}`}>
            {item.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastAPI {
  const context = useContext(ToastContext);
  if (!context) throw new Error("useToast must be used inside ToastProvider");
  return context;
}
