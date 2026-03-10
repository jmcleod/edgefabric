import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useSSHKeys, useCreateSSHKey, useDeleteSSHKey, useRotateSSHKey, useDeploySSHKey } from '@/hooks/useSSHKeys';
import { sshKeySchema, type SSHKeyFormData } from '@/lib/schemas';
import type { SSHKey } from '@/types';
import { KeyRound, MoreHorizontal, Trash2, RefreshCw, Upload } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const keyFields: FieldConfig<SSHKeyFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'production-deploy-key' },
];

export default function SSHKeysPage() {
  const { data, isLoading } = useSSHKeys();
  const keys = data?.items || [];
  const createKey = useCreateSSHKey();
  const deleteKey = useDeleteSSHKey();
  const rotateKey = useRotateSSHKey();
  const deployKey = useDeploySSHKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SSHKey | null>(null);

  const columns: Column<SSHKey>[] = [
    {
      key: 'name',
      header: 'Key Name',
      render: (k) => (
        <div>
          <p className="font-medium text-foreground">{k.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{k.fingerprint}</p>
        </div>
      ),
    },
    {
      key: 'publicKey',
      header: 'Public Key',
      render: (k) => (
        <code className="mono-data text-xs truncate block max-w-[200px]">{k.publicKey}</code>
      ),
    },
    {
      key: 'createdAt',
      header: 'Created',
      render: (k) => <span className="text-sm text-muted-foreground">{new Date(k.createdAt).toLocaleDateString()}</span>,
    },
    {
      key: 'lastRotatedAt',
      header: 'Last Rotated',
      render: (k) => (
        <span className="text-sm text-muted-foreground">
          {k.lastRotatedAt ? new Date(k.lastRotatedAt).toLocaleDateString() : '\u2014'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (k) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => rotateKey.mutate({ id: k.id, action: 'rotate' })}>
              <RefreshCw className="mr-2 h-4 w-4" /> Rotate Key
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => deployKey.mutate({ id: k.id, action: 'deploy' })}>
              <Upload className="mr-2 h-4 w-4" /> Deploy to Nodes
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(k)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Key
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Operations' }, { label: 'SSH Keys' }]}>
      <PageHeader
        title="SSH Keys"
        description={`${data?.total ?? 0} keys`}
        icon={KeyRound}
        action={{ label: 'Create SSH Key', onClick: () => setCreateOpen(true) }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={keys}
          columns={columns}
          searchKeys={['name', 'fingerprint']}
          pageSize={10}
          emptyMessage="No SSH keys configured"
        />
      )}

      <FormDialog<SSHKeyFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create SSH Key"
        description="Generate a new SSH key pair. The private key will be stored securely."
        schema={sshKeySchema}
        defaultValues={{ name: '' }}
        fields={keyFields}
        onSubmit={async (data) => {
          await createKey.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createKey.isPending}
        submitLabel="Create Key"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteKey.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteKey.isPending}
      />
    </AppLayout>
  );
}
