import { EventsOff, EventsOn } from '@wailsjs/runtime/runtime';
import { CancelScreeningQuery, GetScreeningHistoryRun, ListScreeningHistory, RerunScreeningHistoryRun, RunScreeningQuery } from '@wailsjs/go/main/App';
import type {
  ScreeningHistoryItem,
  ScreeningHistoryResponse,
  ScreeningQueryProgress,
  ScreeningQueryRequest,
  ScreeningQueryResponse,
} from '../types';

export const runScreeningQuery = async (request: ScreeningQueryRequest): Promise<ScreeningQueryResponse> => {
  const response = normalizeScreeningQueryResponse(await RunScreeningQuery(request));
  if (response.error) {
    throw new Error(response.error);
  }
  return response;
};

export const listScreeningHistory = async (limit = 20): Promise<ScreeningHistoryItem[]> => {
  const response = await ListScreeningHistory(limit) as ScreeningHistoryResponse;
  if (response.error) {
    throw new Error(response.error);
  }
  return response.items ?? [];
};

export const getScreeningHistoryRun = async (runId: number, page = 1, pageSize = 200): Promise<ScreeningQueryResponse> => {
  const response = normalizeScreeningQueryResponse(await GetScreeningHistoryRun(runId, page, pageSize));
  if (response.error) {
    throw new Error(response.error);
  }
  return response;
};

export const deleteScreeningHistoryRun = async (runId: number): Promise<void> => {
  const error = await (window as any).go.main.App.DeleteScreeningHistoryRun(runId);
  if (error) {
    throw new Error(error);
  }
};

export const rerunScreeningHistoryRun = async (runId: number, page = 1, pageSize = 200): Promise<ScreeningQueryResponse> => {
  const response = normalizeScreeningQueryResponse(await RerunScreeningHistoryRun(runId, page, pageSize));
  if (response.error) {
    throw new Error(response.error);
  }
  return response;
};

export const rerunScreeningHistoryRunWithUniverse = async (
  runId: number,
  universeSymbols: string[],
  page = 1,
  pageSize = 200,
): Promise<ScreeningQueryResponse> => {
  const raw = await (window as any).go.main.App.RerunScreeningHistoryRunWithUniverse(runId, page, pageSize, universeSymbols);
  const response = normalizeScreeningQueryResponse(raw);
  if (response.error) {
    throw new Error(response.error);
  }
  return response;
};

export const cancelScreeningQuery = async (): Promise<boolean> => {
  return await CancelScreeningQuery();
};

export const onScreeningQueryProgress = (callback: (progress: ScreeningQueryProgress) => void): (() => void) => {
  EventsOn('screening:query:progress', (raw: any) => {
    callback(normalizeScreeningQueryProgress(raw));
  });
  return () => EventsOff('screening:query:progress');
};

const normalizeScreeningQueryResponse = (raw: any): ScreeningQueryResponse => ({
  runId: raw?.runId ?? raw?.RunID ?? 0,
  prompt: raw?.prompt ?? raw?.Prompt,
  marketScope: raw?.marketScope ?? raw?.MarketScope ?? '',
  resultMode: raw?.resultMode ?? raw?.ResultMode ?? 'top_n',
  resultLimit: raw?.resultLimit ?? raw?.ResultLimit ?? 0,
  universeSymbols: Array.isArray(raw?.universeSymbols ?? raw?.UniverseSymbols)
    ? (raw.universeSymbols ?? raw.UniverseSymbols).filter((item: unknown): item is string => typeof item === 'string')
    : undefined,
  generatedSql: raw?.generatedSql ?? raw?.GeneratedSQL ?? '',
  totalCount: raw?.totalCount ?? raw?.TotalCount ?? 0,
  page: raw?.page ?? raw?.Page ?? 1,
  pageSize: raw?.pageSize ?? raw?.PageSize ?? 50,
  createdAt: raw?.createdAt ?? raw?.CreatedAt,
  error: raw?.error ?? raw?.Error,
  results: Array.isArray(raw?.results ?? raw?.Results)
    ? (raw.results ?? raw.Results).map(normalizeScreeningRunResult)
    : [],
});

const normalizeScreeningRunResult = (raw: any) => ({
  runId: raw?.runId ?? raw?.RunID ?? 0,
  symbol: raw?.symbol ?? raw?.Symbol ?? '',
  name: raw?.name ?? raw?.Name ?? '',
  rank: raw?.rank ?? raw?.Rank ?? 0,
  score: raw?.score ?? raw?.Score ?? 0,
  snapshotTradeDate: raw?.snapshotTradeDate ?? raw?.SnapshotTradeDate ?? '',
  price: raw?.price ?? raw?.Price ?? 0,
  changePercent: raw?.changePercent ?? raw?.ChangePercent ?? 0,
  volume: raw?.volume ?? raw?.Volume ?? 0,
  amount: raw?.amount ?? raw?.Amount ?? 0,
});

const normalizeScreeningQueryProgress = (raw: any): ScreeningQueryProgress => ({
  runStatus: raw?.runStatus ?? raw?.RunStatus ?? '',
  currentStage: raw?.currentStage ?? raw?.CurrentStage ?? '',
  progressPercent: raw?.progressPercent ?? raw?.ProgressPercent ?? 0,
  message: raw?.message ?? raw?.Message ?? '',
  streamingText: raw?.streamingText ?? raw?.StreamingText ?? '',
  prompt: raw?.prompt ?? raw?.Prompt ?? '',
  universeCount: raw?.universeCount ?? raw?.UniverseCount,
  error: raw?.error ?? raw?.Error,
  logs: Array.isArray(raw?.logs ?? raw?.Logs)
    ? (raw.logs ?? raw.Logs).map((entry: any) => ({
        time: entry?.time ?? entry?.Time ?? '',
        stage: entry?.stage ?? entry?.Stage ?? '',
        status: entry?.status ?? entry?.Status ?? '',
        message: entry?.message ?? entry?.Message ?? '',
      }))
    : [],
});
