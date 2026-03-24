import React from 'react';
import { ArrowRight, Sparkles, X } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { resolveScreeningPrimaryActionLabel } from '../utils/screeningSync';

interface ScreeningWorkspaceProps {
  prompt: string;
  resultPreset: string;
  loading: boolean;
  canReuseHistorySql?: boolean;
  generatedSql: string;
  canCancel?: boolean;
  onPromptChange: (value: string) => void;
  onResultPresetChange: (value: string) => void;
  onRun: () => void;
  onCancel: () => void;
  onViewSql: () => void;
}

export const ScreeningWorkspace: React.FC<ScreeningWorkspaceProps> = ({
  prompt,
  resultPreset,
  loading,
  canReuseHistorySql = false,
  generatedSql,
  canCancel = false,
  onPromptChange,
  onResultPresetChange,
  onRun,
  onCancel,
  onViewSql,
}) => {
  const { colors } = useTheme();
  const hasGeneratedSql = generatedSql.trim().length > 0;

  return (
    <div className="flex h-full min-h-[230px] flex-col justify-end">
      <div className="px-4 pb-4 pt-3">
        <div className={`mb-3 text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
          输入自然语言，描述筛选方式
        </div>
        <textarea
          value={prompt}
          onChange={(event) => onPromptChange(event.target.value)}
          placeholder="例如：找近 30 天放量上涨、回撤不大、今天依旧强势的沪深股票"
          className={`min-h-[96px] w-full resize-none rounded-xl border px-3 py-3 text-sm fin-input ${colors.isDark ? 'placeholder-slate-500' : 'placeholder-slate-400'}`}
        />

        <div className="mt-4 flex flex-wrap items-stretch gap-3">
          <select
            value={resultPreset}
            onChange={(event) => onResultPresetChange(event.target.value)}
            className={`w-[132px] flex-none rounded-xl border px-3 py-2.5 text-sm fin-input ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}
          >
            <option value="50">前 50 条</option>
            <option value="100">前 100 条</option>
            <option value="200">前 200 条</option>
            <option value="unlimited">不限</option>
          </select>

          <div className="flex min-w-[220px] flex-1 flex-wrap justify-end gap-3">
            {loading && canCancel && (
              <button
                type="button"
                onClick={onCancel}
                className={`inline-flex flex-1 items-center justify-center gap-2 rounded-2xl border px-5 py-3 text-sm font-semibold transition-colors ${
                  colors.isDark
                    ? 'border-slate-600 bg-slate-900/60 text-slate-200 hover:border-red-400/40 hover:text-white'
                    : 'border-slate-300 bg-white/90 text-slate-600 hover:border-red-300 hover:text-slate-900'
                }`}
              >
                <X className="h-4 w-4" />
                取消筛选
              </button>
            )}
            <button
              onClick={onRun}
              disabled={loading || !prompt.trim()}
              className="inline-flex min-w-[188px] flex-1 items-center justify-center gap-2 rounded-2xl bg-accent px-5 py-3 text-sm font-semibold text-white transition-all hover:bg-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? <Sparkles className="h-4 w-4 animate-pulse" /> : <ArrowRight className="h-4 w-4" />}
              {resolveScreeningPrimaryActionLabel({
                loading,
                canReuseHistorySql,
              })}
            </button>
          </div>
        </div>

        {hasGeneratedSql && (
          <div className="mt-3 flex justify-end">
            <button
              type="button"
              onClick={onViewSql}
              className={`rounded-xl border px-4 py-2 text-sm font-medium transition-colors ${
                colors.isDark
                  ? 'border-slate-700 bg-slate-900/40 text-slate-300 hover:border-accent/40 hover:text-accent-2'
                  : 'border-slate-300 bg-white/90 text-slate-600 hover:border-accent/40 hover:text-accent-2'
              }`}
            >
              查看 SQL
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
