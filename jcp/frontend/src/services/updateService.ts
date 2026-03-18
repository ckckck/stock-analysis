import { CheckForUpdate, DoUpdate, RestartApp, GetCurrentVersion } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

export interface UpdateInfo {
  hasUpdate: boolean;
  latestVersion: string;
  currentVersion: string;
  releaseUrl: string;
  releaseNotes: string;
  error?: string;
}

export interface UpdateProgress {
  status: 'checking' | 'downloading' | 'installing' | 'completed' | 'error';
  message: string;
  percent: number;
}

export async function checkForUpdate(): Promise<UpdateInfo> {
  return await CheckForUpdate();
}

export async function doUpdate(): Promise<string> {
  return await DoUpdate();
}

export async function restartApp(): Promise<string> {
  return await RestartApp();
}

export async function getCurrentVersion(): Promise<string> {
  return await GetCurrentVersion();
}

export function onUpdateProgress(callback: (progress: UpdateProgress) => void): () => void {
  EventsOn('update:progress', callback);
  return () => EventsOff('update:progress');
}
