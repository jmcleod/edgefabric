import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Search, ChevronLeft, ChevronRight } from 'lucide-react';
import { useState, useMemo } from 'react';

export interface Column<T> {
  key: string;
  header: string;
  render?: (item: T) => React.ReactNode;
  sortable?: boolean;
  className?: string;
}

interface ServerSideProps {
  /** Total number of items across all pages (from the API response). */
  total: number;
  /** Current zero-based page index. */
  page: number;
  /** Called when the user navigates to a different page. */
  onPageChange: (page: number) => void;
}

interface DataTableProps<T> {
  data: T[];
  columns: Column<T>[];
  searchKeys?: (keyof T)[];
  pageSize?: number;
  onRowClick?: (item: T) => void;
  emptyMessage?: string;
  actions?: React.ReactNode;
  /** When provided, pagination is server-side. The component renders `data` as-is. */
  serverSide?: ServerSideProps;
}

export function DataTable<T extends object>({
  data,
  columns,
  searchKeys = [],
  pageSize = 10,
  onRowClick,
  emptyMessage = 'No data found',
  actions,
  serverSide,
}: DataTableProps<T>) {
  // Client-side search & pagination (used only when serverSide is not set)
  const [search, setSearch] = useState('');
  const [clientPage, setClientPage] = useState(0);

  const filteredData = useMemo(() => {
    if (serverSide) return data;
    if (!search || searchKeys.length === 0) return data;
    const query = search.toLowerCase();
    return data.filter((item) =>
      searchKeys.some((key) => {
        const value = item[key];
        return typeof value === 'string' && value.toLowerCase().includes(query);
      })
    );
  }, [data, search, searchKeys, serverSide]);

  const displayData = useMemo(() => {
    if (serverSide) return data; // already paginated by the server
    const start = clientPage * pageSize;
    return filteredData.slice(start, start + pageSize);
  }, [filteredData, clientPage, pageSize, serverSide, data]);

  // Pagination state
  const page = serverSide ? serverSide.page : clientPage;
  const totalItems = serverSide ? serverSide.total : filteredData.length;
  const totalPages = Math.ceil(totalItems / pageSize);

  const handlePageChange = (newPage: number) => {
    if (serverSide) {
      serverSide.onPageChange(newPage);
    } else {
      setClientPage(newPage);
    }
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        {!serverSide && searchKeys.length > 0 ? (
          <div className="relative flex-1 max-w-sm">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search..."
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setClientPage(0);
              }}
              className="pl-9 bg-muted border-0 placeholder:text-muted-foreground/60"
            />
          </div>
        ) : (
          <div />
        )}
        {actions && (
          <div className="flex items-center gap-2">
            {actions}
          </div>
        )}
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-muted/30">
              {columns.map((col) => (
                <TableHead key={col.key} className={col.className}>
                  {col.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {displayData.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center text-muted-foreground">
                  {emptyMessage}
                </TableCell>
              </TableRow>
            ) : (
              displayData.map((item, i) => (
                <TableRow
                  key={i}
                  className={onRowClick ? 'cursor-pointer data-row' : 'data-row'}
                  onClick={() => onRowClick?.(item)}
                >
                  {columns.map((col) => (
                    <TableCell key={col.key} className={col.className}>
                      {col.render ? col.render(item) : (item[col.key] as React.ReactNode)}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {page * pageSize + 1} to {Math.min((page + 1) * pageSize, totalItems)} of {totalItems} results
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              disabled={page === 0}
              onClick={() => handlePageChange(page - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {page + 1} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="icon"
              disabled={page >= totalPages - 1}
              onClick={() => handlePageChange(page + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
