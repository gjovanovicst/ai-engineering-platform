import { RunView } from "@/components/run-view";

export default async function RunPage({
  params,
}: {
  params: Promise<{ projectId: string; runId: string }>;
}) {
  const { projectId, runId } = await params;
  return <RunView projectId={projectId} runId={runId} />;
}
