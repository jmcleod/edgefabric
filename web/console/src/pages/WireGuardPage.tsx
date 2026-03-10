import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Skeleton } from '@/components/ui/skeleton';
import { useWireGuardPeers } from '@/hooks/useWireGuard';
import type { WireGuardPeer } from '@/types';
import { Shield } from 'lucide-react';

export default function WireGuardPage() {
  // WireGuard peers endpoint is list-only (no CRUD), pass undefined to list all
  const { data, isLoading } = useWireGuardPeers('');

  // If the hook requires ownerId, we pass empty string to list all peers
  // The backend filters by owner_id if provided
  const peers = data?.items || [];

  const columns: Column<WireGuardPeer>[] = [
    {
      key: 'publicKey',
      header: 'Public Key',
      render: (peer) => (
        <code className="mono-data text-xs truncate block max-w-[200px]">{peer.publicKey}</code>
      ),
    },
    {
      key: 'endpoint',
      header: 'Endpoint',
      render: (peer) => <code className="mono-data text-sm">{peer.endpoint}</code>,
    },
    {
      key: 'allowedIps',
      header: 'Allowed IPs',
      render: (peer) => (
        <span className="text-sm">{peer.allowedIps.join(', ') || '\u2014'}</span>
      ),
    },
    {
      key: 'nodeId',
      header: 'Owner',
      render: (peer) => <code className="mono-data text-xs">{peer.nodeId}</code>,
    },
    {
      key: 'lastHandshake',
      header: 'Last Rotation',
      render: (peer) => (
        <span className="text-sm text-muted-foreground">
          {peer.lastHandshake ? new Date(peer.lastHandshake).toLocaleDateString() : '\u2014'}
        </span>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Networking' }, { label: 'WireGuard' }]}>
      <PageHeader
        title="WireGuard Peers"
        description="Overlay network peer connections"
        icon={Shield}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={peers}
          columns={columns}
          searchKeys={['publicKey', 'endpoint', 'nodeId']}
          pageSize={10}
          emptyMessage="No WireGuard peers found"
        />
      )}
    </AppLayout>
  );
}
