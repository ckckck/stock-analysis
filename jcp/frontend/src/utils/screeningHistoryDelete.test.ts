import { describe, expect, it } from 'vitest';
import { resolveHistorySelectionAfterDelete } from './screeningHistoryDelete';

describe('resolveHistorySelectionAfterDelete', () => {
  it('clears selected history item when deleting the selected run', () => {
    expect(resolveHistorySelectionAfterDelete({
      selectedHistoryRunId: 12,
      deletedRunId: 12,
    })).toBeNull();
  });

  it('keeps selected history item when deleting another run', () => {
    expect(resolveHistorySelectionAfterDelete({
      selectedHistoryRunId: 12,
      deletedRunId: 8,
    })).toBe(12);
  });

  it('keeps empty selection unchanged', () => {
    expect(resolveHistorySelectionAfterDelete({
      selectedHistoryRunId: null,
      deletedRunId: 8,
    })).toBeNull();
  });
});
