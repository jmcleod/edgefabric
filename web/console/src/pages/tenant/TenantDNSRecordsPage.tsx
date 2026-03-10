import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useAuth } from '@/hooks/useAuth';
import { useDNSZones, useDNSRecords, useCreateDNSRecord, useUpdateDNSRecord, useDeleteDNSRecord } from '@/hooks/useDNS';
import { dnsRecordSchema, type DNSRecordFormData } from '@/lib/schemas';
import type { DNSRecord } from '@/types';
import { FileText, MoreHorizontal, Pencil, Trash2 } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const recordFields: FieldConfig<DNSRecordFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'www' },
  {
    name: 'type',
    label: 'Type',
    type: 'select',
    options: [
      { label: 'A', value: 'A' },
      { label: 'AAAA', value: 'AAAA' },
      { label: 'CNAME', value: 'CNAME' },
      { label: 'MX', value: 'MX' },
      { label: 'TXT', value: 'TXT' },
      { label: 'NS', value: 'NS' },
      { label: 'SRV', value: 'SRV' },
      { label: 'CAA', value: 'CAA' },
      { label: 'PTR', value: 'PTR' },
    ],
  },
  { name: 'value', label: 'Value', placeholder: '192.0.2.1' },
  { name: 'ttl', label: 'TTL (seconds)', type: 'number', placeholder: '3600' },
  { name: 'priority', label: 'Priority', type: 'number', placeholder: 'MX/SRV priority (optional)' },
];

export default function TenantDNSRecordsPage() {
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data: zonesData } = useDNSZones(tenantId || undefined);
  const zones = zonesData?.items || [];

  const [selectedZoneId, setSelectedZoneId] = useState<string>('');
  const { data: recordsData, isLoading } = useDNSRecords(selectedZoneId || undefined);
  const records = recordsData?.items || [];

  const createRecord = useCreateDNSRecord(selectedZoneId);
  const updateRecord = useUpdateDNSRecord();
  const deleteRecord = useDeleteDNSRecord();

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<DNSRecord | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DNSRecord | null>(null);

  const columns: Column<DNSRecord>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (r) => <code className="mono-data text-sm">{r.name}</code>,
    },
    {
      key: 'type',
      header: 'Type',
      render: (r) => (
        <span className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium">
          {r.type}
        </span>
      ),
    },
    {
      key: 'value',
      header: 'Value',
      render: (r) => <code className="mono-data text-sm truncate block max-w-[300px]">{r.value}</code>,
    },
    {
      key: 'ttl',
      header: 'TTL',
      render: (r) => <span className="text-sm text-muted-foreground">{r.ttl}s</span>,
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (r) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setEditTarget(r)}>
              <Pencil className="mr-2 h-4 w-4" /> Edit Record
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(r)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Record
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'DNS' }, { label: 'Records' }]}>
      <PageHeader
        title="DNS Records"
        description="Manage records across your DNS zones"
        icon={FileText}
        action={selectedZoneId ? { label: 'Add Record', onClick: () => setCreateOpen(true) } : undefined}
      />

      <div className="mb-4 max-w-xs">
        <Select value={selectedZoneId} onValueChange={setSelectedZoneId}>
          <SelectTrigger>
            <SelectValue placeholder="Select a zone..." />
          </SelectTrigger>
          <SelectContent>
            {zones.map((zone) => (
              <SelectItem key={zone.id} value={zone.id}>
                {zone.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {!selectedZoneId ? (
        <div className="text-center py-16 text-muted-foreground">
          Select a zone above to view and manage its records.
        </div>
      ) : isLoading ? (
        <Skeleton className="h-64" />
      ) : (
        <DataTable
          data={records}
          columns={columns}
          searchKeys={['name', 'type', 'value']}
          pageSize={20}
          emptyMessage="No records in this zone"
        />
      )}

      {/* Create Record Dialog */}
      <FormDialog<DNSRecordFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add DNS Record"
        schema={dnsRecordSchema}
        defaultValues={{ name: '', type: 'A', value: '', ttl: 3600 }}
        fields={recordFields}
        onSubmit={async (data) => {
          await createRecord.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createRecord.isPending}
        submitLabel="Add Record"
      />

      {/* Edit Record Dialog */}
      {editTarget && (
        <FormDialog<DNSRecordFormData>
          open={!!editTarget}
          onOpenChange={(open) => !open && setEditTarget(null)}
          title="Edit DNS Record"
          schema={dnsRecordSchema}
          defaultValues={{
            name: editTarget.name,
            type: editTarget.type as DNSRecordFormData['type'],
            value: editTarget.value,
            ttl: editTarget.ttl,
            priority: editTarget.priority,
          }}
          fields={recordFields}
          onSubmit={async (data) => {
            await updateRecord.mutateAsync({ id: editTarget.id, body: data });
            setEditTarget(null);
          }}
          isSubmitting={updateRecord.isPending}
          submitLabel="Save Changes"
        />
      )}

      {/* Delete Record Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget ? `${deleteTarget.type} record "${deleteTarget.name}"` : undefined}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteRecord.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteRecord.isPending}
      />
    </AppLayout>
  );
}
