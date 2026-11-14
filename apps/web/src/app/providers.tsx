"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

import { ServiceWorkerRegister } from "@/components/service-worker";
import { ToastProvider } from "@/components/ui/toast";

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(() => new QueryClient());
  return (
    <QueryClientProvider client={client}>
      <ServiceWorkerRegister />
      <ToastProvider>{children}</ToastProvider>
    </QueryClientProvider>
  );
}
