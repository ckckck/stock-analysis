import { describe, expect, it } from 'vitest';

import { shouldShowKlineRateLimitToast } from './klineFallback';

describe('shouldShowKlineRateLimitToast', () => {
  it('returns true when retries are exhausted and symbol has no cached kline data', () => {
    expect(shouldShowKlineRateLimitToast({
      retriesExhausted: true,
      hasAnyCachedData: false,
    })).toBe(true);
  });

  it('returns false when symbol already has cached kline data', () => {
    expect(shouldShowKlineRateLimitToast({
      retriesExhausted: true,
      hasAnyCachedData: true,
    })).toBe(false);
  });

  it('returns false before retries are exhausted', () => {
    expect(shouldShowKlineRateLimitToast({
      retriesExhausted: false,
      hasAnyCachedData: false,
    })).toBe(false);
  });
});
