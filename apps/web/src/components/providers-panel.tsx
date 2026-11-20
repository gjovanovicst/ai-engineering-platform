"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  createProvider,
  deleteProvider,
  listModelPrices,
  listProviders,
  rotateProviderKey,
  setProviderEnabled,
  upsertModelPrice,
} from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { aiProviderSchema, type AiProviderValues } from "@/lib/schemas";

export function ProvidersPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<AiProviderValues>(aiProviderSchema, {
    defaultValues: { name: "", base_url: "https://api.openai.com/v1", model: "", api_key: "" },
  });

  const providers = useQuery({ queryKey: ["providers", orgID], queryFn: () => listProviders(orgID) });
  const prices = useQuery({ queryKey: ["prices", orgID], queryFn: () => listModelPrices(orgID) });
  const [priceForm, setPriceForm] = useState({ provider: "openai-compatible", model: "", input: "", output: "" });
  const [rotating, setRotating] = useState<string | null>(null);
  const [newKey, setNewKey] = useState("");
  const [showProviderForm, setShowProviderForm] = useState(false);
  const [showPriceForm, setShowPriceForm] = useState(false);

  const create = useMutation({
    mutationFn: (input: AiProviderValues) => createProvider(orgID, input),
    onSuccess: () => {
      reset({ name: "", base_url: "https://api.openai.com/v1", model: "", api_key: "" });
      setShowProviderForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["providers", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the provider"),
  });

  const rotate = useMutation({
    mutationFn: (id: string) => rotateProviderKey(orgID, id, newKey),
    onSuccess: () => {
      setRotating(null);
      setNewKey("");
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["providers", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not rotate the key"),
  });

  const toggle = useMutation({
    mutationFn: (input: { id: string; enabled: boolean }) => setProviderEnabled(orgID, input.id, input.enabled),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["providers", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not update the provider"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteProvider(orgID, id),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["providers", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not remove the provider"),
  });

  const savePrice = useMutation({
    mutationFn: () =>
      upsertModelPrice(orgID, {
        provider: priceForm.provider,
        model: priceForm.model,
        input_per_1k: Number(priceForm.input || 0),
        output_per_1k: Number(priceForm.output || 0),
      }),
    onSuccess: () => {
      setPriceForm({ ...priceForm, model: "", input: "", output: "" });
      setShowPriceForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["prices", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the price"),
  });

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="flex items-start justify-between gap-3 border-b border-border p-4">
          <div>
            <h2 className="text-sm font-semibold">AI providers</h2>
            <p className="mt-1 text-xs text-text-secondary">
              The API key is encrypted at rest and never returned — only whether a key is set.
            </p>
          </div>
          <Button variant="secondary" onClick={() => setShowProviderForm(true)}>+ Add provider</Button>
        </header>
        <ul className="divide-y divide-border">
          {providers.isSuccess && providers.data.data.length === 0 ? (
            <li className="p-4 text-sm text-text-secondary">No providers configured.</li>
          ) : null}
          {providers.isSuccess
            ? providers.data.data.map((provider) => (
                <li key={provider.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5 text-sm">
                  <span className="font-medium">{provider.name}</span>
                  <span className="font-mono text-xs text-text-muted">{provider.base_url}</span>
                  {provider.model ? (
                    <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] text-text-muted">{provider.model}</span>
                  ) : null}
                  <span
                    className={`rounded px-1.5 py-0.5 text-[11px] ${
                      provider.has_key
                        ? "bg-status-success/10 text-status-success"
                        : "bg-status-warning/10 text-status-warning"
                    }`}
                  >
                    {provider.has_key ? `key ${provider.key_hint}` : "no key"}
                  </span>
                  <div className="ml-auto flex gap-1">
                    <Button
                      variant="ghost"
                      className="h-7 px-2 text-xs"
                      onClick={() => toggle.mutate({ id: provider.id, enabled: !provider.enabled })}
                    >
                      {provider.enabled ? "Disable" : "Enable"}
                    </Button>
                    <Button
                      variant="secondary"
                      className="h-7 px-2 text-xs"
                      onClick={() => {
                        setRotating(provider.id);
                        setNewKey("");
                      }}
                    >
                      Rotate key
                    </Button>
                    <Button
                      variant="danger"
                      className="h-7 px-2 text-xs"
                      loading={remove.isPending && remove.variables === provider.id}
                      onClick={() => {
                        if (window.confirm(`Remove provider ${provider.name}? Its key is deleted.`)) {
                          remove.mutate(provider.id);
                        }
                      }}
                    >
                      Remove
                    </Button>
                  </div>
                  {rotating === provider.id ? (
                    <div className="flex w-full items-center gap-2 pt-2">
                      <Input
                        aria-label="New API key"
                        type="password"
                        value={newKey}
                        onChange={(e) => setNewKey(e.target.value)}
                        placeholder="new API key"
                      />
                      <Button
                        className="h-8"
                        loading={rotate.isPending}
                        disabled={newKey.trim() === ""}
                        onClick={() => rotate.mutate(provider.id)}
                      >
                        Save key
                      </Button>
                    </div>
                  ) : null}
                </li>
              ))
            : null}
        </ul>
      </section>

      {showProviderForm ? (
      <SidePanel
        dirty
        title="Add AI provider"
        description="The API key is encrypted at rest and never returned."
        onClose={() => setShowProviderForm(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowProviderForm(false)}>Cancel</Button>
            <Button type="submit" form="provider-form" loading={create.isPending}>Save provider</Button>
          </>
        }
      >
      <form
        id="provider-form"
        className="space-y-4 p-5"
        onSubmit={handleSubmit((values) => create.mutate(values))}
      >
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="Name" htmlFor="provider-name" error={errors.name?.message}>
            <Input id="provider-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="e.g. OpenAI" />
          </Field>
          <Field label="Base URL" htmlFor="provider-url" error={errors.base_url?.message}>
            <Input id="provider-url" aria-invalid={Boolean(errors.base_url)} {...register("base_url")} placeholder="https://api.openai.com/v1" />
          </Field>
          <Field label="Model (optional)" htmlFor="provider-model" error={errors.model?.message}>
            <Input id="provider-model" aria-invalid={Boolean(errors.model)} {...register("model")} placeholder="gpt-4o" />
          </Field>
          <Field label="API key" htmlFor="provider-key" error={errors.api_key?.message}>
            <Input
              id="provider-key"
              type="password"
              aria-invalid={Boolean(errors.api_key)}
              {...register("api_key")}
              placeholder="sk-…"
            />
          </Field>
        </div>
        {error ? (
          <p role="alert" className="mt-3 text-xs text-status-danger">
            {error}
          </p>
        ) : null}
      </form>
      </SidePanel>
      ) : null}

      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="flex items-start justify-between gap-3 border-b border-border p-4">
          <div>
            <h2 className="text-sm font-semibold">Model prices</h2>
            <p className="mt-1 text-xs text-text-secondary">
              USD per 1K tokens. These override the configured default when costing a run.
            </p>
          </div>
          <Button variant="secondary" onClick={() => setShowPriceForm(true)}>+ Add price</Button>
        </header>
        <ul className="divide-y divide-border">
          {prices.isSuccess && prices.data.data.length === 0 ? (
            <li className="p-4 text-sm text-text-secondary">No prices yet; the default config price is used.</li>
          ) : null}
          {prices.isSuccess
            ? prices.data.data.map((price) => (
                <li key={`${price.provider}/${price.model}`} className="flex items-center gap-3 px-4 py-2 text-sm">
                  <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] text-text-muted">{price.provider}</span>
                  <span className="flex-1 font-mono text-xs">{price.model}</span>
                  <span className="font-mono text-xs text-text-secondary">
                    in {price.input_per_1k} · out {price.output_per_1k}
                  </span>
                </li>
              ))
            : null}
        </ul>
        {showPriceForm ? (
        <SidePanel
          title="Add model price"
          description="USD per 1K tokens; overrides the configured default for this model."
          onClose={() => setShowPriceForm(false)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setShowPriceForm(false)}>Cancel</Button>
              <Button type="submit" form="price-form" loading={savePrice.isPending}>Save price</Button>
            </>
          }
        >
        <form
          id="price-form"
          className="space-y-4 p-5"
          onSubmit={(event) => {
            event.preventDefault();
            savePrice.mutate();
          }}
        >
          <Field label="Provider" htmlFor="price-provider">
            <Input
              id="price-provider"
              value={priceForm.provider}
              onChange={(e) => setPriceForm({ ...priceForm, provider: e.target.value })}
            />
          </Field>
          <Field label="Model" htmlFor="price-model">
            <Input id="price-model" required value={priceForm.model} onChange={(e) => setPriceForm({ ...priceForm, model: e.target.value })} />
          </Field>
          <Field label="Input / 1K" htmlFor="price-in">
            <Input id="price-in" type="number" step="0.0001" min="0" value={priceForm.input} onChange={(e) => setPriceForm({ ...priceForm, input: e.target.value })} />
          </Field>
          <div className="flex items-end gap-2">
            <Field label="Output / 1K" htmlFor="price-out">
              <Input id="price-out" type="number" step="0.0001" min="0" value={priceForm.output} onChange={(e) => setPriceForm({ ...priceForm, output: e.target.value })} />
            </Field>
            <Button type="submit" loading={savePrice.isPending}>
              Save
            </Button>
          </div>
        </form>
        </SidePanel>
        ) : null}
      </section>
    </div>
  );
}
