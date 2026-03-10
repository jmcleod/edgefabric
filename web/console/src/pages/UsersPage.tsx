import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useUsers, useCreateUser, useUpdateUser, useDeleteUser } from '@/hooks/useUsers';
import { userSchema, type UserFormData } from '@/lib/schemas';
import type { User } from '@/types';
import { Users, MoreHorizontal, Pencil, Trash2 } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const userFields: FieldConfig<UserFormData>[] = [
  { name: 'name', label: 'Full Name', placeholder: 'Jane Doe' },
  { name: 'email', label: 'Email', type: 'email', placeholder: 'jane@example.com' },
  { name: 'password', label: 'Password', type: 'password', placeholder: 'Min 8 characters', description: 'Leave blank when editing to keep current password' },
  { name: 'role', label: 'Role', type: 'select', options: [
    { label: 'Superuser', value: 'superuser' },
    { label: 'Admin', value: 'admin' },
    { label: 'Read Only', value: 'readonly' },
  ] },
  { name: 'tenant_id', label: 'Tenant ID', placeholder: 'Optional — leave blank for platform users', description: 'Assign this user to a specific tenant' },
];

export default function UsersPage() {
  const { data, isLoading } = useUsers();
  const users = data?.items || [];
  const createUser = useCreateUser();
  const updateUser = useUpdateUser();
  const deleteUser = useDeleteUser();

  const [createOpen, setCreateOpen] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);

  const columns: Column<User>[] = [
    {
      key: 'name',
      header: 'User',
      render: (user) => (
        <div>
          <p className="font-medium text-foreground">{user.name}</p>
          <p className="text-xs text-muted-foreground">{user.email}</p>
        </div>
      ),
    },
    {
      key: 'role',
      header: 'Role',
      render: (user) => (
        <Badge variant={user.role === 'superuser' ? 'default' : 'secondary'}>
          {user.role}
        </Badge>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (user) => (
        <StatusBadge status={user.status === 'active' ? 'healthy' : 'critical'} size="sm" />
      ),
    },
    {
      key: 'lastLogin',
      header: 'Last Login',
      render: (user) => (
        <span className="text-sm text-muted-foreground">
          {user.lastLogin ? new Date(user.lastLogin).toLocaleDateString() : '\u2014'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (user) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setEditUser(user)}>
              <Pencil className="mr-2 h-4 w-4" /> Edit User
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(user)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete User
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Administration' }, { label: 'Users & Access' }]}>
      <PageHeader
        title="Users & Access"
        description={`${data?.total ?? 0} users`}
        icon={Users}
        action={{
          label: 'Add User',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={users}
          columns={columns}
          searchKeys={['name', 'email']}
          pageSize={10}
        />
      )}

      {/* Create Dialog */}
      <FormDialog<UserFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create User"
        description="Add a new user to the platform."
        schema={userSchema}
        defaultValues={{ email: '', name: '', role: 'admin', tenant_id: '', password: '' }}
        fields={userFields}
        onSubmit={async (data) => {
          await createUser.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createUser.isPending}
        submitLabel="Create User"
      />

      {/* Edit Dialog */}
      <FormDialog<UserFormData>
        open={!!editUser}
        onOpenChange={(open) => !open && setEditUser(null)}
        title="Edit User"
        schema={userSchema}
        defaultValues={editUser ? { email: editUser.email, name: editUser.name, role: editUser.role, tenant_id: editUser.tenantId || '', password: '' } : undefined}
        fields={userFields}
        onSubmit={async (data) => {
          if (editUser) {
            const body = { ...data };
            if (!body.password) delete body.password;
            await updateUser.mutateAsync({ id: editUser.id, body });
            setEditUser(null);
          }
        }}
        isSubmitting={updateUser.isPending}
        submitLabel="Save Changes"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteUser.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteUser.isPending}
      />
    </AppLayout>
  );
}
