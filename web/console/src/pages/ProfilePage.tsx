import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useAuth } from '@/hooks/useAuth';
import { UserCircle, Shield, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { apiPost } from '@/lib/api';

export default function ProfilePage() {
  const { user } = useAuth();

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);

  const [totpCode, setTotpCode] = useState('');
  const [togglingTotp, setTogglingTotp] = useState(false);

  const handleChangePassword = async () => {
    if (newPassword !== confirmPassword) {
      toast.error('Passwords do not match');
      return;
    }
    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters');
      return;
    }
    setChangingPassword(true);
    try {
      await apiPost('/api/v1/auth/change-password', {
        current_password: currentPassword,
        new_password: newPassword,
      });
      toast.success('Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: any) {
      toast.error(err.message || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  const handleToggleTotp = async () => {
    setTogglingTotp(true);
    try {
      if (user?.totpEnabled) {
        await apiPost('/api/v1/auth/totp/disable', { code: totpCode });
        toast.success('TOTP disabled');
      } else {
        await apiPost('/api/v1/auth/totp/enable', { code: totpCode });
        toast.success('TOTP enabled');
      }
      setTotpCode('');
    } catch (err: any) {
      toast.error(err.message || 'Failed to toggle TOTP');
    } finally {
      setTogglingTotp(false);
    }
  };

  if (!user) return null;

  return (
    <AppLayout breadcrumbs={[{ label: 'Profile' }]}>
      <PageHeader
        title="Profile"
        description="Manage your account settings"
        icon={UserCircle}
      />

      <div className="grid gap-6 md:grid-cols-2">
        {/* User Info */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Account Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Name" value={user.name} />
            <InfoRow label="Email" value={user.email} />
            <InfoRow label="Role" value={user.role} />
            {user.tenantId && <InfoRow label="Tenant ID" value={user.tenantId} mono />}
            <InfoRow label="Status" value={user.status} />
            <InfoRow label="TOTP" value={user.totpEnabled ? 'Enabled' : 'Disabled'} />
            <InfoRow label="Member since" value={new Date(user.createdAt).toLocaleDateString()} />
            {user.lastLogin && <InfoRow label="Last login" value={new Date(user.lastLogin).toLocaleString()} />}
          </CardContent>
        </Card>

        {/* Change Password */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Change Password</CardTitle>
            <CardDescription>Update your account password</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="currentPassword">Current Password</Label>
              <Input
                id="currentPassword"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="newPassword">New Password</Label>
              <Input
                id="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirmPassword">Confirm New Password</Label>
              <Input
                id="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
            <Button onClick={handleChangePassword} disabled={changingPassword || !currentPassword || !newPassword}>
              {changingPassword && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Change Password
            </Button>
          </CardContent>
        </Card>

        {/* TOTP */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Shield className="h-4 w-4" />
              Two-Factor Authentication
            </CardTitle>
            <CardDescription>
              {user.totpEnabled
                ? 'TOTP is currently enabled. Enter a code to disable it.'
                : 'Enable TOTP for additional security. Set up your authenticator app first.'}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="totpCode">TOTP Code</Label>
              <Input
                id="totpCode"
                placeholder="123456"
                value={totpCode}
                onChange={(e) => setTotpCode(e.target.value)}
                maxLength={6}
              />
            </div>
            <Button
              variant={user.totpEnabled ? 'destructive' : 'default'}
              onClick={handleToggleTotp}
              disabled={togglingTotp || totpCode.length !== 6}
            >
              {togglingTotp && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {user.totpEnabled ? 'Disable TOTP' : 'Enable TOTP'}
            </Button>
          </CardContent>
        </Card>
      </div>
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
