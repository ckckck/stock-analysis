import { describe, expect, it } from 'vitest';
import {
  createPendingScreeningSyncStatus,
  resolveScreeningSyncCoverageStats,
  resolveScreeningPrimaryActionLabel,
  resolveSyncDialogCopy,
  resolveTopbarSyncButtonState,
  shouldContinueAfterScreeningSync,
} from './screeningSync';

describe('resolveTopbarSyncButtonState', () => {
  it('returns completed state when all stocks are synced', () => {
    expect(resolveTopbarSyncButtonState({
      syncedToLatestStocks: 5003,
      marketStockCount: 5003,
    })).toEqual({
      tone: 'success',
      label: '已同步',
      detail: '(5003/5003)',
      disabled: true,
      loading: false,
    });
  });

  it('returns partial state when part of the market is synced', () => {
    expect(resolveTopbarSyncButtonState({
      syncedToLatestStocks: 1,
      marketStockCount: 5003,
    })).toEqual({
      tone: 'warning',
      label: '立即同步',
      detail: '(1/5003)',
      disabled: false,
      loading: false,
    });
  });

  it('uses the current completed stock count while sync is running', () => {
    expect(resolveTopbarSyncButtonState({
      syncedToLatestStocks: 1,
      completedStocks: 5,
      marketStockCount: 5003,
      runStatus: 'running',
    })).toEqual({
      tone: 'warning',
      label: '立即同步',
      detail: '(5/5003)',
      disabled: true,
      loading: true,
    });
  });

  it('returns empty state when nothing has been synced yet', () => {
    expect(resolveTopbarSyncButtonState({
      syncedToLatestStocks: 0,
      marketStockCount: 5003,
    })).toEqual({
      tone: 'danger',
      label: '立即同步',
      detail: '(0/5003)',
      disabled: false,
      loading: false,
    });
  });

  it('falls back to unknown total when market size is not available', () => {
    expect(resolveTopbarSyncButtonState({
      syncedToLatestStocks: 0,
      marketStockCount: 0,
    }).detail).toBe('(--/--)');
  });
});

describe('resolveSyncDialogCopy', () => {
  it('returns sync-only copy', () => {
    expect(resolveSyncDialogCopy('sync-only')).toEqual({
      title: '确认后开始同步',
      description: '本次会按当前设置同步本地数据库，不会触发 AI 筛选。',
      confirmLabel: '开始同步',
    });
  });

  it('returns screening copy', () => {
    expect(resolveSyncDialogCopy('screening')).toEqual({
      title: '确认同步后开始筛选',
      description: '本次会先按当前设置同步本地数据库，再基于最新数据执行 AI 筛选。',
      confirmLabel: '开始同步并筛选',
    });
  });
});

describe('createPendingScreeningSyncStatus', () => {
  it('preserves checkpoint progress when resuming a partial sync', () => {
    expect(createPendingScreeningSyncStatus({
      marketScope: '沪市、深市',
      initialSyncDays: 30,
      retentionMode: 'forever',
      retentionDays: 60,
      lastTradeDate: '2026-03-24',
      lastSyncedAt: '2026-03-24 10:00:00',
      targetTradeDate: '2026-03-24',
      latestSyncedTradeDate: '2026-03-24',
      stocksSynced: 5,
      barsSynced: 100,
      snapshotsSynced: 5,
      marketStockCount: 5151,
      syncedToLatestStocks: 5,
      pendingSyncStocks: 5146,
      completedStocks: 5,
      totalStocks: 5151,
      currentStage: 'canceled',
      runStatus: 'canceled',
    }, {
      initialSyncDays: 30,
      retentionMode: 'forever',
      retentionDays: 60,
      limitStocks: 0,
      message: '准备启动同步任务...',
    })).toMatchObject({
      runStatus: 'running',
      completedStocks: 5,
      totalStocks: 5151,
      syncedToLatestStocks: 5,
    });
  });
});

describe('resolveScreeningSyncCoverageStats', () => {
  it('prefers market-wide coverage metrics over current task progress', () => {
    expect(resolveScreeningSyncCoverageStats({
      syncedToLatestStocks: 65,
      marketStockCount: 5151,
      pendingSyncStocks: 5086,
      completedStocks: 31,
      totalStocks: 5101,
    })).toEqual({
      syncedToLatestStocks: 65,
      pendingSyncStocks: 5086,
      marketStockCount: 5151,
      syncedProgressLabel: '65 / 5151',
      pendingSyncLabel: '5086',
      marketStockCountLabel: '5151',
    });
  });

  it('falls back to current task progress when market coverage is unavailable', () => {
    expect(resolveScreeningSyncCoverageStats({
      completedStocks: 12,
      totalStocks: 100,
    })).toEqual({
      syncedToLatestStocks: 12,
      pendingSyncStocks: 88,
      marketStockCount: 100,
      syncedProgressLabel: '12 / 100',
      pendingSyncLabel: '88',
      marketStockCountLabel: '100',
    });
  });
});

describe('resolveScreeningPrimaryActionLabel', () => {
  it('returns 开始筛选 for a normal run', () => {
    expect(resolveScreeningPrimaryActionLabel({
      loading: false,
      canReuseHistorySql: false,
    })).toBe('开始筛选');
  });

  it('returns loading copy while screening is running', () => {
    expect(resolveScreeningPrimaryActionLabel({
      loading: true,
      canReuseHistorySql: false,
    })).toBe('筛选中...');
  });

  it('returns historical rerun copy when the current result still reuses saved sql', () => {
    expect(resolveScreeningPrimaryActionLabel({
      loading: false,
      canReuseHistorySql: true,
    })).toBe('根据历史筛选方式重新筛选');
  });
});

describe('shouldContinueAfterScreeningSync', () => {
  it('returns false when sync is canceled', () => {
    expect(shouldContinueAfterScreeningSync({
      runStatus: 'canceled',
    })).toBe(false);
  });

  it('returns false when sync failed or returned an error', () => {
    expect(shouldContinueAfterScreeningSync({
      runStatus: 'failed',
      error: 'database is locked',
    })).toBe(false);
  });

  it('returns true when sync completed cleanly', () => {
    expect(shouldContinueAfterScreeningSync({
      runStatus: 'completed',
    })).toBe(true);
  });
});
