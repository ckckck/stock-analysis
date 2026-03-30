export interface StartupWindowRestoreDecision {
  shouldRestore: boolean;
  reason: 'maximized' | 'restore' | 'missing-size';
  savedWindowWidth: number;
  savedWindowHeight: number;
}

export const resolveStartupWindowRestore = (
  savedWindowWidth: number,
  savedWindowHeight: number,
  isMaximized: boolean,
): StartupWindowRestoreDecision => {
  if (savedWindowWidth <= 0 || savedWindowHeight <= 0) {
    return {
      shouldRestore: false,
      reason: 'missing-size',
      savedWindowWidth,
      savedWindowHeight,
    };
  }

  if (isMaximized) {
    return {
      shouldRestore: false,
      reason: 'maximized',
      savedWindowWidth,
      savedWindowHeight,
    };
  }

  return {
    shouldRestore: true,
    reason: 'restore',
    savedWindowWidth,
    savedWindowHeight,
  };
};
