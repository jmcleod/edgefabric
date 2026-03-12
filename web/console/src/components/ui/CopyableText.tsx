import { useState, useCallback } from 'react';
import { Check, Copy } from 'lucide-react';
import { cn } from '@/lib/utils';

interface CopyableTextProps {
  value: string;
  className?: string;
  mono?: boolean;
}

export function CopyableText({ value, className, mono = true }: CopyableTextProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Fallback: select the text
    }
  }, [value]);

  return (
    <span
      role="button"
      tabIndex={0}
      onClick={handleCopy}
      onKeyDown={(e) => e.key === 'Enter' && handleCopy()}
      className={cn(
        'inline-flex items-center gap-1.5 group cursor-pointer rounded px-1 -mx-1 hover:bg-muted transition-colors',
        mono && 'mono-data',
        className,
      )}
      title="Click to copy"
    >
      <span className="truncate">{value}</span>
      {copied ? (
        <Check className="h-3 w-3 text-green-500 shrink-0" />
      ) : (
        <Copy className="h-3 w-3 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
      )}
    </span>
  );
}
