import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useAPIKeys, useCreateAPIKey, useDeleteAPIKey } from '@/hooks/useAPIKeys';
import { apiKeySchema, type APIKeyFormData } from '@/lib/schemas';
import type { APIKey } from '@/types';
import { Key, MoreHorizontal, Trash2, Copy, Check } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { toast } from 'sonner';

const apiKeyFields: FieldConfig<APIKeyFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'ci-deploy-key' },
  {
    name: 'role',
    label: 'Role',
    type: 'select',
    options: [
      { label: 'Read Only', value: 'readonly' },
      { label: 'Admin', value: 'admin' },
      { label: 'Superuser', value: 'superuser' },
    ],
  },
  { name: 'expires_at', label: 'Expires At', placeholder: 'YYYY-MM-DD (optional)' },
];

export default function APIKeysPage() {
  const { data, isLoading } = useAPIKeys();
  const keys = data?.items || [];
  const createKey = useCreateAPIKey();
  const deleteKey = useDeleteAPIKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<APIKey | null>(null);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (newKey) {
      await navigator.clipboard.writeText(newKey);
      setCopied(true);
      toast.success('API key copied to clipboard');
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const columns: Column<APIKey>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (k) => (
        <div>
          <p className="font-medium text-foreground">{k.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{k.prefix}...</p>
        </div>
      ),
    },
    {
      key: 'scopes',
      header: 'Scopes',
      render: (k) => (
        <div className="flex gap-1 flex-wrap">
          {k.scopes.map((s) => (
            <span key={s} className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium">
              {s}
            </span>
          ))}
        </div>
      ),
    },
    {
      key: 'createdAt',
      header: 'Created',
      render: (k) => <span className="text-sm text-muted-foreground">{new Date(k.createdAt).toLocaleDateString()}</span>,
    },
    {
      key: 'lastUsed',
      header: 'Last Used',
      render: (k) => (
        <span className="text-sm text-muted-foreground">
          {k.lastUsed ? new Date(k.lastUsed).toLocaleDateString() : 'Never'}
        </span>
      ),
    },
    {
      key: 'expiresAt',
      header: 'Expires',
      render: (k) => (
        <span className="text-sm text-muted-foreground">
          {k.expiresAt ? new Date(k.expiresAt).toLocaleDateString() : 'Never'}
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
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(k)}>
              <Trash2 className="mr-2 h-4 w-4" /> Revoke Key
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Settings' }, { label: 'API Keys' }]}>
      <PageHeader
        title="API Keys"
        description={`${data?.total ?? 0} keys`}
        icon={Key}
        action={{ label: 'Create API Key', onClick: () => setCreateOpen(true) }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={keys}
          columns={columns}
          searchKeys={['name', 'prefix']}
          pageSize={10}
          emptyMessage="No API keys created"
        />
      )}

      {/* Create Dialog */}
      <FormDialog<APIKeyFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create API Key"
        description="Generate a new API key for programmatic access."
        schema={apiKeySchema}
        defaultValues={{ name: '', role: 'readonly', expires_at: '' }}
        fields={apiKeyFields}
        onSubmit={async (data) => {
          const result = await createKey.mutateAsync(data);
          setCreateOpen(false);
          // Show the key once — it cannot be retrieved later
          if (result?.raw_key) {
            setNewKey(result.raw_key);
          }
        }}
        isSubmitting={createKey.isPending}
        submitLabel="Create Key"
      />

      {/* One-time key display */}
      <Dialog open={!!newKey} onOpenChange={(open) => !open && setNewKey(null)}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>API Key Created</DialogTitle>
            <DialogDescription>
              Copy this key now. It will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="flex gap-2">
            <Input
              value={newKey || ''}
              readOnly
              className="font-mono text-sm"
            />
            <Button variant="outline" size="icon" onClick={handleCopy}>
              {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewKey(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete/Revoke Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Revoke API Key"
        entityName={deleteTarget?.name}
        description={deleteTarget ? `Are you sure you want to revoke "${deleteTarget.name}"? Any integrations using this key will stop working.` : undefined}
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
