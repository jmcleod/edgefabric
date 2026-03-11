import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useStatus } from '@/hooks/useStatus';
import { Settings, Server, Shield, Bell, Database } from 'lucide-react';
import { StatusBadge } from '@/components/ui/StatusBadge';

export default function SettingsPage() {
  const { data: statusData, isLoading } = useStatus();
  const raw = statusData?.raw;

  return (
    <AppLayout breadcrumbs={[{ label: 'Administration' }, { label: 'Settings' }]}>
      <PageHeader
        title="System Settings"
        description="Platform configuration (read-only)"
        icon={Settings}
      />

      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-48" />
          <Skeleton className="h-48" />
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Server className="h-4 w-4" />
                Platform Info
              </CardTitle>
              <CardDescription>Current system configuration</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="Version" value={raw?.version || 'unknown'} />
              <InfoRow label="Commit" value={raw?.commit ? raw.commit.substring(0, 8) : 'unknown'} />
              <InfoRow label="Schema Version" value={String(raw?.schema_version || 0)} />
              <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
                <span className="text-sm text-muted-foreground">Leader Status</span>
                <StatusBadge status={raw?.is_leader ? 'leader' : 'follower'} size="sm" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Security
              </CardTitle>
              <CardDescription>Security and access settings</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="TLS Enforcement" value="Enabled" />
              <InfoRow label="Rate Limiting" value="Enabled" />
              <InfoRow label="CORS" value="Configured" />
              <InfoRow label="TOTP Support" value="Available" />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Database className="h-4 w-4" />
                Infrastructure
              </CardTitle>
              <CardDescription>Resource summary</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="Total Nodes" value={String(raw?.node_count || 0)} />
              <InfoRow label="Total Gateways" value={String(raw?.gateway_count || 0)} />
              <InfoRow label="Total Tenants" value={String(raw?.tenant_count || 0)} />
              <InfoRow label="DNS Zones" value={String(raw?.dns_zone_count || 0)} />
              <InfoRow label="CDN Services" value={String(raw?.cdn_site_count || 0)} />
              <InfoRow label="Routes" value={String(raw?.route_count || 0)} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                <Bell className="h-4 w-4" />
                Notifications
              </CardTitle>
              <CardDescription>Alert configuration</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <InfoRow label="Email Alerts" value="Not configured" />
              <InfoRow label="Webhook Alerts" value="Not configured" />
              <InfoRow label="Audit Retention" value="90 days" />
            </CardContent>
          </Card>
        </div>
      )}
    </AppLayout>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  );
}
