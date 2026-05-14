import { CircleAlert, RefreshCw } from 'lucide-react';
import toast from 'react-hot-toast';
import {
  useForceVersionCheck,
  useVersionCheck,
  type VersionCheckTarget,
} from '@/hooks/use-version-check';
import type { VersionCheckResult } from '@/types';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { CopyButton } from './CopyButton';
import { manualVersionCheckToast } from './version-update-toast';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';

interface VersionUpdateIndicatorProps {
  target: VersionCheckTarget;
  label?: string;
}

function displayVersion(version?: string) {
  return version || '-';
}

function targetInstruction(kind: 'server' | 'client') {
  if (kind === 'client') {
    return '在该 Client 所在机器执行以下命令升级，过程中会重启 NetsGo 服务。不要在 Server 机器上执行。';
  }
  return '在 Server 所在机器执行以下命令升级，过程中会重启 NetsGo 服务。';
}

export function VersionUpdateContent({
  data,
  target,
}: {
  data: VersionCheckResult;
  target: VersionCheckTarget;
}) {
  const releaseHref = data.release_url || 'https://github.com/zsio/netsgo/releases';
  const isDocker = data.install_method === 'docker';
  const isService = data.install_method === 'service';

  return (
    <>
      <div className="grid gap-2 rounded-md border bg-muted/30 p-3 text-sm">
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">当前版本</span>
          <span className="font-mono text-foreground">{displayVersion(data.current_version || target.version)}</span>
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">最新版本</span>
          <span className="font-mono text-foreground">{data.latest_version}</span>
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">推荐通道</span>
          <span className="text-foreground">{data.recommended_channel || '-'}</span>
        </div>
      </div>
      {isService && data.commands ? (
        <div className="grid gap-3 text-sm">
          <p className="text-muted-foreground">{targetInstruction(target.kind)}</p>
          {[
            ['国内源', data.commands.domestic],
            ['国外源', data.commands.global],
          ].map(([name, command]) => (
            <div key={name} className="grid gap-1.5">
              <div className="text-xs text-muted-foreground">{name}</div>
              <div className="flex items-start gap-2 rounded-md bg-muted p-2">
                <code className="min-w-0 flex-1 break-all text-xs text-foreground">{command}</code>
                <CopyButton
                  value={command}
                  title={`复制${name}升级命令`}
                  className="inline-flex size-6 items-center justify-center rounded-[min(var(--radius-md),10px)] transition-colors hover:bg-background/70"
                />
              </div>
            </div>
          ))}
        </div>
      ) : isDocker ? (
        <p className="text-sm text-muted-foreground">当前目标以容器方式运行，请使用镜像发布页或部署文档手动更新。</p>
      ) : (
        <p className="text-sm text-muted-foreground">当前目标以二进制方式运行，请前往 GitHub Releases 手动下载更新。</p>
      )}
      <DialogFooter>
        <Button type="button" variant="outline" asChild>
          <a href={releaseHref} target="_blank" rel="noreferrer">
            GitHub Releases
          </a>
        </Button>
      </DialogFooter>
    </>
  );
}

export function VersionUpdateIndicator({ target, label = '运行版本' }: VersionUpdateIndicatorProps) {
  const check = useVersionCheck(target);
  const forceCheck = useForceVersionCheck(target);
  const data = forceCheck.data || check.data;
  const hasUpdate = Boolean(data?.update_available);
  const manualFailed = Boolean(forceCheck.data?.check_failed || forceCheck.error);

  if (!target.version || target.enabled === false) return null;

  const handleManualCheck = () => {
    forceCheck.mutate(undefined, {
      onSuccess: (result) => {
        const toastKind = manualVersionCheckToast(result);
        if (toastKind === 'error') {
          toast.error('检查更新失败，请前往 GitHub Releases 手动确认');
          return;
        }
        if (toastKind === 'success') toast.success('已是最新版本');
      },
      onError: () => {
        if (manualVersionCheckToast(null, true) === 'error') toast.error('检查更新失败，请前往 GitHub Releases 手动确认');
      },
    });
  };

  if (!hasUpdate) {
    return (
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        title={manualFailed ? '检查失败' : '检查更新'}
        disabled={forceCheck.isPending}
        onClick={handleManualCheck}
        className={cn(
          'size-4 opacity-0 transition-opacity hover:opacity-100 focus-visible:opacity-100 group-hover/version-update:opacity-100',
          forceCheck.isPending && 'opacity-100',
        )}
      >
        <RefreshCw className={cn('size-3', forceCheck.isPending && 'animate-spin')} />
      </Button>
    );
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        <button
          type="button"
          className="inline-flex size-4 shrink-0 items-center justify-center text-amber-500 transition-colors hover:text-amber-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
          aria-label={`${label}可更新到 ${data?.latest_version}`}
        >
          <CircleAlert className="size-3.5" />
        </button>
      </DialogTrigger>
      <DialogContent>
        {data && (
          <>
            <DialogHeader>
              <DialogTitle>发现可用更新</DialogTitle>
              <DialogDescription>
                可更新：{displayVersion(data.current_version || target.version)} → {data.latest_version}
              </DialogDescription>
            </DialogHeader>
            <VersionUpdateContent data={data} target={target} />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
