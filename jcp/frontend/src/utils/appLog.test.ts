import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  logFrontendDebugMock: vi.fn(() => Promise.resolve()),
  logFrontendInfoMock: vi.fn(() => Promise.resolve()),
  logFrontendWarningMock: vi.fn(() => Promise.resolve()),
  logFrontendErrorMock: vi.fn(() => Promise.resolve()),
  logDebugMock: vi.fn(),
  logInfoMock: vi.fn(),
  logWarningMock: vi.fn(),
  logErrorMock: vi.fn(),
}));

const consoleDebugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
const consoleInfoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

vi.mock('@wailsjs/go/main/App', () => ({
  LogFrontendDebug: mocks.logFrontendDebugMock,
  LogFrontendInfo: mocks.logFrontendInfoMock,
  LogFrontendWarning: mocks.logFrontendWarningMock,
  LogFrontendError: mocks.logFrontendErrorMock,
}));

vi.mock('@wailsjs/runtime/runtime', () => ({
  LogDebug: mocks.logDebugMock,
  LogInfo: mocks.logInfoMock,
  LogWarning: mocks.logWarningMock,
  LogError: mocks.logErrorMock,
}));

import * as appLog from './appLog';

describe('appLog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('bridges warnAppEvent to runtime warning and frontend file logging', async () => {
    appLog.warnAppEvent('kline', 'empty data', { requestId: 3, attempt: 2 });
    await Promise.resolve();

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      '[kline] empty data {"requestId":3,"attempt":2}',
      { requestId: 3, attempt: 2 },
    );
    expect(mocks.logWarningMock).toHaveBeenCalledWith('[kline] empty data {"requestId":3,"attempt":2}');
    expect(mocks.logFrontendWarningMock).toHaveBeenCalledWith(
      'kline',
      'empty data',
      '{"requestId":3,"attempt":2}',
    );
  });

  it('exports errorAppEvent and bridges errors to runtime and frontend file logging', async () => {
    expect(typeof (appLog as Record<string, unknown>).errorAppEvent).toBe('function');

    const errorAppEvent = (appLog as { errorAppEvent: (scope: string, message: string, payload?: unknown) => void }).errorAppEvent;
    errorAppEvent('orderbook', 'load failed', { symbol: 'sz000001', err: 'timeout' });
    await Promise.resolve();

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      '[orderbook] load failed {"symbol":"sz000001","err":"timeout"}',
      { symbol: 'sz000001', err: 'timeout' },
    );
    expect(mocks.logErrorMock).toHaveBeenCalledWith('[orderbook] load failed {"symbol":"sz000001","err":"timeout"}');
    expect(mocks.logFrontendErrorMock).toHaveBeenCalledWith(
      'orderbook',
      'load failed',
      '{"symbol":"sz000001","err":"timeout"}',
    );
  });

  it('keeps debugAppEvent behavior unchanged', async () => {
    appLog.debugAppEvent('meeting', 'request start', { stockCode: '600000' });
    await Promise.resolve();

    expect(consoleDebugSpy).toHaveBeenCalledWith(
      '[meeting] request start {"stockCode":"600000"}',
      { stockCode: '600000' },
    );
    expect(mocks.logDebugMock).toHaveBeenCalledWith('[meeting] request start {"stockCode":"600000"}');
    expect(mocks.logFrontendDebugMock).toHaveBeenCalledWith('meeting', 'request start', '{"stockCode":"600000"}');
  });

  it('bridges infoAppEvent to runtime info and frontend file logging', async () => {
    appLog.infoAppEvent('window', 'startup restore', { restoreReason: 'maximized' });
    await Promise.resolve();

    expect(consoleInfoSpy).toHaveBeenCalledWith(
      '[window] startup restore {"restoreReason":"maximized"}',
      { restoreReason: 'maximized' },
    );
    expect(mocks.logInfoMock).toHaveBeenCalledWith('[window] startup restore {"restoreReason":"maximized"}');
    expect(mocks.logFrontendInfoMock).toHaveBeenCalledWith(
      'window',
      'startup restore',
      '{"restoreReason":"maximized"}',
    );
  });
});
