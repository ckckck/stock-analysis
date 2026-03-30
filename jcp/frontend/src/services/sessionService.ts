import { GetOrCreateSession, GetSessionMessages, ClearSessionMessages, SendMeetingMessage, UpdateStockPosition, RetryAgent, RetryAgentAndContinue, CancelInterruptedMeeting } from '../../wailsjs/go/main/App';
import type { StockPosition } from '../types';
import { debugAppEvent, errorAppEvent } from '../utils/appLog';

export interface StockSession {
  id: string;
  stockCode: string;
  stockName: string;
  messages: ChatMessage[];
  position?: StockPosition; // 持仓信息
  createdAt: number;
  updatedAt: number;
}

export interface ChatMessage {
  id: string;
  agentId: string;
  agentName: string;
  role: string;
  content: string;
  timestamp: number;
  replyTo?: string;
  mentions?: string[];
  round?: number;
  msgType?: string;
  error?: string;  // 失败时的错误信息
  meetingMode?: string; // smart=串行, direct=独立
}

// 会议室消息请求
export interface MeetingMessageRequest {
  stockCode: string;
  content: string;
  mentionIds: string[];
  replyToId: string;
  replyContent: string;
  requestId: string;
}

const toErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
};

const nextRequestId = (prefix: string): string => `${prefix}-${Date.now()}`;

// 获取或创建Session
export const getOrCreateSession = async (stockCode: string, stockName: string): Promise<StockSession> => {
  return await GetOrCreateSession(stockCode, stockName);
};

// 获取Session消息
export const getSessionMessages = async (stockCode: string): Promise<ChatMessage[]> => {
  return await GetSessionMessages(stockCode);
};

// 清空Session消息
export const clearSessionMessages = async (stockCode: string): Promise<string> => {
  return await ClearSessionMessages(stockCode);
};

// 发送会议室消息（@指定成员回复）
export const sendMeetingMessage = async (req: MeetingMessageRequest): Promise<ChatMessage[]> => {
  debugAppEvent('meeting', 'frontend request start', {
    module: 'meeting',
    action: 'send_message.request',
    stockCode: req.stockCode,
    requestId: req.requestId,
    mentionCount: req.mentionIds.length,
    hasReply: req.replyToId.trim().length > 0,
    contentLen: req.content.length,
  });
  try {
    const messages = await SendMeetingMessage(req);
    debugAppEvent('meeting', 'frontend request success', {
      module: 'meeting',
      action: 'send_message.success',
      stockCode: req.stockCode,
      requestId: req.requestId,
      resultLen: Array.isArray(messages) ? messages.length : 0,
    });
    return messages;
  } catch (error) {
    errorAppEvent('meeting', 'frontend request failed', {
      module: 'meeting',
      action: 'send_message.failed',
      stockCode: req.stockCode,
      requestId: req.requestId,
      mentionCount: req.mentionIds.length,
      hasReply: req.replyToId.trim().length > 0,
      contentLen: req.content.length,
      err: toErrorMessage(error),
    });
    throw error;
  }
};

// 更新股票持仓信息
export const updateStockPosition = async (stockCode: string, shares: number, costPrice: number): Promise<string> => {
  return await UpdateStockPosition(stockCode, shares, costPrice);
};

// 重试单个失败的专家
export const retryAgent = async (stockCode: string, agentId: string, query: string): Promise<ChatMessage> => {
  const requestId = nextRequestId('retry-agent');
  debugAppEvent('meeting', 'frontend retry agent start', {
    module: 'meeting',
    action: 'retry_agent.request',
    stockCode,
    agentId,
    requestId,
  });
  try {
    const message = await RetryAgent(stockCode, agentId, query);
    debugAppEvent('meeting', 'frontend retry agent success', {
      module: 'meeting',
      action: 'retry_agent.success',
      stockCode,
      agentId,
      requestId,
    });
    return message;
  } catch (error) {
    errorAppEvent('meeting', 'frontend retry agent failed', {
      module: 'meeting',
      action: 'retry_agent.failed',
      stockCode,
      agentId,
      requestId,
      err: toErrorMessage(error),
    });
    throw error;
  }
};

// 重试失败专家并继续执行剩余专家
export const retryAgentAndContinue = async (stockCode: string): Promise<ChatMessage[]> => {
  const requestId = nextRequestId('retry-continue');
  debugAppEvent('meeting', 'frontend retry continue start', {
    module: 'meeting',
    action: 'retry_continue.request',
    stockCode,
    requestId,
  });
  try {
    const messages = await RetryAgentAndContinue(stockCode);
    debugAppEvent('meeting', 'frontend retry continue success', {
      module: 'meeting',
      action: 'retry_continue.success',
      stockCode,
      requestId,
      resultLen: Array.isArray(messages) ? messages.length : 0,
    });
    return messages;
  } catch (error) {
    errorAppEvent('meeting', 'frontend retry continue failed', {
      module: 'meeting',
      action: 'retry_continue.failed',
      stockCode,
      requestId,
      err: toErrorMessage(error),
    });
    throw error;
  }
};

// 取消中断的会议（用户放弃重试）
export const cancelInterruptedMeeting = async (stockCode: string): Promise<boolean> => {
  return await CancelInterruptedMeeting(stockCode);
};
