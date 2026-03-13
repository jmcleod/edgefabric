import { SidebarTrigger } from '@/components/ui/sidebar';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Search, ChevronDown, LogOut, Settings, User } from 'lucide-react';
import { useAuth } from '@/hooks/useAuth';
import { useNavigate } from 'react-router-dom';
import { Badge } from '@/components/ui/badge';
import { ThemeToggle } from '@/components/ThemeToggle';
import { openCommandPalette } from '@/components/CommandPalette';

interface AppHeaderProps {
  title?: string;
  breadcrumbs?: { label: string; href?: string }[];
}

export function AppHeader({ title, breadcrumbs }: AppHeaderProps) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  return (
    <header className="sticky top-0 z-50 flex h-14 items-center gap-4 border-b border-border bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <SidebarTrigger className="shrink-0" />

      {/* Breadcrumbs / Title */}
      <div className="flex items-center gap-2">
        {breadcrumbs ? (
          <nav className="flex items-center gap-1 text-sm">
            {breadcrumbs.map((crumb, i) => (
              <span key={i} className="flex items-center gap-1">
                {i > 0 && <span className="text-muted-foreground">/</span>}
                <span className={i === breadcrumbs.length - 1 ? 'text-foreground font-medium' : 'text-muted-foreground'}>
                  {crumb.label}
                </span>
              </span>
            ))}
          </nav>
        ) : title ? (
          <h1 className="text-lg font-semibold text-foreground">{title}</h1>
        ) : null}
      </div>

      <div className="flex-1" />

      {/* Search — opens command palette */}
      <button
        onClick={openCommandPalette}
        className="hidden lg:flex items-center gap-2 w-64 h-9 rounded-md bg-muted px-3 text-sm text-muted-foreground/60 hover:bg-muted/80 transition-colors"
      >
        <Search className="h-4 w-4 shrink-0" />
        <span className="flex-1 text-left">Search...</span>
        <kbd className="pointer-events-none hidden h-5 select-none items-center gap-1 rounded border border-border bg-background px-1.5 font-mono text-[10px] font-medium text-muted-foreground sm:flex">
          <span className="text-xs">&#8984;</span>K
        </kbd>
      </button>

      {/* Theme Toggle */}
      <ThemeToggle />

      {/* User Menu */}
      {user && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="gap-2 px-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary text-xs font-medium text-primary-foreground">
                {user.name.split(' ').map((n) => n[0]).join('')}
              </div>
              <span className="hidden text-sm font-medium md:inline-block">{user.name}</span>
              <Badge variant="secondary" className="hidden text-xs md:inline-flex">
                {user.role}
              </Badge>
              <ChevronDown className="h-4 w-4 text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel>My Account</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => navigate('/profile')}>
              <User className="mr-2 h-4 w-4" />
              Profile
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => navigate('/settings')}>
              <Settings className="mr-2 h-4 w-4" />
              Settings
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={logout} className="text-destructive focus:text-destructive">
              <LogOut className="mr-2 h-4 w-4" />
              Log out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </header>
  );
}
