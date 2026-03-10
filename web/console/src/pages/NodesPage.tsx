import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useNodes, useCreateNode, useDeleteNode, useNodeAction } from '@/hooks/useNodes';
import { nodeSchema, type NodeFormData } from '@/lib/schemas';
import type { Node } from '@/types';
import { Server, Eye, MoreHorizontal, Trash2, RefreshCw, Power, PlayCircle } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const nodeFields: FieldConfig<NodeFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'edge-us-east-01' },
  { name: 'hostname', label: 'Hostname', placeholder: 'edge-us-east-01.example.com' },
  { name: 'public_ip', label: 'Public IP', placeholder: '203.0.113.10' },
  { name: 'ssh_port', label: 'SSH Port', type: 'number', placeholder: '22' },
  { name: 'ssh_user', label: 'SSH User', placeholder: 'root' },
  { name: 'region', label: 'Region', placeholder: 'us-east-1' },
  { name: 'provider', label: 'Provider', placeholder: 'aws' },
  { name: 'tenant_id', label: 'Tenant ID', placeholder: 'Optional', description: 'Assign to a specific tenant' },
];

export default function NodesPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useNodes();
  const nodes = data?.items || [];
  const createNode = useCreateNode();
  const deleteNode = useDeleteNode();
  const nodeAction = useNodeAction();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Node | null>(null);

  const columns: Column<Node>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (node) => (
        <div>
          <p className="font-medium text-foreground">{node.name}</p>
          <p className="text-xs text-muted-foreground">{node.hostname}</p>
        </div>
      ),
    },
    {
      key: 'ipv4',
      header: 'IP Address',
      render: (node) => (
        <code className="mono-data text-sm">{node.ipv4}</code>
      ),
    },
    { key: 'region', header: 'Region' },
    {
      key: 'status',
      header: 'Status',
      render: (node) => <StatusBadge status={node.status} size="sm" />,
    },
    {
      key: 'version',
      header: 'Version',
      className: 'hidden md:table-cell',
      render: (node) => <code className="mono-data text-xs">{node.version}</code>,
    },
    {
      key: 'lastSeen',
      header: 'Last Seen',
      className: 'hidden lg:table-cell',
      render: (node) => (
        <span className="text-muted-foreground text-sm">
          {node.lastSeen
            ? formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })
            : '\u2014'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (node) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/nodes/${node.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => nodeAction.mutate({ id: node.id, action: 'restart' })}>
              <RefreshCw className="mr-2 h-4 w-4" /> Restart
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => nodeAction.mutate({ id: node.id, action: 'start' })}>
              <PlayCircle className="mr-2 h-4 w-4" /> Start
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => nodeAction.mutate({ id: node.id, action: 'stop' })}>
              <Power className="mr-2 h-4 w-4" /> Stop
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(node)}>
              <Trash2 className="mr-2 h-4 w-4" /> Remove Node
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Nodes' }]}>
      <PageHeader
        title="Nodes"
        description={`${data?.total ?? 0} nodes across all regions`}
        icon={Server}
        action={{
          label: 'Add Node',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={nodes}
          columns={columns}
          searchKeys={['name', 'hostname', 'ipv4', 'region']}
          pageSize={10}
          onRowClick={(node) => navigate(`/nodes/${node.id}`)}
        />
      )}

      {/* Create Dialog */}
      <FormDialog<NodeFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add Node"
        description="Register a new node in the platform."
        schema={nodeSchema}
        defaultValues={{ name: '', hostname: '', public_ip: '', ssh_port: 22, ssh_user: 'root', region: '', provider: '', tenant_id: '' }}
        fields={nodeFields}
        onSubmit={async (data) => {
          await createNode.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createNode.isPending}
        submitLabel="Add Node"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteNode.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteNode.isPending}
      />
    </AppLayout>
  );
}
