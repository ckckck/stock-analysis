import React from 'react';
import { Check, ChevronRight, Plus, Sparkles } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { useCandleColor } from '../contexts/CandleColorContext';
import { ScreeningRunResult, Stock } from '../types';

interface ScreeningResultListProps {
  results: ScreeningRunResult[];
  selectedSymbol: string;
  totalCount: number;
  isLoading: boolean;
  error: string;
  watchlistSymbols: Set<string>;
  onSelect: (symbol: string) => void;
  onAddToWatchlist: (stock: Stock) => void;
}

export const ScreeningResultList: React.FC<ScreeningResultListProps> = ({
  results,
  selectedSymbol,
  totalCount,
  isLoading,
  error,
  watchlistSymbols,
  onSelect,
  onAddToWatchlist,
}) => {
  const { colors } = useTheme();
  const cc = useCandleColor();

  return (
    <div className="flex h-full flex-col">
      <div className="border-b fin-divider-soft px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="rounded-full bg-accent/15 p-2 text-accent-2">
            <Sparkles className="h-4 w-4" />
          </div>
          <div>
            <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>筛选结果</div>
            <div className={`text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>命中 {totalCount} 条，当前显示 {results.length} 条</div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto fin-scrollbar">
        {isLoading ? (
          <div className={`flex h-full items-center justify-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>AI 正在分析条件...</div>
        ) : error ? (
          <div className="p-4 text-sm text-red-400">{error}</div>
        ) : results.length === 0 ? (
          <div className={`flex h-full items-center justify-center px-6 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
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
