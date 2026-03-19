import React from 'react';
import { ArrowRight, Database, History, Sparkles } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { ScreeningHistoryItem } from '../types';

interface ScreeningWorkspaceProps {
  prompt: string;
  resultPreset: string;
  loading: boolean;
  history: ScreeningHistoryItem[];
  selectedRunId: number | null;
  onPromptChange: (value: string) => void;
  onResultPresetChange: (value: string) => void;
  onRun: () => void;
  onSelectHistory: (runId: number) => void;
}

export const ScreeningWorkspace: React.FC<ScreeningWorkspaceProps> = ({
  prompt,
  resultPreset,
  loading,
  history,
  selectedRunId,
  onPromptChange,
  onResultPresetChange,
  onRun,
  onSelectHistory,
}) => {
  const { colors } = useTheme();

  return (
    <div className="flex h-full flex-col">
      <div className="border-b fin-divider-soft px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="rounded-full bg-accent-2/15 p-2 text-accent-2">
            <Database className="h-4 w-4" />
          </div>
          <div>
            <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>AI 筛选工作区</div>
            <div className={`text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>输入自然语言，生成只读 SQL 并读取本地 SQLite 数据</div>
          </div>
        </div>
      </div>

      <div className="border-b fin-divider-soft px-4 py-4">
        <label className={`mb-2 block text-xs font-medium ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>筛选条件</label>
        <textarea
          value={prompt}
          onChange={(event) => onPromptChange(event.target.value)}
          placeholder="例如：找近 30 天放量上涨、回撤不大、今天依旧强势的沪深股票"
          className={`min-h-[96px] w-full resize-none rounded-xl border px-3 py-3 text-sm fin-input ${colors.isDark ? 'placeholder-slate-500' : 'placeholder-slate-400'}`}
        />

        <div className="mt-3 flex items-center gap-3">
          <select
            value={resultPreset}
            onChange={(event) => onResultPresetChange(event.target.value)}
            className={`rounded-lg border px-3 py-2 text-sm fin-input ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}
          >
            <option value="50">前 50 条</option>
            <option value="100">前 100 条</option>
            <option value="200">前 200 条</option>
            <option value="unlimited">不限</option>
          </select>

          <button
            onClick={onRun}
            disabled={loading || !prompt.trim()}
            className="inline-flex items-center gap-2 rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white transition-all hover:bg-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? <Sparkles className="h-4 w-4 animate-pulse" /> : <ArrowRight className="h-4 w-4" />}
            {loading ? '筛选中...' : '重新筛选'}
          </button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden">
        <div className="flex items-center gap-2 px-4 py-3">
          <History className="h-4 w-4 text-accent-2" />
          <span className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>历史筛选记录</span>
        </div>

        <div className="h-full overflow-y-auto fin-scrollbar pb-4">
          {history.length === 0 ? (
            <div className={`px-4 text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>还没有历史筛选记录。</div>
          ) : (
            history.map((item) => {
              const isActive = item.runId === selectedRunId;
              return (
                <button
                  key={item.runId}
                  onClick={() => onSelectHistory(item.runId)}
                  className={`mx-3 mb-2 block w-[calc(100%-24px)] rounded-xl border px-3 py-3 text-left transition-colors ${
                    isActive
                      ? 'border-accent/40 bg-accent/10'
                      : `${colors.isDark ? 'border-slate-700 bg-slate-900/40 hover:border-slate-600 hover:bg-slate-800/40' : 'border-slate-200 bg-white/80 hover:border-slate-300 hover:bg-slate-50'}`
                  }`}
                >
                  <div className={`line-clamp-2 text-sm font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>{item.prompt}</div>
                  <div className={`mt-2 flex items-center justify-between text-[11px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                    <span>{item.createdAt}</span>
                    <span>命中 {item.matchedCount} 条</span>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
