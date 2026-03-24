import { describe, expect, it } from 'vitest';
import { canReuseHistorySql, canShowHistoryRerunLabel } from './screeningHistoryReuse';

describe('canReuseHistorySql', () => {
  const baseInput = {
    currentPrompt: '最近20日放量上涨',
    currentMarketScope: 'shanghai,shenzhen',
    currentResultMode: 'top_n' as const,
    currentResultLimit: 50,
    currentTestMode: false,
    currentTestLimit: 0,
    historyPrompt: '最近20日放量上涨',
    historyMarketScope: 'shanghai,shenzhen',
    historyResultMode: 'top_n' as const,
    historyResultLimit: 50,
    historyUsedTestScope: false,
    historyTestLimit: 0,
  };

  it('returns true when prompt and scope are unchanged', () => {
    expect(canReuseHistorySql(baseInput)).toBe(true);
  });

  it('returns false when prompt changes', () => {
    expect(canReuseHistorySql({
      ...baseInput,
      currentPrompt: '最近20日缩量回调后放量反包',
    })).toBe(false);
  });

  it('returns false when market scope changes', () => {
    expect(canReuseHistorySql({
      ...baseInput,
      currentMarketScope: 'shanghai,shenzhen,beijing',
    })).toBe(false);
  });

  it('returns false when test scope changes', () => {
    expect(canReuseHistorySql({
      ...baseInput,
      currentTestMode: true,
      currentTestLimit: 20,
    })).toBe(false);
  });

  it('returns false when result preset changes', () => {
    expect(canReuseHistorySql({
      ...baseInput,
      currentResultLimit: 100,
    })).toBe(false);
  });
});

describe('canShowHistoryRerunLabel', () => {
  it('returns true when prompt is unchanged from history', () => {
    expect(canShowHistoryRerunLabel({
      currentPrompt: '最近20日放量上涨',
      historyPrompt: '最近20日放量上涨',
    })).toBe(true);
  });

  it('returns false when prompt changes', () => {
    expect(canShowHistoryRerunLabel({
      currentPrompt: '最近20日缩量回调',
      historyPrompt: '最近20日放量上涨',
    })).toBe(false);
  });
});
