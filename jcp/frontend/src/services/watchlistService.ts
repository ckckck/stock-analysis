// 自选股服务 - 调用后端API
import { GetWatchlist, AddToWatchlist, RemoveFromWatchlist } from '@wailsjs/go/main/App';
import type { Stock } from '../types';

export const getWatchlist = async (): Promise<Stock[]> => {
  return await GetWatchlist() as Stock[];
};

export const addToWatchlist = async (stock: Stock): Promise<string> => {
  return await AddToWatchlist(stock as any);
};

export const removeFromWatchlist = async (symbol: string): Promise<string> => {
  return await RemoveFromWatchlist(symbol);
};
