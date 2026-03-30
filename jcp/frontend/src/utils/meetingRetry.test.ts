import { describe, expect, it } from 'vitest';
import type { ChatMessage } from '../services/sessionService';
import { resolveRetryFeedback, resolveRetryFeedbackMessages } from './meetingRetry';

describe('resolveRetryFeedbackMessages', () => {
  const current: ChatMessage[] = [
    { id: 'existing-1', agentId: 'user', agentName: '老散牛', role: '', content: '原消息', timestamp: 100 },
  ];

  it('prefers latest session messages when available', () => {
    const latest: ChatMessage[] = [{ id: 'stored-1', agentId: 'a', agentName: 'A', role: 'r', content: 'done', timestamp: 200 }];
    const fallback: ChatMessage = { id: '', agentId: 'b', agentName: 'B', role: 'r', content: 'fallback', timestamp: 0 };

    expect(resolveRetryFeedbackMessages(current, latest, fallback)).toEqual(latest);
  });

  it('falls back to retry result and appends stamped message when latest session messages are unavailable', () => {
    const fallback: ChatMessage = { id: '', agentId: 'b', agentName: 'B', role: 'r', content: 'fallback', timestamp: 0 };

    expect(resolveRetryFeedbackMessages(current, [], fallback, () => 123, () => 0.5)).toEqual([
      ...current,
      { ...fallback, id: 'msg-123-0.5', timestamp: 123 },
    ]);
  });

  it('returns latest as the resolution source when latest session messages are available', () => {
    const latest: ChatMessage[] = [{ id: 'stored-2', agentId: 'c', agentName: 'C', role: 'r', content: 'latest', timestamp: 300 }];

    expect(resolveRetryFeedback(current, latest, null)).toEqual({
      messages: latest,
      source: 'latest',
      latestResultLen: 1,
      fallbackResultLen: 0,
    });
  });

  it('returns current as the resolution source when neither latest nor fallback messages exist', () => {
    expect(resolveRetryFeedback(current, [], null)).toEqual({
      messages: current,
      source: 'current',
      latestResultLen: 0,
      fallbackResultLen: 0,
    });
  });
});
