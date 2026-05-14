import { useEffect, useRef, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { cn } from '@/lib/utils';

interface CopyButtonProps {
  value: string;
  title: string;
  ariaLabel?: string;
  className?: string;
  iconClassName?: string;
}

function canCopy(value: string) {
  return value.trim() !== '' && value !== '-';
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
}

export function CopyButton({
  value,
  title,
  ariaLabel = title,
  className,
  iconClassName = 'h-3.5 w-3.5',
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const copyTimerRef = useRef<number | null>(null);
  const copyable = canCopy(value);

  useEffect(() => () => {
    if (copyTimerRef.current !== null) {
      window.clearTimeout(copyTimerRef.current);
    }
  }, []);

  const resetCopyTimer = () => {
    if (copyTimerRef.current !== null) {
      window.clearTimeout(copyTimerRef.current);
    }
    copyTimerRef.current = window.setTimeout(() => {
      setCopied(false);
      copyTimerRef.current = null;
    }, 1200);
  };

  const handleCopy = async () => {
    if (!copyable) return;

    try {
      await copyText(value);
      setCopied(true);
      resetCopyTimer();
    } catch {
      setCopied(false);
    }
  };

  if (!copyable) return null;

  return (
    <button
      type="button"
      className={cn(
        'shrink-0 rounded-sm text-muted-foreground transition-opacity hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
        className,
      )}
      title={title}
      aria-label={ariaLabel}
      onClick={handleCopy}
    >
      {copied ? <Check className={cn(iconClassName, 'text-emerald-500')} /> : <Copy className={iconClassName} />}
    </button>
  );
}
