import { PullRequestDetailView } from "@/components/pull-request-detail";

export default async function PullRequestPage({
  params,
}: {
  params: Promise<{ projectId: string; prId: string }>;
}) {
  const { projectId, prId } = await params;
  return <PullRequestDetailView projectId={projectId} prId={prId} />;
}
