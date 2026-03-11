import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Skeleton } from '@/components/ui/skeleton';
import { useWireGuardPeers } from '@/hooks/useWireGuard';
import { useStatus } from '@/hooks/useStatus';
import type { WireGuardPeer } from '@/types';
import { Shield } from 'lucide-react';

export default function WireGuardPage() {
  // WireGuard peers endpoint is list-only (no CRUD), pass undefined to list all
  const { data, isLoading } = useWireGuardPeers('');
  const { data: statusData } = useStatus();

  // If the hook requires ownerId, we pass empty string to list all peers
  // The backend filters by owner_id if provided
  const peers = data?.items || [];

  // Topology from status endpoint (raw backend data).
  const topology = statusData?.raw?.overlay_topology || 'hub-spoke';

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

      <div className="mb-4 flex items-center gap-2">
        <span className="text-sm text-muted-foreground">Overlay Topology:</span>
        <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
          topology === 'mesh'
            ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
            : 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
        }`}>
          {topology === 'mesh' ? 'Full Mesh' : 'Hub-Spoke'}
        </span>
      </div>

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
