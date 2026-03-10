import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/hooks/useAuth';
import { useCDNSites } from '@/hooks/useCDN';
import type { CDNService } from '@/types';
import { Network } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

export default function TenantCDNDomainsPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data, isLoading } = useCDNSites(tenantId || undefined);
  const services = data?.items || [];

  // Flatten: show CDN services as domain groups (domain details are inside the service)
  const columns: Column<CDNService>[] = [
    {
      key: 'name',
      header: 'Service',
      render: (s) => (
        <div>
          <p className="font-medium text-foreground">{s.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{s.id}</p>
        </div>
      ),
    },
    {
      key: 'domainCount',
      header: 'Domains',
      render: (s) => <span className="text-sm font-medium">{s.domainCount}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (s) => <StatusBadge status={s.status} size="sm" />,
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'CDN' }, { label: 'Domains' }]}>
      <PageHeader
        title="CDN Domains"
        description="Domain names associated with your CDN services"
        icon={Network}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={services}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          onRowClick={(s) => navigate(`/tenant/cdn/services/${s.id}`)}
          emptyMessage="No CDN services with domains"
        />
      )}
    </AppLayout>
  );
}
