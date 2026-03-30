// 市场数据服务 - 调用后端API
import {
  GetStockRealTimeDataWithRequest,
  GetKLineDataWithRequest,
  GetOrderBookWithRequest,
  SearchStocks,
} from '@wailsjs/go/main/App';
import type { Stock, KLineData, OrderBook } from '../types';
import { debugAppEvent, errorAppEvent } from '../utils/appLog';

// 股票搜索结果类型
export interface StockSearchResult {
  symbol: string;
  name: string;
  industry: string;
  market: string;
}

export interface RequestOptions {
  requestId?: string;
}

const nextRequestId = (prefix: string): string => `${prefix}-${Date.now()}`;

const toErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
};

export const getStockRealTimeData = async (codes: string[], options: RequestOptions = {}): Promise<Stock[]> => {
  debugAppEvent('market-realtime', 'frontend bridge request', {
    module: 'market',
    action: 'realtime.request',
    codes: codes.length,
    symbols: codes,
    requestId: options.requestId,
  });
  try {
    const data = await GetStockRealTimeDataWithRequest({
      codes,
      requestId: options.requestId ?? '',
    });
    debugAppEvent('market-realtime', 'frontend bridge success', {
      module: 'market',
      action: 'realtime.success',
      codes: codes.length,
      resultLen: Array.isArray(data) ? data.length : 0,
      requestId: options.requestId,
    });
    return data;
  } catch (error) {
    errorAppEvent('market-realtime', 'frontend bridge failed', {
      module: 'market',
      action: 'realtime.failed',
      codes: codes.length,
      symbols: codes,
      requestId: options.requestId,
      err: toErrorMessage(error),
    });
    throw error;
  }
};

export const getKLineData = async (code: string, period: string, days: number, options: RequestOptions = {}): Promise<KLineData[]> => {
  return await GetKLineDataWithRequest({
    code,
    period,
    days,
    requestId: options.requestId ?? '',
  });
};

// 获取真实五档盘口数据
export const getOrderBook = async (code: string, options: RequestOptions = {}): Promise<OrderBook> => {
  const requestId = options.requestId?.trim() || nextRequestId('orderbook');
  debugAppEvent('orderbook', 'frontend bridge request', {
    module: 'market',
    action: 'orderbook.request',
    symbol: code,
    requestId,
  });
  try {
    const data = await GetOrderBookWithRequest({
      code,
      requestId,
    });
    debugAppEvent('orderbook', 'frontend bridge success', {
      module: 'market',
      action: 'orderbook.success',
      symbol: code,
      requestId,
      bids: data?.bids?.length ?? 0,
      asks: data?.asks?.length ?? 0,
    });
    return data;
  } catch (error) {
    errorAppEvent('orderbook', 'frontend bridge failed', {
      module: 'market',
      action: 'orderbook.failed',
      symbol: code,
      requestId,
      err: toErrorMessage(error),
    });
    throw error;
  }
};

// 搜索股票
export const searchStocks = async (keyword: string): Promise<StockSearchResult[]> => {
  if (!keyword.trim()) return [];
  return await SearchStocks(keyword) as StockSearchResult[];
};
