import React from 'react';
import { Check, CheckCircle2, ChevronRight, CircleDashed, History, Loader2, Plus, X } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { useCandleColor } from '../contexts/CandleColorContext';
import { ScreeningHistoryItem, ScreeningQueryProgress, ScreeningResultTab, ScreeningRunResult, Stock } from '../types';
import { formatDateTimeDisplay } from '../utils/datetime';

interface ScreeningResultListProps {
  activeTab: ScreeningResultTab;
  results: ScreeningRunResult[];
  history: ScreeningHistoryItem[];
  generatedSql: string;
  selectedSymbol: string;
  selectedHistoryRunId: number | null;
  totalCount: number;
  isLoading: boolean;
  queryProgress?: ScreeningQueryProgress | null;
  error: string;
  watchlistSymbols: Set<string>;
  deletingHistoryRunId?: number | null;
  onTabChange: (tab: ScreeningResultTab) => void;
  onSelectHistory: (runId: number) => void;
  onRequestDeleteHistory: (item: ScreeningHistoryItem) => void;
  onSelect: (symbol: string) => void;
  onAddToWatchlist: (stock: Stock) => void;
}

export const ScreeningResultList: React.FC<ScreeningResultListProps> = ({
  activeTab,
  results,
  history,
  generatedSql,
  selectedSymbol,
  selectedHistoryRunId,
  totalCount,
  isLoading,
  queryProgress,
  error,
  watchlistSymbols,
  deletingHistoryRunId,
  onTabChange,
  onSelectHistory,
  onRequestDeleteHistory,
  onSelect,
  onAddToWatchlist,
}) => {
  const { colors } = useTheme();
  const cc = useCandleColor();
  const progressPercent = Math.max(0, Math.min(100, Math.round(queryProgress?.progressPercent || 0)));
  const streamingSQL = queryProgress?.streamingText?.trim() || '';
  const isCanceled = queryProgress?.runStatus === 'canceled';
  const hasCompletedEmptyCurrentRun = activeTab === 'current' && results.length === 0 && generatedSql.trim().length > 0;
  const shouldShowProgressCard = isLoading || isCanceled;
  const orderedProgressSteps = buildOrderedQueryProgressSteps(queryProgress, streamingSQL);

  return (
    <div className="flex h-full flex-col">
      <div className="border-b fin-divider-soft px-4 pb-3 pt-7">
        <div className="flex justify-center">
          <div className={`inline-flex rounded-full border p-1 ${colors.isDark ? 'border-slate-700 bg-slate-900/40' : 'border-slate-200 bg-slate-100/70'}`}>
            <button
              type="button"
              onClick={() => onTabChange('current')}
              className={`rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                activeTab === 'current'
                  ? 'bg-accent text-white'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              当前筛选结果
            </button>
            <button
              type="button"
              onClick={() => onTabChange('history')}
              className={`rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                activeTab === 'history'
                  ? 'bg-accent text-white'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              历史筛选结果
            </button>
          </div>
        </div>

        <div className="mt-3 text-center">
          <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
            {activeTab === 'current' ? '筛选结果' : '历史筛选结果'}
          </div>
          <div className={`mt-1 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            {activeTab === 'current' ? `命中 ${totalCount} 条，当前显示 ${results.length} 条` : `历史记录 ${history.length} 条`}
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto fin-scrollbar">
        {activeTab === 'history' ? (
          history.length === 0 ? (
            <div className={`flex h-full items-center justify-center px-6 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              还没有历史筛选记录。
            </div>
          ) : (
            <div className="py-2">
              {history.map((item) => {
                const isActive = item.runId === selectedHistoryRunId;
                const isDeleting = item.runId === deletingHistoryRunId;
                return (
                  <div
                    key={item.runId}
                    className={`mx-3 mb-2 block w-[calc(100%-24px)] rounded-xl border px-3 py-3 text-left transition-colors ${
                      isActive
                        ? 'border-accent/40 bg-accent/10'
                        : `${colors.isDark ? 'border-slate-700 bg-slate-900/40 hover:border-slate-600 hover:bg-slate-800/40' : 'border-slate-200 bg-white/80 hover:border-slate-300 hover:bg-slate-50'}`
                    }`}
                  >
                    <div className="flex items-start gap-2">
                      <button
                        type="button"
                        onClick={() => onSelectHistory(item.runId)}
                        className="min-w-0 flex-1 text-left"
                      >
                        <div className="flex items-start gap-2">
                          <History className="mt-0.5 h-4 w-4 shrink-0 text-accent-2" />
                          <div className="min-w-0 flex-1">
                            <div className={`line-clamp-2 text-sm font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>{item.prompt}</div>
                            <div className={`mt-2 flex items-center justify-between text-[11px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                              <span>{formatDateTimeDisplay(item.createdAt, '--')}</span>
                              <span>{item.matchedCount} 条</span>
                            </div>
                          </div>
                        </div>
                      </button>
                      <button
                        type="button"
                        onClick={() => onRequestDeleteHistory(item)}
                        disabled={isDeleting}
                        className={`rounded-full p-1.5 transition-colors ${
                          colors.isDark
                            ? 'text-slate-500 hover:bg-slate-800 hover:text-red-300'
                            : 'text-slate-400 hover:bg-slate-100 hover:text-red-500'
                        } disabled:cursor-not-allowed disabled:opacity-50`}
                        aria-label={`删除历史记录 ${item.prompt}`}
                      >
                        {isDeleting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <X className="h-3.5 w-3.5" />}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )
        ) : shouldShowProgressCard ? (
          <div className="px-4 pt-6">
            <div className={`w-full rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-900/40' : 'border-slate-200 bg-white/90'}`}>
              <div className="flex items-center gap-2">
                {isCanceled
                  ? <CircleDashed className="h-4 w-4 text-slate-400" />
                  : <Loader2 className="h-4 w-4 animate-spin text-accent-2" />}
                <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                  {isCanceled ? '已取消筛选' : formatQueryStage(queryProgress?.currentStage)}
                </div>
                <div className={`ml-auto text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                  {isCanceled ? '已取消' : `${progressPercent}%`}
                </div>
              </div>
              <div className={`mt-2 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                {queryProgress?.message || (isCanceled ? '筛选已取消' : 'AI 正在分析筛选条件')}
              </div>
              <div className={`mt-3 h-2 overflow-hidden rounded-full ${colors.isDark ? 'bg-slate-800' : 'bg-slate-200'}`}>
                <div
                  className={`h-full rounded-full transition-all duration-300 ${
                    isCanceled
                      ? (colors.isDark ? 'bg-slate-600' : 'bg-slate-400')
                      : 'bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)]'
                  }`}
                  style={{ width: `${progressPercent}%` }}
                />
              </div>
              <div className="mt-4 space-y-3">
                {orderedProgressSteps.map((step) => (
                  <div
                    key={step.key}
                    className={`rounded-xl border px-3 py-3 ${
                      step.status === 'completed'
                        ? (colors.isDark ? 'border-emerald-500/20 bg-emerald-500/8' : 'border-emerald-200 bg-emerald-50/60')
                        : step.status === 'canceled'
                          ? (colors.isDark ? 'border-slate-700 bg-slate-950/70' : 'border-slate-300 bg-slate-50')
                        : step.status === 'active'
                          ? (colors.isDark ? 'border-accent/30 bg-slate-950/70' : 'border-accent/20 bg-slate-50')
                          : (colors.isDark ? 'border-slate-800 bg-slate-950/40' : 'border-slate-200 bg-slate-50/70')
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      {step.status === 'completed' ? (
                        <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                      ) : step.status === 'canceled' ? (
                        <CircleDashed className={colors.isDark ? 'h-4 w-4 text-slate-400' : 'h-4 w-4 text-slate-500'} />
                      ) : step.status === 'active' ? (
                        <Loader2 className="h-4 w-4 animate-spin text-accent-2" />
                      ) : (
                        <CircleDashed className={colors.isDark ? 'h-4 w-4 text-slate-500' : 'h-4 w-4 text-slate-400'} />
                      )}
                      <span className={`text-sm font-semibold ${
                        step.status === 'completed'
                          ? 'text-emerald-500'
                          : step.status === 'canceled'
                            ? (colors.isDark ? 'text-slate-200' : 'text-slate-700')
                          : colors.isDark ? 'text-slate-100' : 'text-slate-800'
                      }`}>
                        {step.label}
                      </span>
                      <span className={`ml-auto text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                        {step.time ? formatDateTimeDisplay(step.time, '--') : '--'}
                      </span>
                    </div>
                    <div className={`mt-2 text-sm ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
                      {step.message}
                    </div>
                    {(step.status === 'active' || step.status === 'canceled') && step.streamingText && (
                      <StreamingOutputBox
                        stepKey={step.key}
                        text={step.streamingText}
                        isDark={colors.isDark}
                      />
                    )}
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : error ? (
          <div className="p-4 text-sm text-red-400">{error}</div>
        ) : hasCompletedEmptyCurrentRun ? (
          <div className={`px-6 pt-8 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            没有符合条件的结果
          </div>
        ) : results.length === 0 ? (
          <div className={`px-6 pt-8 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            AI 筛选结果会显示在这里。你可以直接查看详情，或者加入自选。
          </div>
        ) : (
          results.map((result) => {
            const isSelected = result.symbol === selectedSymbol;
            const inWatchlist = watchlistSymbols.has(result.symbol);
            const stock = toStock(result);

            return (
              <div
                key={`${result.runId}-${result.symbol}-${result.rank}`}
                onClick={() => onSelect(result.symbol)}
                className={`group cursor-pointer border-b fin-divider-soft px-4 py-3 transition-colors ${
                  isSelected
                    ? `${colors.isDark ? 'bg-slate-800/40' : 'bg-slate-100/70'} border-l-4 border-l-accent`
                    : `${colors.isDark ? 'hover:bg-slate-800/30' : 'hover:bg-slate-100/50'} border-l-4 border-l-transparent`
                }`}
              >
                <div className="mb-2 flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="rounded-full bg-accent/10 px-2 py-0.5 text-[11px] font-semibold text-accent-2">#{result.rank}</span>
                      <span className={`truncate font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>{result.name}</span>
                    </div>
                    <div className={`mt-1 font-mono text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{result.symbol}</div>
                  </div>

                  <button
                    onClick={(event) => {
                      event.stopPropagation();
                      if (!inWatchlist) {
                        onAddToWatchlist(stock);
                      }
                    }}
                    className={`inline-flex items-center gap-1 rounded-lg border px-2 py-1 text-xs transition-colors ${
                      inWatchlist
                        ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                        : `${colors.isDark ? 'border-slate-700 bg-slate-900/40 text-slate-300 hover:border-accent/40 hover:text-accent-2' : 'border-slate-300 bg-white/80 text-slate-600 hover:border-accent/40 hover:text-accent-2'}`
                    }`}
                  >
                    {inWatchlist ? <Check className="h-3.5 w-3.5" /> : <Plus className="h-3.5 w-3.5" />}
                    {inWatchlist ? '已加自选' : '加入自选'}
                  </button>
                </div>

                <div className="grid grid-cols-3 gap-3 text-xs">
                  <div>
                    <div className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>价格</div>
                    <div className={`mt-1 font-mono ${cc.getColorClass(result.changePercent >= 0)}`}>{result.price.toFixed(2)}</div>
                  </div>
                  <div>
                    <div className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>涨跌幅</div>
                    <div className={`mt-1 font-mono ${cc.getColorClass(result.changePercent >= 0)}`}>
                      {result.changePercent >= 0 ? '+' : ''}{result.changePercent.toFixed(2)}%
                    </div>
                  </div>
                  <div>
                    <div className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>评分</div>
                    <div className={`mt-1 font-mono ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}>{result.score.toFixed(2)}</div>
                  </div>
                </div>

                <div className={`mt-3 flex items-center justify-between text-[11px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                  <span>成交额 {formatAmount(result.amount)}</span>
                  <span className="inline-flex items-center gap-1 text-accent-2">
                    查看详情
                    <ChevronRight className="h-3 w-3" />
                  </span>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};

type QueryStepStatus = 'completed' | 'active' | 'pending' | 'canceled';

const STREAMING_OUTPUT_BOTTOM_THRESHOLD = 24;

type OrderedQueryStep = {
  key: string;
  label: string;
  status: QueryStepStatus;
  message: string;
  time?: string;
  streamingText?: string;
};

const StreamingOutputBox: React.FC<{
  stepKey: string;
  text: string;
  isDark: boolean;
}> = ({ stepKey, text, isDark }) => {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const [autoFollow, setAutoFollow] = React.useState(true);

  React.useEffect(() => {
    setAutoFollow(true);
  }, [stepKey]);

  React.useEffect(() => {
    const element = containerRef.current;
    if (!element || !autoFollow) {
      return;
    }

    element.scrollTop = element.scrollHeight;
  }, [autoFollow, stepKey, text]);

  const handleScroll = React.useCallback(() => {
    const element = containerRef.current;
    if (!element) {
      return;
    }

    const distanceToBottom = element.scrollHeight - element.scrollTop - element.clientHeight;
    setAutoFollow(distanceToBottom <= STREAMING_OUTPUT_BOTTOM_THRESHOLD);
  }, []);

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      className={`mt-3 max-h-[320px] overflow-y-auto whitespace-pre-wrap break-words rounded-xl border px-3 py-3 text-base leading-7 fin-scrollbar ${
        isDark
          ? 'border-slate-700 bg-slate-950/70 text-slate-100'
          : 'border-slate-200 bg-white text-slate-700'
      }`}
    >
      {text}
    </div>
  );
};

const buildOrderedQueryProgressSteps = (
  queryProgress: ScreeningQueryProgress | null | undefined,
  streamingText: string,
): OrderedQueryStep[] => {
  const stepDefs = [
    { key: 'prepare', label: '准备请求' },
    { key: 'reasoning', label: '思考中' },
    { key: 'generate_sql', label: '生成 SQL' },
  ] as const;

  const latestLogs = new Map<string, { time?: string; message: string }>();
  for (const log of queryProgress?.logs || []) {
    latestLogs.set(log.stage, { time: log.time, message: log.message });
  }

  const stepIndex = resolveOrderedQueryStepIndex(queryProgress?.currentStage);
  const isCanceled = queryProgress?.runStatus === 'canceled';
  const activeIndex = stepIndex >= stepDefs.length ? -1 : stepIndex;
  const completedUntil = stepIndex >= stepDefs.length ? stepDefs.length - 1 : stepIndex - 1;

  return stepDefs.map((step, index) => {
    let status: QueryStepStatus = 'pending';
    if (index <= completedUntil) {
      status = 'completed';
    } else if (index === activeIndex && isCanceled) {
      status = 'canceled';
    } else if (index === activeIndex) {
      status = 'active';
    }

    const latestLog = latestLogs.get(step.key);
    const defaultMessage = step.key === 'prepare'
      ? '准备筛选请求'
      : step.key === 'reasoning'
        ? '正在实时分析筛选条件'
        : '正在生成筛选 SQL';

    return {
      key: step.key,
      label: step.label,
      status,
      message: status === 'active'
        ? (queryProgress?.message || latestLog?.message || defaultMessage)
        : status === 'canceled'
          ? (queryProgress?.message || '已取消筛选')
        : (latestLog?.message || defaultMessage),
      time: latestLog?.time,
      streamingText: (status === 'active' || status === 'canceled') && (step.key === 'reasoning' || step.key === 'generate_sql')
        ? streamingText
        : '',
    };
  });
};

const resolveOrderedQueryStepIndex = (stage?: string): number => {
  switch (stage) {
    case 'prepare':
      return 0;
    case 'reasoning':
      return 1;
    case 'generate_sql':
      return 2;
    case 'validate_sql':
    case 'execute_query':
    case 'store_results':
    case 'completed':
      return 3;
    default:
      return 0;
  }
};

const toStock = (result: ScreeningRunResult): Stock => ({
  symbol: result.symbol,
  name: result.name,
  price: result.price,
  change: 0,
  changePercent: result.changePercent,
  volume: result.volume,
  amount: result.amount,
  marketCap: '',
  sector: '',
  open: result.price,
  high: result.price,
  low: result.price,
  preClose: result.changePercent !== -100 ? result.price / (1 + result.changePercent / 100) : result.price,
});

const formatAmount = (amount: number): string => {
  if (amount >= 100000000) return `${(amount / 100000000).toFixed(2)}亿`;
  if (amount >= 10000) return `${(amount / 10000).toFixed(2)}万`;
  return amount.toFixed(0);
};

const formatQueryStage = (stage?: string): string => {
  switch (stage) {
    case 'prepare':
      return '准备请求';
    case 'reasoning':
      return '思考中';
    case 'generate_sql':
      return '生成 SQL';
    case 'validate_sql':
      return '校验 SQL';
    case 'execute_query':
      return '执行查询';
    case 'store_results':
      return '保存结果';
    case 'completed':
      return '筛选完成';
    default:
      return stage || 'AI 正在分析';
  }
};
