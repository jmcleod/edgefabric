import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { provisioningJobs } from '@/data/mockData';
import { RefreshCw, CheckCircle, XCircle, Clock, Play } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

const statusConfig = {
  pending: { icon: Clock, color: 'text-status-warning', bg: 'bg-status-warning/10' },
  running: { icon: Play, color: 'text-status-syncing', bg: 'bg-status-syncing/10' },
  completed: { icon: CheckCircle, color: 'text-status-healthy', bg: 'bg-status-healthy/10' },
  failed: { icon: XCircle, color: 'text-status-critical', bg: 'bg-status-critical/10' },
};

export default function ProvisioningJobsPage() {
  return (
    <AppLayout breadcrumbs={[{ label: 'Operations' }, { label: 'Provisioning Jobs' }]}>
      <PageHeader
        title="Provisioning Jobs"
        description="Node deployment and configuration tasks"
        icon={RefreshCw}
      />

      <div className="space-y-4">
        {provisioningJobs.map((job) => {
          const config = statusConfig[job.status];
          const Icon = config.icon;

          return (
            <Card key={job.id}>
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${config.bg}`}>
                      <Icon className={`h-5 w-5 ${config.color}`} />
                    </div>
                    <div>
                      <CardTitle className="text-base">{job.targetName}</CardTitle>
                      <p className="text-sm text-muted-foreground">
                        {job.type.replace(/_/g, ' ')} • {job.id}
                      </p>
                    </div>
                  </div>
                  <div className="text-right">
                    <span className={`text-sm font-medium ${config.color}`}>
                      {job.status.charAt(0).toUpperCase() + job.status.slice(1)}
                    </span>
                    <p className="text-xs text-muted-foreground">
                      {formatDistanceToNow(new Date(job.createdAt), { addSuffix: true })}
                    </p>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-4">
                  <Progress value={job.progress} className="flex-1 h-2" />
                  <span className="text-sm font-medium w-12 text-right">{job.progress}%</span>
                </div>

                {job.logs.length > 0 && (
                  <div className="rounded-lg bg-muted/30 p-3">
                    <p className="text-xs font-medium text-muted-foreground mb-2">Recent Logs</p>
                    <div className="space-y-1 font-mono text-xs max-h-24 overflow-auto">
                      {job.logs.slice(-4).map((log, i) => (
                        <p key={i} className={log.includes('Error') ? 'text-status-critical' : 'text-muted-foreground'}>
                          {log}
                        </p>
                      ))}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>
    </AppLayout>
  );
}
