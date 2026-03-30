import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  sendMeetingMessageMock: vi.fn(),
  retryAgentMock: vi.fn(),
  retryAgentAndContinueMock: vi.fn(),
  debugAppEventMock: vi.fn(),
  errorAppEventMock: vi.fn(),
}));

vi.mock('../../wailsjs/go/main/App', () => ({
  GetOrCreateSession: vi.fn(),
  GetSessionMessages: vi.fn(),
  ClearSessionMessages: vi.fn(),
  SendMeetingMessage: mocks.sendMeetingMessageMock,
  UpdateStockPosition: vi.fn(),
  RetryAgent: mocks.retryAgentMock,
  RetryAgentAndContinue: mocks.retryAgentAndContinueMock,
  CancelInterruptedMeeting: vi.fn(),
}));

vi.mock('../utils/appLog', () => ({
  debugAppEvent: mocks.debugAppEventMock,
  errorAppEvent: mocks.errorAppEventMock,
}));

import { retryAgent, retryAgentAndContinue, sendMeetingMessage, type MeetingMessageRequest } from './sessionService';

describe('sessionService.sendMeetingMessage', () => {
  const req: MeetingMessageRequest = {
    stockCode: 'sh600000',
    content: '请分析这只股票',
    mentionIds: ['bull', 'bear'],
    replyToId: 'msg-1',
    replyContent: '上一条消息',
    requestId: 'meeting-1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('records meeting request and success summaries', async () => {
    mocks.sendMeetingMessageMock.mockResolvedValueOnce([{ id: 'resp-1' }]);

    const result = await sendMeetingMessage(req);

    expect(result).toEqual([{ id: 'resp-1' }]);
    expect(mocks.debugAppEventMock).toHaveBeenNthCalledWith(1, 'meeting', 'frontend request start', {
      module: 'meeting',
      action: 'send_message.request',
      stockCode: 'sh600000',
      requestId: 'meeting-1',
      mentionCount: 2,
      hasReply: true,
      contentLen: 7,
    });
    expect(mocks.debugAppEventMock).toHaveBeenNthCalledWith(2, 'meeting', 'frontend request success', {
      module: 'meeting',
      action: 'send_message.success',
      stockCode: 'sh600000',
      requestId: 'meeting-1',
      resultLen: 1,
    });
  });

  it('records meeting request failures as durable error logs', async () => {
    mocks.sendMeetingMessageMock.mockRejectedValueOnce(new Error('timeout'));

    await expect(sendMeetingMessage(req)).rejects.toThrow('timeout');

    expect(mocks.errorAppEventMock).toHaveBeenCalledWith('meeting', 'frontend request failed', {
      module: 'meeting',
      action: 'send_message.failed',
      stockCode: 'sh600000',
      requestId: 'meeting-1',
      mentionCount: 2,
      hasReply: true,
      contentLen: 7,
      err: 'timeout',
    });
  });

  it('records retry agent request and success summaries', async () => {
    mocks.retryAgentMock.mockResolvedValueOnce({ id: 'retry-1' });
    vi.spyOn(Date, 'now').mockReturnValueOnce(123456);

    const result = await retryAgent('sh600000', 'bull', '继续分析');

    expect(result).toEqual({ id: 'retry-1' });
    expect(mocks.debugAppEventMock).toHaveBeenNthCalledWith(1, 'meeting', 'frontend retry agent start', {
      module: 'meeting',
      action: 'retry_agent.request',
      stockCode: 'sh600000',
      agentId: 'bull',
      requestId: 'retry-agent-123456',
    });
    expect(mocks.debugAppEventMock).toHaveBeenNthCalledWith(2, 'meeting', 'frontend retry agent success', {
      module: 'meeting',
      action: 'retry_agent.success',
      stockCode: 'sh600000',
      agentId: 'bull',
      requestId: 'retry-agent-123456',
    });
  });

  it('records retry continue request and failure summaries', async () => {
    mocks.retryAgentAndContinueMock.mockRejectedValueOnce(new Error('interrupted'));
    vi.spyOn(Date, 'now').mockReturnValueOnce(222333);

    await expect(retryAgentAndContinue('sh600000')).rejects.toThrow('interrupted');

    expect(mocks.errorAppEventMock).toHaveBeenCalledWith('meeting', 'frontend retry continue failed', {
      module: 'meeting',
      action: 'retry_continue.failed',
      stockCode: 'sh600000',
      requestId: 'retry-continue-222333',
      err: 'interrupted',
    });
  });
});
