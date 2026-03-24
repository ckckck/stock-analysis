export interface ScreeningHistoryReuseInput {
  currentPrompt: string;
  currentMarketScope: string;
  currentResultMode: 'unlimited' | 'top_n';
  currentResultLimit: number;
  currentTestMode: boolean;
  currentTestLimit: number;
  historyPrompt: string;
  historyMarketScope: string;
  historyResultMode: 'unlimited' | 'top_n';
  historyResultLimit: number;
  historyUsedTestScope: boolean;
  historyTestLimit: number;
}

const normalizePrompt = (prompt: string): string => prompt.trim();

export const canShowHistoryRerunLabel = (input: {
  currentPrompt: string;
  historyPrompt: string;
}): boolean => (
  normalizePrompt(input.currentPrompt) === normalizePrompt(input.historyPrompt)
);

export const canReuseHistorySql = (input: ScreeningHistoryReuseInput): boolean => {
  if (!canShowHistoryRerunLabel({
    currentPrompt: input.currentPrompt,
    historyPrompt: input.historyPrompt,
  })) {
    return false;
  }
  if (input.currentMarketScope !== input.historyMarketScope) {
    return false;
  }
  if (input.currentResultMode !== input.historyResultMode) {
    return false;
  }
  if (input.currentResultLimit !== input.historyResultLimit) {
    return false;
  }
  if (input.currentTestMode !== input.historyUsedTestScope) {
    return false;
  }
  if (!input.currentTestMode) {
    return true;
  }
  return input.currentTestLimit === input.historyTestLimit;
};
