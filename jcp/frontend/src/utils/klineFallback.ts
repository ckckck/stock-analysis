export interface KlineRateLimitToastOptions {
  retriesExhausted: boolean;
  hasAnyCachedData: boolean;
}

export const shouldShowKlineRateLimitToast = ({
  retriesExhausted,
  hasAnyCachedData,
}: KlineRateLimitToastOptions): boolean => {
  return retriesExhausted && !hasAnyCachedData;
};
