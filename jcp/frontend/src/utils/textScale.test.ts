import { describe, expect, it } from 'vitest';
import {
  DEFAULT_TEXT_SCALE_PERCENT,
  MAX_TEXT_SCALE_PERCENT,
  MIN_TEXT_SCALE_PERCENT,
  getNextTextScalePercent,
  resolveTextScaleShortcutDirection,
} from './textScale';

describe('resolveTextScaleShortcutDirection', () => {
  it('detects increase shortcuts for meta/ctrl plus keys', () => {
    expect(resolveTextScaleShortcutDirection({
      metaKey: true,
      ctrlKey: false,
      altKey: false,
      key: '=',
    })).toBe('increase');

    expect(resolveTextScaleShortcutDirection({
      metaKey: false,
      ctrlKey: true,
      altKey: false,
      key: 'NumpadAdd',
    })).toBe('increase');
  });

  it('detects decrease shortcuts for meta/ctrl minus keys', () => {
    expect(resolveTextScaleShortcutDirection({
      metaKey: true,
      ctrlKey: false,
      altKey: false,
      key: '-',
    })).toBe('decrease');

    expect(resolveTextScaleShortcutDirection({
      metaKey: false,
      ctrlKey: true,
      altKey: false,
      key: 'Subtract',
    })).toBe('decrease');
  });

  it('ignores unsupported keys and modifier combinations', () => {
    expect(resolveTextScaleShortcutDirection({
      metaKey: false,
      ctrlKey: false,
      altKey: false,
      key: '=',
    })).toBeNull();

    expect(resolveTextScaleShortcutDirection({
      metaKey: true,
      ctrlKey: false,
      altKey: true,
      key: '=',
    })).toBeNull();
  });
});

describe('getNextTextScalePercent', () => {
  it('uses 100 percent as the default baseline', () => {
    expect(getNextTextScalePercent(undefined, 'increase')).toBe(DEFAULT_TEXT_SCALE_PERCENT + 10);
  });

  it('increases and decreases in fixed 10 percent steps', () => {
    expect(getNextTextScalePercent(100, 'increase')).toBe(110);
    expect(getNextTextScalePercent(100, 'decrease')).toBe(90);
  });

  it('clamps the value within the supported range', () => {
    expect(getNextTextScalePercent(MAX_TEXT_SCALE_PERCENT, 'increase')).toBe(MAX_TEXT_SCALE_PERCENT);
    expect(getNextTextScalePercent(MIN_TEXT_SCALE_PERCENT, 'decrease')).toBe(MIN_TEXT_SCALE_PERCENT);
  });
});
