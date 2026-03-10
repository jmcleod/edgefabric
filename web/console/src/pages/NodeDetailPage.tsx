import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { StatCard } from '@/components/ui/StatCard';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useNode, useDeleteNode, useNodeAction } from '@/hooks/useNodes';
import { useBGPSessions } from '@/hooks/useBGP';
import { useWireGuardPeers } from '@/hooks/useWireGuard';
import { useProvisioningJobs } from '@/hooks/useProvisioning';
import { useState } from 'react';
import {
  Server,
  ArrowLeft,
  RefreshCw,
  Power,
  Clock,
  Radio,
  Shield,
  Tag,
  Globe,
  Trash2,
  PlayCircle,
  RotateCcw,
  ArrowUpCircle,
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

export default function NodeDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: node, isLoading, error } = useNode(id);
  const { data: bgpData } = useBGPSessions(id);
  const { data: wgData } = useWireGuardPeers(id);
  const { data: jobsData } = useProvisioningJobs(id);
  const nodeAction = useNodeAction();
  const deleteNode = useDeleteNode();
  const [deleteOpen, setDeleteOpen] = useState(false);

  const nodeBgpPeers = bgpData?.items || [];
  const nodeWgPeers = wgData?.items || [];
  const nodeJobs = jobsData?.items || [];

  if (isLoading) {
    return (
      <AppLayout>
        <Skeleton className="h-12 w-64 mb-6" />
        <div className="grid gap-4 md:grid-cols-4 mb-6">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
        <Skeleton className="h-96" />
      </AppLayout>
    );
  }

  if (!node || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Server className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Node not found</h2>
          <p className="text-muted-foreground mb-4">The requested node does not exist.</p>
          <Button onClick={() => navigate('/nodes')}>Back to Nodes</Button>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'Infrastructure' },
        { label: 'Nodes', href: '/nodes' },
        { label: node.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/nodes')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Nodes
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Server className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{node.name}</h1>
                <StatusBadge status={node.status} />
              </div>
              <p className="text-muted-foreground mono-data">{node.hostname}</p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => nodeAction.mutate({ id: node.id, action: 'restart' })}>
              <RefreshCw className="mr-2 h-4 w-4" /> Restart
            </Button>
            <Button variant="outline" size="sm" onClick={() => nodeAction.mutate({ id: node.id, action: 'upgrade' })}>
              <ArrowUpCircle className="mr-2 h-4 w-4" /> Upgrade
            </Button>
            <Button variant="outline" size="sm" onClick={() => nodeAction.mutate({ id: node.id, action: 'reprovision' })}>
              <RotateCcw className="mr-2 h-4 w-4" /> Reprovision
            </Button>
            <Button variant="destructive" size="sm" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete
            </Button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4 mb-6">
        <StatCard title="Region" value={node.region} icon={Globe} />
        <StatCard title="Version" value={node.version} icon={Tag} />
        <StatCard title="Uptime" value={node.uptime} icon={Clock} />
        <StatCard
          title="Last Seen"
          value={node.lastSeen
            ? formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })
            : '\u2014'}
          icon={Server}
        />
      </div>

      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="networking">Networking</TabsTrigger>
          <TabsTrigger value="jobs">Jobs ({nodeJobs.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Node Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Hostname" value={node.hostname} mono />
                <InfoRow label="IPv4 Address" value={node.ipv4} mono />
                {node.ipv6 && <InfoRow label="IPv6 / WireGuard IP" value={node.ipv6} mono />}
                <InfoRow label="Location" value={node.location} />
                <InfoRow label="Region" value={node.region} />
                <InfoRow label="Last Seen" value={
                  node.lastSeen
                    ? formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })
                    : '\u2014'
                } />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Metadata</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Node ID" value={node.id} mono />
                <InfoRow label="Version" value={node.version} />
                <InfoRow label="Uptime" value={node.uptime} />
                {node.tenantId && <InfoRow label="Tenant ID" value={node.tenantId} mono />}
                {node.nodeGroupId && <InfoRow label="Node Group" value={node.nodeGroupId} mono />}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="networking" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <Radio className="h-4 w-4 text-primary" />
                  BGP Peers
                </CardTitle>
              </CardHeader>
              <CardContent>
                {nodeBgpPeers.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No BGP peers configured</p>
                ) : (
                  <div className="space-y-3">
                    {nodeBgpPeers.map((peer) => (
                      <div key={peer.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                        <div>
                          <code className="mono-data text-sm">{peer.peerIp}</code>
                          <p className="text-xs text-muted-foreground">AS{peer.peerAsn}</p>
                        </div>
                        <StatusBadge status={peer.status === 'established' ? 'healthy' : 'warning'} size="sm" />
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <Shield className="h-4 w-4 text-primary" />
                  WireGuard Peers
                </CardTitle>
              </CardHeader>
              <CardContent>
                {nodeWgPeers.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No WireGuard peers configured</p>
                ) : (
                  <div className="space-y-3">
                    {nodeWgPeers.map((peer) => (
                      <div key={peer.id} className="p-3 rounded-lg bg-muted/30">
                        <code className="mono-data text-xs block truncate">{peer.publicKey}</code>
                        <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                          <span>Endpoint: {peer.endpoint}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="jobs">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Provisioning Jobs</CardTitle>
            </CardHeader>
            <CardContent>
              {nodeJobs.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">No provisioning jobs found for this node.</p>
              ) : (
                <div className="space-y-3">
                  {nodeJobs.map((job) => (
                    <div key={job.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                      <div>
                        <p className="text-sm font-medium">{job.type}</p>
                        <p className="text-xs text-muted-foreground">
                          {job.createdAt ? new Date(job.createdAt).toLocaleString() : '\u2014'}
                        </p>
                      </div>
                      <div className="flex items-center gap-3">
                        {job.progress > 0 && job.progress < 100 && (
                          <span className="text-xs text-muted-foreground">{job.progress}%</span>
                        )}
                        <StatusBadge
                          status={
                            job.status === 'completed' ? 'healthy' :
                            job.status === 'failed' ? 'critical' :
                            job.status === 'running' ? 'syncing' : 'unknown'
                          }
                          size="sm"
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        entityName={node.name}
        onConfirm={async () => {
          await deleteNode.mutateAsync(node.id);
          navigate('/nodes');
        }}
        isDeleting={deleteNode.isPending}
      />
    </AppLayout>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={`text-sm ${mono ? 'mono-data' : ''}`}>{value}</span>
    </div>
  );
}
