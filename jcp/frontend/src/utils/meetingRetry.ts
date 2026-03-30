import type { ChatMessage } from '../services/sessionService';

export type RetryFeedbackSource = 'latest' | 'fallback' | 'current';

export interface RetryFeedbackResolution {
  messages: ChatMessage[];
  source: RetryFeedbackSource;
  latestResultLen: number;
  fallbackResultLen: number;
}

const toStampedMessages = (
  messages: ChatMessage[],
  now: () => number,
  random: () => number,
): ChatMessage[] => messages.map((message) => ({
  ...message,
  id: message.id || `msg-${now()}-${random()}`,
  timestamp: message.timestamp || now(),
}));

export const resolveRetryFeedbackMessages = (
  currentMessages: ChatMessage[],
  latestMessages: ChatMessage[] | null | undefined,
  fallbackResult: ChatMessage | ChatMessage[] | null | undefined,
  now: () => number = () => Date.now(),
  random: () => number = () => Math.random(),
): ChatMessage[] => {
  return resolveRetryFeedback(currentMessages, latestMessages, fallbackResult, now, random).messages;
};

export const resolveRetryFeedback = (
  currentMessages: ChatMessage[],
  latestMessages: ChatMessage[] | null | undefined,
  fallbackResult: ChatMessage | ChatMessage[] | null | undefined,
  now: () => number = () => Date.now(),
  random: () => number = () => Math.random(),
): RetryFeedbackResolution => {
  const latestResultLen = Array.isArray(latestMessages) ? latestMessages.length : 0;
  const fallbackMessages = fallbackResult
    ? (Array.isArray(fallbackResult) ? fallbackResult : [fallbackResult])
    : [];
  const fallbackResultLen = fallbackMessages.length;

  if (latestResultLen > 0) {
    return {
      messages: latestMessages as ChatMessage[],
      source: 'latest',
      latestResultLen,
      fallbackResultLen,
    };
  }

  if (fallbackResultLen === 0) {
    return {
      messages: currentMessages,
      source: 'current',
      latestResultLen,
      fallbackResultLen,
    };
  }

  return {
    messages: [...currentMessages, ...toStampedMessages(fallbackMessages, now, random)],
    source: 'fallback',
    latestResultLen,
    fallbackResultLen,
  };
};
