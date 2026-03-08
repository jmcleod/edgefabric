import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { StatCard } from '@/components/ui/StatCard';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Progress } from '@/components/ui/progress';
import { nodes, bgpPeers, wireguardPeers } from '@/data/mockData';
import {
  Server,
  ArrowLeft,
  RefreshCw,
  Terminal,
  Power,
  Cpu,
  HardDrive,
  Clock,
  Globe,
  Radio,
  Shield,
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

export default function NodeDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const node = nodes.find((n) => n.id === id);

  if (!node) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Server className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Node not found</h2>
          <p className="text-muted-foreground mb-4">The requested node does not exist.</p>
          <Button onClick={() => navigate('/nodes')}>Back to Nodes</Button>
        </div>
      </AppLayout>
    );
  }

  const nodeBgpPeers = bgpPeers.filter((p) => p.nodeId === node.id);
  const nodeWgPeers = wireguardPeers.filter((p) => p.nodeId === node.id);

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'Infrastructure' },
        { label: 'Nodes', href: '/nodes' },
        { label: node.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/nodes')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Nodes
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Server className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{node.name}</h1>
                <StatusBadge status={node.status} />
              </div>
              <p className="text-muted-foreground mono-data">{node.hostname}</p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm">
              <Terminal className="mr-2 h-4 w-4" /> Console
            </Button>
            <Button variant="outline" size="sm">
              <RefreshCw className="mr-2 h-4 w-4" /> Restart
            </Button>
            <Button variant="destructive" size="sm">
              <Power className="mr-2 h-4 w-4" /> Shutdown
            </Button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4 mb-6">
        <StatCard title="CPU Usage" value={`${node.cpu}%`} icon={Cpu} variant={node.cpu > 80 ? 'warning' : 'default'} />
        <StatCard title="Memory" value={`${node.memory}%`} icon={HardDrive} variant={node.memory > 80 ? 'warning' : 'default'} />
        <StatCard title="Uptime" value={node.uptime} icon={Clock} />
        <StatCard title="Version" value={node.version} icon={Server} />
      </div>

      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="networking">Networking</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Node Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Hostname" value={node.hostname} mono />
                <InfoRow label="IPv4 Address" value={node.ipv4} mono />
                {node.ipv6 && <InfoRow label="IPv6 Address" value={node.ipv6} mono />}
                <InfoRow label="Location" value={node.location} />
                <InfoRow label="Region" value={node.region} />
                <InfoRow label="Last Seen" value={formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })} />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Resource Usage</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-muted-foreground">CPU</span>
                    <span className={node.cpu > 80 ? 'text-status-warning font-medium' : ''}>{node.cpu}%</span>
                  </div>
                  <Progress value={node.cpu} className="h-2" />
                </div>
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-muted-foreground">Memory</span>
                    <span className={node.memory > 80 ? 'text-status-warning font-medium' : ''}>{node.memory}%</span>
                  </div>
                  <Progress value={node.memory} className="h-2" />
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="networking" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <Radio className="h-4 w-4 text-primary" />
                  BGP Peers
                </CardTitle>
              </CardHeader>
              <CardContent>
                {nodeBgpPeers.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No BGP peers configured</p>
                ) : (
                  <div className="space-y-3">
                    {nodeBgpPeers.map((peer) => (
                      <div key={peer.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/30">
                        <div>
                          <code className="mono-data text-sm">{peer.peerIp}</code>
                          <p className="text-xs text-muted-foreground">AS{peer.peerAsn}</p>
                        </div>
                        <StatusBadge status={peer.status} size="sm" />
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base flex items-center gap-2">
                  <Shield className="h-4 w-4 text-primary" />
                  WireGuard Peers
                </CardTitle>
              </CardHeader>
              <CardContent>
                {nodeWgPeers.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No WireGuard peers configured</p>
                ) : (
                  <div className="space-y-3">
                    {nodeWgPeers.map((peer) => (
                      <div key={peer.id} className="p-3 rounded-lg bg-muted/30">
                        <code className="mono-data text-xs block truncate">{peer.publicKey}</code>
                        <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                          <span>↓ {(peer.rxBytes / 1e6).toFixed(0)} MB</span>
                          <span>↑ {(peer.txBytes / 1e6).toFixed(0)} MB</span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="logs">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Recent Logs</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="bg-muted/30 rounded-lg p-4 font-mono text-xs space-y-1 max-h-80 overflow-auto">
                <p className="text-muted-foreground">[2024-03-08 09:59:45] INFO: Health check passed</p>
                <p className="text-muted-foreground">[2024-03-08 09:55:00] INFO: BGP session established with 169.254.169.1</p>
                <p className="text-muted-foreground">[2024-03-08 09:50:00] INFO: WireGuard handshake completed</p>
                <p className="text-muted-foreground">[2024-03-08 09:45:00] INFO: Configuration reload successful</p>
                <p className="text-status-warning">[2024-03-08 09:40:00] WARN: High memory usage detected (82%)</p>
                <p className="text-muted-foreground">[2024-03-08 09:30:00] INFO: Service started</p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </AppLayout>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={`text-sm ${mono ? 'mono-data' : ''}`}>{value}</span>
    </div>
  );
}
