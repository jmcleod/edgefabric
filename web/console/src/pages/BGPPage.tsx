import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useNodes } from '@/hooks/useNodes';
import { useBGPSessions, useDeleteBGPPeer } from '@/hooks/useBGP';
import type { BGPPeer } from '@/types';
import { Radio, MoreHorizontal, Trash2 } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

export default function BGPPage() {
  const { data: nodesData } = useNodes();
  const nodes = nodesData?.items || [];
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');

  const { data: bgpData, isLoading } = useBGPSessions(selectedNodeId || undefined);
  const peers = bgpData?.items || [];
  const deletePeer = useDeleteBGPPeer();
  const [deleteTarget, setDeleteTarget] = useState<BGPPeer | null>(null);

  const columns: Column<BGPPeer>[] = [
    {
      key: 'peerIp',
      header: 'Peer Address',
      render: (peer) => <code className="mono-data text-sm">{peer.peerIp}</code>,
    },
    {
      key: 'peerAsn',
      header: 'Peer ASN',
      render: (peer) => <span className="text-sm">AS{peer.peerAsn}</span>,
    },
    {
      key: 'localAsn',
      header: 'Local ASN',
      render: (peer) => <span className="text-sm">AS{peer.localAsn}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (peer) => (
        <StatusBadge
          status={peer.status === 'established' ? 'healthy' : peer.status === 'idle' ? 'warning' : 'unknown'}
          size="sm"
        />
      ),
    },
    {
      key: 'prefixesReceived',
      header: 'Prefixes',
      render: (peer) => <span className="text-sm">{peer.prefixesReceived} recv / {peer.prefixesAdvertised} adv</span>,
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (peer) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(peer)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Peer
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Networking' }, { label: 'BGP Peers' }]}>
      <PageHeader
        title="BGP Peers"
        description="Border Gateway Protocol sessions by node"
        icon={Radio}
      />

      <div className="mb-4 max-w-xs">
        <Select value={selectedNodeId} onValueChange={setSelectedNodeId}>
          <SelectTrigger>
            <SelectValue placeholder="Select a node..." />
          </SelectTrigger>
          <SelectContent>
            {nodes.map((node) => (
              <SelectItem key={node.id} value={node.id}>
                {node.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {!selectedNodeId ? (
        <div className="text-center py-16 text-muted-foreground">
          Select a node above to view its BGP peers.
        </div>
      ) : isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable data={peers} columns={columns} searchKeys={['peerIp']} pageSize={10} emptyMessage="No BGP peers configured for this node" />
      )}

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget ? `BGP peer ${deleteTarget.peerIp}` : undefined}
        onConfirm={async () => { if (deleteTarget) { await deletePeer.mutateAsync(deleteTarget.id); setDeleteTarget(null); } }}
        isDeleting={deletePeer.isPending}
      />
    </AppLayout>
  );
}
