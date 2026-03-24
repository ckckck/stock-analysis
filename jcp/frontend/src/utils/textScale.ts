export const DEFAULT_TEXT_SCALE_PERCENT = 100;
export const MIN_TEXT_SCALE_PERCENT = 80;
export const MAX_TEXT_SCALE_PERCENT = 140;
export const TEXT_SCALE_STEP_PERCENT = 10;

export type TextScaleShortcutDirection = 'increase' | 'decrease';

export interface TextScaleShortcutEvent {
  metaKey: boolean;
  ctrlKey: boolean;
  altKey?: boolean;
  key: string;
}

const INCREASE_KEYS = new Set(['+', '=', 'Add', 'NumpadAdd']);
const DECREASE_KEYS = new Set(['-', '_', 'Subtract', 'NumpadSubtract']);

export const clampTextScalePercent = (value: number): number => (
  Math.max(MIN_TEXT_SCALE_PERCENT, Math.min(MAX_TEXT_SCALE_PERCENT, Math.round(value)))
);

export const getNextTextScalePercent = (
  current: number | null | undefined,
  direction: TextScaleShortcutDirection,
): number => {
  const baseline = clampTextScalePercent(current ?? DEFAULT_TEXT_SCALE_PERCENT);
  const delta = direction === 'increase' ? TEXT_SCALE_STEP_PERCENT : -TEXT_SCALE_STEP_PERCENT;
  return clampTextScalePercent(baseline + delta);
};

export const resolveTextScaleShortcutDirection = (
  event: TextScaleShortcutEvent,
): TextScaleShortcutDirection | null => {
  if (!(event.metaKey || event.ctrlKey) || event.altKey) {
    return null;
  }
  if (INCREASE_KEYS.has(event.key)) {
    return 'increase';
  }
  if (DECREASE_KEYS.has(event.key)) {
    return 'decrease';
  }
  return null;
};
