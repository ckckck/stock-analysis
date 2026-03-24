import { LogFrontendDebug } from '@wailsjs/go/main/App';
import { LogDebug, LogWarning } from '@wailsjs/runtime/runtime';

const serialize = (payload?: unknown): string => {
  if (payload === undefined) {
    return '';
  }

  try {
    return ` ${JSON.stringify(payload)}`;
  } catch (error) {
    return ` {"serializeError":${JSON.stringify(String(error))}}`;
  }
};

const serializePayload = (payload?: unknown): string => {
  if (payload === undefined) {
    return '';
  }

  try {
    return JSON.stringify(payload);
  } catch (error) {
    return JSON.stringify({ serializeError: String(error) });
  }
};

const safeCall = (fn: (message: string) => void, message: string): void => {
  try {
    fn(message);
  } catch {
    // 在非 Wails 环境下忽略日志桥失败，避免影响本地测试。
  }
};

export const debugAppEvent = (scope: string, message: string, payload?: unknown): void => {
  const line = `[${scope}] ${message}${serialize(payload)}`;
  console.debug(line, payload);
  safeCall(LogDebug, line);
  void LogFrontendDebug(scope, message, serializePayload(payload)).catch(() => {
    // 绑定尚未就绪时忽略，避免影响前端行为。
  });
};

export const warnAppEvent = (scope: string, message: string, payload?: unknown): void => {
  const line = `[${scope}] ${message}${serialize(payload)}`;
  console.warn(line, payload);
  safeCall(LogWarning, line);
};
