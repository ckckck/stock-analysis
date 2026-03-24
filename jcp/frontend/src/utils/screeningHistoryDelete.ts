export const resolveHistorySelectionAfterDelete = (input: {
  selectedHistoryRunId: number | null;
  deletedRunId: number;
}): number | null => {
  if (input.selectedHistoryRunId === input.deletedRunId) {
    return null;
  }
  return input.selectedHistoryRunId;
};
