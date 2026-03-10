import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useNodeGroups, useCreateNodeGroup, useDeleteNodeGroup } from '@/hooks/useNodeGroups';
import { nodeGroupSchema, type NodeGroupFormData } from '@/lib/schemas';
import type { NodeGroup } from '@/types';
import { Boxes, MoreHorizontal, Trash2 } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const fields: FieldConfig<NodeGroupFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'cdn-us-east' },
  { name: 'description', label: 'Description', placeholder: 'CDN nodes in US East region' },
  { name: 'tenant_id', label: 'Tenant ID', placeholder: 'Tenant UUID' },
];

export default function NodeGroupsPage() {
  const { data, isLoading } = useNodeGroups();
  const groups = data?.items || [];
  const createGroup = useCreateNodeGroup();
  const deleteGroup = useDeleteNodeGroup();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<NodeGroup | null>(null);

  const columns: Column<NodeGroup>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (g) => (
        <div>
          <p className="font-medium text-foreground">{g.name}</p>
          {g.description && <p className="text-xs text-muted-foreground">{g.description}</p>}
        </div>
      ),
    },
    {
      key: 'nodeCount',
      header: 'Nodes',
      render: (g) => <span className="text-sm">{g.nodeCount}</span>,
    },
    {
      key: 'createdAt',
      header: 'Created',
      render: (g) => new Date(g.createdAt).toLocaleDateString(),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (g) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(g)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Group
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Node Groups' }]}>
      <PageHeader
        title="Node Groups"
        description={`${data?.total ?? 0} groups`}
        icon={Boxes}
        action={{ label: 'Create Group', onClick: () => setCreateOpen(true) }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable data={groups} columns={columns} searchKeys={['name']} pageSize={10} />
      )}

      <FormDialog<NodeGroupFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create Node Group"
        schema={nodeGroupSchema}
        defaultValues={{ name: '', description: '', tenant_id: '' }}
        fields={fields}
        onSubmit={async (data) => { await createGroup.mutateAsync(data); setCreateOpen(false); }}
        isSubmitting={createGroup.isPending}
        submitLabel="Create Group"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => { if (deleteTarget) { await deleteGroup.mutateAsync(deleteTarget.id); setDeleteTarget(null); } }}
        isDeleting={deleteGroup.isPending}
      />
    </AppLayout>
  );
}
