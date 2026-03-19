import { GetScreeningHistoryRun, ListScreeningHistory, RunScreeningQuery } from '@wailsjs/go/main/App';
import type {
  ScreeningHistoryItem,
  ScreeningHistoryResponse,
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

const normalizeScreeningQueryResponse = (raw: any): ScreeningQueryResponse => ({
  runId: raw?.runId ?? raw?.RunID ?? 0,
  prompt: raw?.prompt ?? raw?.Prompt,
  marketScope: raw?.marketScope ?? raw?.MarketScope ?? '',
  resultMode: raw?.resultMode ?? raw?.ResultMode ?? '',
  resultLimit: raw?.resultLimit ?? raw?.ResultLimit ?? 0,
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
