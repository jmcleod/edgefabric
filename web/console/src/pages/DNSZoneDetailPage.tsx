import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import {
  useDNSZone,
  useDNSRecords,
  useCreateDNSRecord,
  useUpdateDNSRecord,
  useDeleteDNSRecord,
  useDeleteDNSZone,
} from '@/hooks/useDNS';
import { dnsRecordSchema, type DNSRecordFormData } from '@/lib/schemas';
import type { DNSRecord } from '@/types';
import { Globe, ArrowLeft, Trash2, MoreHorizontal, Pencil, Plus } from 'lucide-react';
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

export default function DNSZoneDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: zone, isLoading: zoneLoading, error } = useDNSZone(id);
  const { data: recordsData, isLoading: recordsLoading } = useDNSRecords(id);
  const records = recordsData?.items || [];

  const createRecord = useCreateDNSRecord(id || '');
  const updateRecord = useUpdateDNSRecord();
  const deleteRecord = useDeleteDNSRecord();
  const deleteZone = useDeleteDNSZone();

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<DNSRecord | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DNSRecord | null>(null);
  const [deleteZoneOpen, setDeleteZoneOpen] = useState(false);

  if (zoneLoading) {
    return (
      <AppLayout>
        <Skeleton className="h-12 w-64 mb-6" />
        <Skeleton className="h-48" />
      </AppLayout>
    );
  }

  if (!zone || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Globe className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Zone not found</h2>
          <p className="text-muted-foreground mb-4">The requested DNS zone does not exist.</p>
          <Button onClick={() => navigate('/tenant/dns/zones')}>Back to Zones</Button>
        </div>
      </AppLayout>
    );
  }

  const recordColumns: Column<DNSRecord>[] = [
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
    <AppLayout
      breadcrumbs={[
        { label: 'DNS' },
        { label: 'Zones', href: '/tenant/dns/zones' },
        { label: zone.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/tenant/dns/zones')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Zones
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Globe className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{zone.name}</h1>
                <StatusBadge status={zone.status} />
              </div>
              <p className="text-muted-foreground text-sm">Serial: {zone.serial} &middot; {zone.recordCount} records</p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="destructive" size="sm" onClick={() => setDeleteZoneOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Zone
            </Button>
          </div>
        </div>
      </div>

      {/* Zone Info Card */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">Zone Information</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <InfoItem label="Zone ID" value={zone.id} mono />
          <InfoItem label="Tenant ID" value={zone.tenantId} mono />
          <InfoItem label="Serial" value={String(zone.serial)} />
          <InfoItem label="Created" value={new Date(zone.createdAt).toLocaleDateString()} />
        </CardContent>
      </Card>

      {/* Zone Transfer Card */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">Zone Transfer (AXFR)</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <div>
              <p className="text-xs text-muted-foreground mb-0.5">Allowed Transfer IPs</p>
              {zone.transferAllowedIPs.length > 0 ? (
                <div className="flex flex-wrap gap-1.5">
                  {zone.transferAllowedIPs.map((ip) => (
                    <span key={ip} className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium mono-data">
                      {ip}
                    </span>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">None &mdash; transfers disabled</p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Records Table */}
      <PageHeader
        title="DNS Records"
        description={`${recordsData?.total ?? 0} records`}
        action={{ label: 'Add Record', onClick: () => setCreateOpen(true) }}
      />

      {recordsLoading ? (
        <Skeleton className="h-64" />
      ) : (
        <DataTable
          data={records}
          columns={recordColumns}
          searchKeys={['name', 'type', 'value']}
          pageSize={20}
          emptyMessage="No DNS records in this zone"
        />
      )}

      {/* Create Record Dialog */}
      <FormDialog<DNSRecordFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add DNS Record"
        description={`Add a record to ${zone.name}`}
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

      {/* Delete Zone Confirmation */}
      <DeleteConfirmDialog
        open={deleteZoneOpen}
        onOpenChange={setDeleteZoneOpen}
        title="Delete DNS Zone"
        description={`Are you sure you want to delete zone "${zone.name}" and all its records? This action cannot be undone.`}
        onConfirm={async () => {
          await deleteZone.mutateAsync(zone.id);
          navigate('/tenant/dns/zones');
        }}
        isDeleting={deleteZone.isPending}
      />
    </AppLayout>
  );
}

function InfoItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
      <p className={`text-sm font-medium ${mono ? 'mono-data' : ''}`}>{value}</p>
    </div>
  );
}
