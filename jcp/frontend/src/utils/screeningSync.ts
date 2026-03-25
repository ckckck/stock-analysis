import type { ScreeningResultMode, ScreeningResultPreset, ScreeningSyncProgress, ScreeningSyncStatus } from '../types';

export type SyncButtonTone = 'success' | 'warning' | 'danger';
export type ScreeningSyncDialogMode = 'screening' | 'sync-only';

export interface TopbarSyncButtonState {
  tone: SyncButtonTone;
  label: string;
  detail: string;
  disabled: boolean;
  loading: boolean;
}

export interface ScreeningSyncDialogCopy {
  title: string;
  description: string;
  confirmLabel: string;
}

export interface ScreeningPrimaryActionLabelOptions {
  loading: boolean;
  showHistoryRerunLabel: boolean;
}

export interface ScreeningSyncCoverageStats {
  syncedToLatestStocks: number;
  pendingSyncStocks: number;
  marketStockCount: number;
  syncedProgressLabel: string;
  pendingSyncLabel: string;
  marketStockCountLabel: string;
}

export const mergeScreeningSyncProgress = (
  current: ScreeningSyncStatus | null,
  progress: ScreeningSyncProgress,
): ScreeningSyncStatus => ({
  marketScope: progress.marketScope || current?.marketScope || '',
  initialSyncDays: current?.initialSyncDays || 0,
  retentionMode: current?.retentionMode || '',
  retentionDays: current?.retentionDays || 0,
  lastTradeDate: current?.lastTradeDate || '',
  lastSyncedAt: current?.lastSyncedAt || '',
  targetTradeDate: current?.targetTradeDate || '',
  latestSyncedTradeDate: current?.latestSyncedTradeDate || '',
  stocksSynced: current?.stocksSynced || 0,
  barsSynced: current?.barsSynced || 0,
  snapshotsSynced: current?.snapshotsSynced || 0,
  storedStocks: current?.storedStocks,
  storedBars: current?.storedBars,
  storedSnapshots: current?.storedSnapshots,
  marketStockCount: current?.marketStockCount,
  syncedToLatestStocks: current?.syncedToLatestStocks,
  pendingSyncStocks: current?.pendingSyncStocks,
  runStatus: progress.runStatus,
  progressPercent: progress.progressPercent,
  totalStocks: progress.totalStocks,
  completedStocks: progress.completedStocks,
  currentSymbol: progress.currentSymbol,
  currentName: progress.currentName,
  currentStage: progress.currentStage,
  activeSource: progress.activeSource,
  lastMessage: progress.lastMessage,
  limitStocks: progress.limitStocks,
  resumeFromCheckpoint: progress.resumeFromCheckpoint,
  events: progress.events || [],
  error: progress.error || undefined,
});

export const formatScreeningSyncRunStatus = (status?: string): string => {
  switch ((status || '').toLowerCase()) {
    case 'running':
      return '进行中';
    case 'completed':
      return '已完成';
    case 'failed':
      return '失败';
    case 'canceled':
      return '已取消';
    default:
      return '未开始';
  }
};

export const formatScreeningSourceName = (source?: string): string => {
  switch ((source || '').toLowerCase()) {
    case 'baostock':
      return 'Baostock';
    case 'sina':
      return 'Sina';
    default:
      return source || '--';
  }
};

export const createPendingScreeningSyncStatus = (
  current: ScreeningSyncStatus | null,
  options: {
    initialSyncDays: number;
    retentionMode: string;
    retentionDays: number;
    limitStocks: number;
    message: string;
  },
): ScreeningSyncStatus => {
  const sameLimit = (current?.limitStocks ?? 0) === options.limitStocks;
  const hasCheckpointProgress = Boolean(
    current
    && sameLimit
    && ['canceled', 'failed'].includes((current.runStatus || '').toLowerCase())
    && (current.completedStocks ?? 0) > 0
    && (current.totalStocks ?? 0) >= (current.completedStocks ?? 0),
  );
  const resumedCompletedStocks = hasCheckpointProgress ? (current?.completedStocks ?? 0) : 0;
  const totalStocks = hasCheckpointProgress
    ? current?.totalStocks
    : (
      options.limitStocks > 0
        ? (
          (current?.marketStockCount ?? 0) > 0
            ? Math.min(options.limitStocks, current?.marketStockCount ?? 0)
            : options.limitStocks
        )
        : current?.marketStockCount
    );
  const progressPercent = totalStocks && totalStocks > 0
    ? Math.max(0, Math.min(100, (resumedCompletedStocks / totalStocks) * 100))
    : 0;

  return ({
  marketScope: current?.marketScope || '',
  initialSyncDays: current?.initialSyncDays || options.initialSyncDays,
  retentionMode: current?.retentionMode || options.retentionMode,
  retentionDays: current?.retentionDays || options.retentionDays,
  lastTradeDate: current?.lastTradeDate || '',
  lastSyncedAt: current?.lastSyncedAt || '',
  targetTradeDate: current?.targetTradeDate || '',
  latestSyncedTradeDate: current?.latestSyncedTradeDate || '',
  stocksSynced: current?.stocksSynced || 0,
  barsSynced: current?.barsSynced || 0,
  snapshotsSynced: current?.snapshotsSynced || 0,
  storedStocks: current?.storedStocks,
  storedBars: current?.storedBars,
  storedSnapshots: current?.storedSnapshots,
  marketStockCount: current?.marketStockCount,
  syncedToLatestStocks: current?.syncedToLatestStocks,
  pendingSyncStocks: current?.pendingSyncStocks,
  syncedSymbols: current?.syncedSymbols || [],
  runStatus: 'running',
  progressPercent,
  totalStocks,
  completedStocks: resumedCompletedStocks,
  currentSymbol: '',
  currentName: '',
  currentStage: 'prepare',
  activeSource: '',
  lastMessage: options.message,
  limitStocks: options.limitStocks,
  resumeFromCheckpoint: hasCheckpointProgress,
  events: [],
  error: undefined,
  });
};

export const resolveTopbarSyncButtonState = (
  status: Pick<ScreeningSyncStatus, 'syncedToLatestStocks' | 'completedStocks' | 'marketStockCount' | 'runStatus'> | null | undefined,
): TopbarSyncButtonState => {
  const runStatus = (status?.runStatus || '').toLowerCase();
  const synced = Math.max(
    0,
    runStatus && runStatus !== 'completed'
      ? Math.max(status?.syncedToLatestStocks ?? 0, status?.completedStocks ?? 0)
      : (status?.syncedToLatestStocks ?? 0),
  );
  const total = Math.max(0, status?.marketStockCount ?? 0);
  const loading = runStatus === 'running';
  const detail = total > 0 ? `(${Math.min(synced, total)}/${total})` : '(--/--)';

  if (total > 0 && synced >= total) {
    return {
      tone: 'success',
      label: '已同步',
      detail,
      disabled: true,
      loading,
    };
  }

  if (synced > 0) {
    return {
      tone: 'warning',
      label: '立即同步',
      detail,
      disabled: loading,
      loading,
    };
  }

  return {
    tone: 'danger',
    label: '立即同步',
    detail,
    disabled: loading,
    loading,
  };
};

export const resolveScreeningSyncCoverageStats = (
  status: Pick<ScreeningSyncStatus, 'syncedToLatestStocks' | 'pendingSyncStocks' | 'marketStockCount' | 'completedStocks' | 'totalStocks'> | null | undefined,
): ScreeningSyncCoverageStats => {
  const marketStockCount = Math.max(0, status?.marketStockCount ?? status?.totalStocks ?? 0);
  const syncedToLatestStocks = Math.max(
    0,
    Math.min(marketStockCount, status?.syncedToLatestStocks ?? status?.completedStocks ?? 0),
  );
  const pendingSyncStocks = Math.max(
    0,
    status?.pendingSyncStocks ?? Math.max(0, marketStockCount - syncedToLatestStocks),
  );

  return {
    syncedToLatestStocks,
    pendingSyncStocks,
    marketStockCount,
    syncedProgressLabel: marketStockCount > 0 ? `${syncedToLatestStocks} / ${marketStockCount}` : '--',
    pendingSyncLabel: marketStockCount > 0 ? String(pendingSyncStocks) : '--',
    marketStockCountLabel: marketStockCount > 0 ? String(marketStockCount) : '--',
  };
};

export const resolveSyncDialogCopy = (mode: ScreeningSyncDialogMode): ScreeningSyncDialogCopy => {
  if (mode === 'sync-only') {
    return {
      title: '确认后开始同步',
      description: '本次会按当前设置同步本地数据库，不会触发 AI 筛选。',
      confirmLabel: '开始同步',
    };
  }

  return {
    title: '确认同步后开始筛选',
    description: '本次会先按当前设置同步本地数据库，再基于最新数据执行 AI 筛选。',
    confirmLabel: '开始同步并筛选',
  };
};

export const resolveScreeningPrimaryActionLabel = ({
  loading,
  showHistoryRerunLabel,
}: ScreeningPrimaryActionLabelOptions): string => {
  if (loading) {
    return '筛选中...';
  }
  if (showHistoryRerunLabel) {
    return '根据历史筛选方式重新筛选';
  }
  return '开始筛选';
};

export const resolveScreeningPresetFromResult = (input: {
  resultMode: ScreeningResultMode;
  resultLimit: number;
  fallbackPreset: ScreeningResultPreset;
}): ScreeningResultPreset => {
  if (input.resultMode === 'unlimited') {
    return 'unlimited';
  }
  switch (input.resultLimit) {
    case 50:
      return '50';
    case 100:
      return '100';
    case 200:
      return '200';
    default:
      return input.fallbackPreset;
  }
};

export const shouldContinueAfterScreeningSync = (
  status: Pick<ScreeningSyncStatus, 'runStatus' | 'error'> | null | undefined,
): boolean => {
  const runStatus = (status?.runStatus || '').toLowerCase();
  if (status?.error) {
    return false;
  }
  return runStatus !== 'failed' && runStatus !== 'canceled';
};
