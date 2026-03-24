// 配置服务 - 调用后端API
import { EventsOff, EventsOn } from '@wailsjs/runtime/runtime';
import { GetConfig, UpdateConfig, GetAvailableTools, TestAIConnection, GetScreeningSyncStatus, RunScreeningSync, CancelScreeningSync, GetScreeningUniverseSymbols } from '@wailsjs/go/main/App';
import type { models } from '@wailsjs/go/models';
import type { ScreeningSyncProgress, ScreeningSyncRunOptions, ScreeningSyncStatus } from '../types';

export type AppConfig = models.AppConfig;

// 内置工具信息
export interface ToolInfo {
  name: string;
  description: string;
}

export const getConfig = async (): Promise<AppConfig> => {
  return await GetConfig();
};

export const updateConfig = async (config: AppConfig): Promise<string> => {
  return await UpdateConfig(config);
};

// 获取可用的内置工具列表
export const getAvailableTools = async (): Promise<ToolInfo[]> => {
  return await GetAvailableTools();
};

// 测试 AI 配置连通性
export const testAIConnection = async (config: models.AIConfig): Promise<string> => {
  return await TestAIConnection(config);
};

export const getScreeningSyncStatus = async (): Promise<ScreeningSyncStatus> => {
  return normalizeScreeningSyncStatus(await GetScreeningSyncStatus());
};

export const runScreeningSync = async (options: ScreeningSyncRunOptions): Promise<ScreeningSyncStatus> => {
  return normalizeScreeningSyncStatus(await RunScreeningSync(options as any));
};

export const cancelScreeningSync = async (): Promise<boolean> => {
  return await CancelScreeningSync();
};

export const getScreeningUniverseSymbols = async (limit: number): Promise<string[]> => {
  const raw = await GetScreeningUniverseSymbols(limit);
  return Array.isArray(raw) ? raw.filter((item): item is string => typeof item === 'string' && item.trim().length > 0) : [];
};

export const onScreeningSyncProgress = (callback: (progress: ScreeningSyncProgress) => void): (() => void) => {
  EventsOn('screening:sync:progress', (raw: any) => callback(normalizeScreeningSyncProgress(raw)));
  return () => EventsOff('screening:sync:progress');
};

const normalizeScreeningSyncProgress = (raw: any): ScreeningSyncProgress => ({
  marketScope: raw?.marketScope ?? raw?.MarketScope ?? '',
  mode: raw?.mode ?? raw?.Mode ?? '',
  runStatus: raw?.runStatus ?? raw?.RunStatus ?? '',
  progressPercent: raw?.progressPercent ?? raw?.ProgressPercent ?? 0,
  totalStocks: raw?.totalStocks ?? raw?.TotalStocks ?? 0,
  completedStocks: raw?.completedStocks ?? raw?.CompletedStocks ?? 0,
  currentSymbol: raw?.currentSymbol ?? raw?.CurrentSymbol ?? '',
  currentName: raw?.currentName ?? raw?.CurrentName ?? '',
  currentStage: raw?.currentStage ?? raw?.CurrentStage ?? '',
  activeSource: raw?.activeSource ?? raw?.ActiveSource ?? '',
  lastMessage: raw?.lastMessage ?? raw?.LastMessage ?? '',
  limitStocks: raw?.limitStocks ?? raw?.LimitStocks ?? 0,
  resumeFromCheckpoint: raw?.resumeFromCheckpoint ?? raw?.ResumeFromCheckpoint ?? false,
  events: Array.isArray(raw?.events ?? raw?.Events) ? (raw.events ?? raw.Events).map(normalizeScreeningSyncEvent) : [],
  error: raw?.error ?? raw?.Error,
});

const normalizeScreeningSyncStatus = (raw: any): ScreeningSyncStatus => ({
  marketScope: raw?.marketScope ?? raw?.MarketScope ?? '',
  initialSyncDays: raw?.initialSyncDays ?? raw?.InitialSyncDays ?? 0,
  retentionMode: raw?.retentionMode ?? raw?.RetentionMode ?? '',
  retentionDays: raw?.retentionDays ?? raw?.RetentionDays ?? 0,
  lastTradeDate: raw?.lastTradeDate ?? raw?.LastTradeDate ?? '',
  lastSyncedAt: raw?.lastSyncedAt ?? raw?.LastSyncedAt ?? '',
  targetTradeDate: raw?.targetTradeDate ?? raw?.TargetTradeDate ?? '',
  latestSyncedTradeDate: raw?.latestSyncedTradeDate ?? raw?.LatestSyncedTradeDate ?? '',
  stocksSynced: raw?.stocksSynced ?? raw?.StocksSynced ?? 0,
  barsSynced: raw?.barsSynced ?? raw?.BarsSynced ?? 0,
  snapshotsSynced: raw?.snapshotsSynced ?? raw?.SnapshotsSynced ?? 0,
  storedStocks: raw?.storedStocks ?? raw?.StoredStocks,
  storedBars: raw?.storedBars ?? raw?.StoredBars,
  storedSnapshots: raw?.storedSnapshots ?? raw?.StoredSnapshots,
  marketStockCount: raw?.marketStockCount ?? raw?.MarketStockCount,
  syncedToLatestStocks: raw?.syncedToLatestStocks ?? raw?.SyncedToLatestStocks,
  pendingSyncStocks: raw?.pendingSyncStocks ?? raw?.PendingSyncStocks,
  runStatus: raw?.runStatus ?? raw?.RunStatus,
  progressPercent: raw?.progressPercent ?? raw?.ProgressPercent,
  totalStocks: raw?.totalStocks ?? raw?.TotalStocks,
  completedStocks: raw?.completedStocks ?? raw?.CompletedStocks,
  currentSymbol: raw?.currentSymbol ?? raw?.CurrentSymbol,
  currentName: raw?.currentName ?? raw?.CurrentName,
  currentStage: raw?.currentStage ?? raw?.CurrentStage,
  activeSource: raw?.activeSource ?? raw?.ActiveSource,
  lastMessage: raw?.lastMessage ?? raw?.LastMessage,
  limitStocks: raw?.limitStocks ?? raw?.LimitStocks,
  resumeFromCheckpoint: raw?.resumeFromCheckpoint ?? raw?.ResumeFromCheckpoint,
  syncedSymbols: Array.isArray(raw?.syncedSymbols ?? raw?.SyncedSymbols) ? (raw.syncedSymbols ?? raw.SyncedSymbols) : [],
  events: Array.isArray(raw?.events ?? raw?.Events) ? (raw.events ?? raw.Events).map(normalizeScreeningSyncEvent) : [],
  error: raw?.error ?? raw?.Error,
});

const normalizeScreeningSyncEvent = (raw: any) => ({
  time: raw?.time ?? raw?.Time ?? '',
  symbol: raw?.symbol ?? raw?.Symbol ?? '',
  name: raw?.name ?? raw?.Name ?? '',
  source: raw?.source ?? raw?.Source ?? '',
  status: raw?.status ?? raw?.Status ?? '',
  message: raw?.message ?? raw?.Message ?? '',
});
