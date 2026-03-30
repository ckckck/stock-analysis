import { describe, expect, it } from 'vitest';
import { resolveStartupWindowRestore } from './windowLayout';

describe('resolveStartupWindowRestore', () => {
  it('skips restoring saved window size when the window is already maximized', () => {
    expect(resolveStartupWindowRestore(1600, 900, true)).toEqual({
      shouldRestore: false,
      reason: 'maximized',
      savedWindowWidth: 1600,
      savedWindowHeight: 900,
    });
  });

  it('restores saved window size when a valid saved size exists and the window is not maximized', () => {
    expect(resolveStartupWindowRestore(1600, 900, false)).toEqual({
      shouldRestore: true,
      reason: 'restore',
      savedWindowWidth: 1600,
      savedWindowHeight: 900,
    });
  });

  it('skips restoring when saved size is missing', () => {
    expect(resolveStartupWindowRestore(0, 900, false)).toEqual({
      shouldRestore: false,
      reason: 'missing-size',
      savedWindowWidth: 0,
      savedWindowHeight: 900,
    });
  });
});
