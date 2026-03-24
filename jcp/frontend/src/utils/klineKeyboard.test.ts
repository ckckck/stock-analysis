import { describe, expect, it } from 'vitest';
import {
  DEFAULT_KLINE_ZOOM_PERCENT,
  MAX_KLINE_ZOOM_PERCENT,
  MIN_KLINE_ZOOM_PERCENT,
  buildKlineZoomLogicalRange,
  getKlinePointerInteractionOptions,
  getNextKlineZoomPercent,
  getNextEffectiveKlineZoomPercent,
  panKlineLogicalRange,
  resolveKlineKeyboardAction,
  shouldFocusKeyboardHostOnPointerDown,
  shouldBlurActiveEditableOnPointerDown,
} from './klineKeyboard';

describe('resolveKlineKeyboardAction', () => {
  it('maps bare arrow keys to chart actions', () => {
    expect(resolveKlineKeyboardAction({ key: 'ArrowUp' })).toBe('zoom_in');
    expect(resolveKlineKeyboardAction({ key: 'ArrowDown' })).toBe('zoom_out');
    expect(resolveKlineKeyboardAction({ key: 'ArrowLeft' })).toBe('pan_left');
    expect(resolveKlineKeyboardAction({ key: 'ArrowRight' })).toBe('pan_right');
  });

  it('ignores modifiers and editable targets', () => {
    expect(resolveKlineKeyboardAction({ key: 'ArrowUp', metaKey: true })).toBeNull();
    expect(resolveKlineKeyboardAction({
      key: 'ArrowLeft',
      target: { tagName: 'input', isContentEditable: false },
    })).toBeNull();
    expect(resolveKlineKeyboardAction({
      key: 'ArrowRight',
      target: { tagName: 'div', isContentEditable: true },
    })).toBeNull();
  });

  it('ignores arrow keys only when explicit editing mode is active', () => {
    expect(resolveKlineKeyboardAction({
      key: 'ArrowLeft',
      target: { tagName: 'input', isContentEditable: false },
      editingMode: true,
    })).toBeNull();
    expect(resolveKlineKeyboardAction({
      key: 'ArrowLeft',
      target: { tagName: 'input', isContentEditable: false },
      editingMode: false,
    })).toBe('pan_left');
  });

});

describe('shouldBlurActiveEditableOnPointerDown', () => {
  it('blurs editing focus when clicking outside editable areas', () => {
    expect(shouldBlurActiveEditableOnPointerDown({
      activeTarget: { tagName: 'input', isContentEditable: false },
      pointerTarget: { tagName: 'div', isContentEditable: false },
    })).toBe(true);
  });

  it('keeps focus when clicking inside another editable area', () => {
    expect(shouldBlurActiveEditableOnPointerDown({
      activeTarget: { tagName: 'textarea', isContentEditable: false },
      pointerTarget: { tagName: 'input', isContentEditable: false },
    })).toBe(false);
  });
});

describe('shouldFocusKeyboardHostOnPointerDown', () => {
  it('focuses the keyboard host after clicking non-editable areas', () => {
    expect(shouldFocusKeyboardHostOnPointerDown({
      tagName: 'div',
      isContentEditable: false,
    })).toBe(true);
    expect(shouldFocusKeyboardHostOnPointerDown({
      tagName: 'button',
      isContentEditable: false,
    })).toBe(true);
  });

  it('does not focus the keyboard host after clicking editable areas', () => {
    expect(shouldFocusKeyboardHostOnPointerDown({
      tagName: 'input',
      isContentEditable: false,
    })).toBe(false);
    expect(shouldFocusKeyboardHostOnPointerDown({
      tagName: 'div',
      isContentEditable: true,
    })).toBe(false);
  });
});

describe('getNextKlineZoomPercent', () => {
  it('uses 100 percent as the default baseline', () => {
    expect(getNextKlineZoomPercent(undefined, 'zoom_in')).toBe(DEFAULT_KLINE_ZOOM_PERCENT + 10);
  });

  it('changes the zoom in fixed steps with bounds', () => {
    expect(getNextKlineZoomPercent(100, 'zoom_in')).toBe(110);
    expect(getNextKlineZoomPercent(100, 'zoom_out')).toBe(90);
    expect(getNextKlineZoomPercent(MAX_KLINE_ZOOM_PERCENT, 'zoom_in')).toBe(MAX_KLINE_ZOOM_PERCENT);
    expect(getNextKlineZoomPercent(MIN_KLINE_ZOOM_PERCENT, 'zoom_out')).toBe(MIN_KLINE_ZOOM_PERCENT);
  });
});

describe('getNextEffectiveKlineZoomPercent', () => {
  it('skips zoom-in levels that would keep the chart visually unchanged', () => {
    expect(getNextEffectiveKlineZoomPercent({
      currentZoomPercent: 50,
      action: 'zoom_in',
      baseRange: { from: 69.31145247723717, to: 235 },
      bounds: { from: 0, to: 237 },
      currentRange: { from: 0, to: 237 },
    })).toBe(80);
  });

  it('keeps the immediate step when it already changes the visible range', () => {
    expect(getNextEffectiveKlineZoomPercent({
      currentZoomPercent: 90,
      action: 'zoom_in',
      baseRange: { from: 69.31145247723717, to: 235 },
      bounds: { from: 0, to: 237 },
      currentRange: { from: 52.90161386359685, to: 237 },
    })).toBe(100);
  });
});

describe('getKlinePointerInteractionOptions', () => {
  it('keeps default chart scroll and scale gestures enabled for all periods', () => {
    expect(getKlinePointerInteractionOptions('1m')).toEqual({
      handleScroll: true,
      handleScale: true,
    });
    expect(getKlinePointerInteractionOptions('1d')).toEqual({
      handleScroll: true,
      handleScale: true,
    });
  });
});

describe('buildKlineZoomLogicalRange', () => {
  it('shrinks the visible logical width when zooming in', () => {
    expect(buildKlineZoomLogicalRange({
      baseRange: { from: 0, to: 100 },
      currentRange: { from: 20, to: 100 },
      zoomPercent: 200,
    })).toEqual({
      from: 50,
      to: 100,
    });
  });

  it('expands the visible logical width when zooming out', () => {
    expect(buildKlineZoomLogicalRange({
      baseRange: { from: 0, to: 100 },
      currentRange: { from: 40, to: 100 },
      zoomPercent: 100,
    })).toEqual({
      from: 0,
      to: 100,
    });
  });

  it('expands beyond the fitted window when wider logical bounds exist', () => {
    expect(buildKlineZoomLogicalRange({
      baseRange: { from: 100, to: 200 },
      bounds: { from: 0, to: 300 },
      currentRange: { from: 100, to: 200 },
      zoomPercent: 50,
    })).toEqual({
      from: 0,
      to: 200,
    });
  });
});

describe('panKlineLogicalRange', () => {
  it('moves the visible range left and right within the fitted bounds', () => {
    expect(panKlineLogicalRange({
      currentRange: { from: 50, to: 100 },
      bounds: { from: 0, to: 100 },
      direction: 'pan_left',
    })).toEqual({
      from: 40,
      to: 90,
    });

    expect(panKlineLogicalRange({
      currentRange: { from: 40, to: 90 },
      bounds: { from: 0, to: 100 },
      direction: 'pan_right',
    })).toEqual({
      from: 50,
      to: 100,
    });
  });

  it('clamps panning at the fitted bounds', () => {
    expect(panKlineLogicalRange({
      currentRange: { from: 0, to: 50 },
      bounds: { from: 0, to: 100 },
      direction: 'pan_left',
    })).toEqual({
      from: 0,
      to: 50,
    });

    expect(panKlineLogicalRange({
      currentRange: { from: 50, to: 100 },
      bounds: { from: 0, to: 100 },
      direction: 'pan_right',
    })).toEqual({
      from: 50,
      to: 100,
    });
  });
});
