import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  getStockRealTimeDataWithRequestMock: vi.fn(),
  getKLineDataWithRequestMock: vi.fn(),
  getOrderBookWithRequestMock: vi.fn(),
  searchStocksMock: vi.fn(),
  debugAppEventMock: vi.fn(),
  errorAppEventMock: vi.fn(),
}));

vi.mock('@wailsjs/go/main/App', () => ({
  GetStockRealTimeDataWithRequest: mocks.getStockRealTimeDataWithRequestMock,
  GetKLineDataWithRequest: mocks.getKLineDataWithRequestMock,
  GetOrderBookWithRequest: mocks.getOrderBookWithRequestMock,
  SearchStocks: mocks.searchStocksMock,
}));

vi.mock('../utils/appLog', () => ({
  debugAppEvent: mocks.debugAppEventMock,
  errorAppEvent: mocks.errorAppEventMock,
}));

import { getKLineData, getOrderBook, getStockRealTimeData } from './stockService';

describe('stockService request forwarding', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('forwards requestId for realtime requests', async () => {
    mocks.getStockRealTimeDataWithRequestMock.mockResolvedValueOnce([]);

    await getStockRealTimeData(['sh600000', 'sz000001'], { requestId: 'rt-123' });

    expect(mocks.getStockRealTimeDataWithRequestMock).toHaveBeenCalledWith({
      codes: ['sh600000', 'sz000001'],
      requestId: 'rt-123',
    });
  });

  it('forwards requestId for kline requests', async () => {
    mocks.getKLineDataWithRequestMock.mockResolvedValueOnce([]);

    await getKLineData('sh600000', '1d', 240, { requestId: 'kline-7' });

    expect(mocks.getKLineDataWithRequestMock).toHaveBeenCalledWith({
      code: 'sh600000',
      period: '1d',
      days: 240,
      requestId: 'kline-7',
    });
  });

  it('forwards requestId for orderbook requests', async () => {
    mocks.getOrderBookWithRequestMock.mockResolvedValueOnce({ bids: [], asks: [] });

    await getOrderBook('sz000001', { requestId: 'ob-9' });

    expect(mocks.getOrderBookWithRequestMock).toHaveBeenCalledWith({
      code: 'sz000001',
      requestId: 'ob-9',
    });
  });

  it('generates requestId for orderbook requests when caller does not provide one', async () => {
    mocks.getOrderBookWithRequestMock.mockResolvedValueOnce({ bids: [], asks: [] });

    await getOrderBook('sz000001');

    expect(mocks.getOrderBookWithRequestMock).toHaveBeenCalledWith({
      code: 'sz000001',
      requestId: expect.stringMatching(/^orderbook-\d+$/),
    });
  });
});
