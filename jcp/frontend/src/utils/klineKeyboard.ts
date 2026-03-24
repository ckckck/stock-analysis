export const DEFAULT_KLINE_ZOOM_PERCENT = 100;
export const MIN_KLINE_ZOOM_PERCENT = 50;
export const MAX_KLINE_ZOOM_PERCENT = 200;
export const KLINE_ZOOM_STEP_PERCENT = 10;
export const KLINE_PAN_STEP_RATIO = 0.2;
export const MIN_VISIBLE_LOGICAL_BARS = 10;
export const MIN_EFFECTIVE_LOGICAL_RANGE_DELTA = 1;

export type KlineKeyboardAction = 'zoom_in' | 'zoom_out' | 'pan_left' | 'pan_right';

export interface KlineKeyboardEventLike {
  key: string;
  metaKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  shiftKey?: boolean;
  editingMode?: boolean;
  target?: {
    tagName?: string | null;
    isContentEditable?: boolean | null;
  } | null;
}

export interface LogicalRangeLike {
  from: number;
  to: number;
}

const clamp = (value: number, min: number, max: number): number => (
  Math.max(min, Math.min(max, value))
);

export const isEditableTarget = (target: KlineKeyboardEventLike['target']): boolean => {
  const tagName = (target?.tagName || '').toUpperCase();
  return Boolean(
    target?.isContentEditable ||
    tagName === 'INPUT' ||
    tagName === 'TEXTAREA' ||
    tagName === 'SELECT',
  );
};

export const resolveKlineKeyboardAction = (
  event: KlineKeyboardEventLike,
): KlineKeyboardAction | null => {
  if (event.metaKey || event.ctrlKey || event.altKey) {
    return null;
  }
  if (event.editingMode ?? isEditableTarget(event.target)) {
    return null;
  }

  switch (event.key) {
    case 'ArrowUp':
      return 'zoom_in';
    case 'ArrowDown':
      return 'zoom_out';
    case 'ArrowLeft':
      return 'pan_left';
    case 'ArrowRight':
      return 'pan_right';
    default:
      return null;
  }
};

export const shouldBlurActiveEditableOnPointerDown = ({
  activeTarget,
  pointerTarget,
}: {
  activeTarget?: KlineKeyboardEventLike['target'];
  pointerTarget?: KlineKeyboardEventLike['target'];
}): boolean => (
  isEditableTarget(activeTarget) && !isEditableTarget(pointerTarget)
);

export const shouldFocusKeyboardHostOnPointerDown = (
  pointerTarget?: KlineKeyboardEventLike['target'],
): boolean => !isEditableTarget(pointerTarget);

export const getKlinePointerInteractionOptions = (_period: string): {
  handleScroll: boolean;
  handleScale: boolean;
} => ({
  handleScroll: true,
  handleScale: true,
});

export const getNextKlineZoomPercent = (
  current: number | null | undefined,
  action: 'zoom_in' | 'zoom_out',
): number => {
  const baseline = clamp(Math.round(current ?? DEFAULT_KLINE_ZOOM_PERCENT), MIN_KLINE_ZOOM_PERCENT, MAX_KLINE_ZOOM_PERCENT);
  const delta = action === 'zoom_in' ? KLINE_ZOOM_STEP_PERCENT : -KLINE_ZOOM_STEP_PERCENT;
  return clamp(baseline + delta, MIN_KLINE_ZOOM_PERCENT, MAX_KLINE_ZOOM_PERCENT);
};

export const buildKlineZoomLogicalRange = ({
  baseRange,
  bounds,
  currentRange,
  zoomPercent,
}: {
  baseRange: LogicalRangeLike;
  bounds?: LogicalRangeLike | null;
  currentRange: LogicalRangeLike | null;
  zoomPercent: number;
}): LogicalRangeLike => {
  const boundsRange = bounds ?? baseRange;
  const boundsWidth = Math.max(1, boundsRange.to - boundsRange.from);
  const minVisibleWidth = Math.min(MIN_VISIBLE_LOGICAL_BARS, boundsWidth);
  const baseWidth = Math.max(1, Math.min(baseRange.to - baseRange.from, boundsWidth));
  const normalizedZoom = clamp(zoomPercent, MIN_KLINE_ZOOM_PERCENT, MAX_KLINE_ZOOM_PERCENT);
  const targetWidth = clamp(
    baseWidth * (DEFAULT_KLINE_ZOOM_PERCENT / normalizedZoom),
    minVisibleWidth,
    boundsWidth,
  );
  const currentTo = currentRange?.to ?? baseRange.to;
  const clampedTo = clamp(currentTo, boundsRange.from + targetWidth, boundsRange.to);
  return {
    from: clampedTo - targetWidth,
    to: clampedTo,
  };
};

export const getNextEffectiveKlineZoomPercent = ({
  currentZoomPercent,
  action,
  baseRange,
  bounds,
  currentRange,
}: {
  currentZoomPercent: number | null | undefined;
  action: 'zoom_in' | 'zoom_out';
  baseRange: LogicalRangeLike;
  bounds?: LogicalRangeLike | null;
  currentRange: LogicalRangeLike | null;
}): number => {
  const baseline = clamp(
    Math.round(currentZoomPercent ?? DEFAULT_KLINE_ZOOM_PERCENT),
    MIN_KLINE_ZOOM_PERCENT,
    MAX_KLINE_ZOOM_PERCENT,
  );
  if (!currentRange) {
    return getNextKlineZoomPercent(baseline, action);
  }

  let candidate = baseline;
  while (true) {
    const next = getNextKlineZoomPercent(candidate, action);
    if (next === candidate) {
      return next;
    }

    const nextRange = buildKlineZoomLogicalRange({
      baseRange,
      bounds,
      currentRange,
      zoomPercent: next,
    });
    const rangeDelta = Math.max(
      Math.abs(nextRange.from - currentRange.from),
      Math.abs(nextRange.to - currentRange.to),
      Math.abs((nextRange.to - nextRange.from) - (currentRange.to - currentRange.from)),
    );
    if (rangeDelta >= MIN_EFFECTIVE_LOGICAL_RANGE_DELTA) {
      return next;
    }

    candidate = next;
  }
};

export const panKlineLogicalRange = ({
  currentRange,
  bounds,
  direction,
}: {
  currentRange: LogicalRangeLike;
  bounds: LogicalRangeLike;
  direction: 'pan_left' | 'pan_right';
}): LogicalRangeLike => {
  const width = Math.max(1, currentRange.to - currentRange.from);
  const step = Math.max(1, Math.round(width * KLINE_PAN_STEP_RATIO));
  const delta = direction === 'pan_left' ? -step : step;
  const unclampedFrom = currentRange.from + delta;
  const unclampedTo = currentRange.to + delta;

  if (unclampedFrom < bounds.from) {
    return {
      from: bounds.from,
      to: bounds.from + width,
    };
  }

  if (unclampedTo > bounds.to) {
    return {
      from: bounds.to - width,
      to: bounds.to,
    };
  }

  return {
    from: unclampedFrom,
    to: unclampedTo,
  };
};
