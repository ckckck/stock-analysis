import React, { useState, useEffect, useRef } from 'react';
import { Stock } from '../types';
import { searchStocks, StockSearchResult } from '../services/stockService';
import { Search, TrendingUp, X, Settings, Sparkles } from 'lucide-react';
import { WindowClose } from '../../wailsjs/go/main/App';
import { useTheme } from '../contexts/ThemeContext';
import logo from '../assets/images/logo.png';

interface WelcomePageProps {
  screeningPrompt: string;
  onScreeningPromptChange: (value: string) => void;
  onSubmitScreening: () => void;
  screeningSubmitting: boolean;
  onAddStock: (stock: Stock) => void;
  hasExistingWorkspace?: boolean;
  onOpenWatchlist?: () => void;
  onOpenScreening?: () => void;
  onOpenSettings?: () => void;
}

export const WelcomePage: React.FC<WelcomePageProps> = ({
  screeningPrompt,
  onScreeningPromptChange,
  onSubmitScreening,
  screeningSubmitting,
  onAddStock,
  hasExistingWorkspace = false,
  onOpenWatchlist,
  onOpenScreening,
  onOpenSettings,
}) => {
  const { colors } = useTheme();
  const [entryMode, setEntryMode] = useState<'screening' | 'watchlist'>('screening');
  const [stockSearchTerm, setStockSearchTerm] = useState('');
  const [stockSearchResults, setStockSearchResults] = useState<StockSearchResult[]>([]);
  const [showStockDropdown, setShowStockDropdown] = useState(false);
  const [isSearchingStock, setIsSearchingStock] = useState(false);
  const searchRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  const isScreeningMode = entryMode === 'screening';

  // 点击外部关闭下拉
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowStockDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // 搜索防抖
  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    if (entryMode !== 'watchlist' || !stockSearchTerm.trim()) {
      setStockSearchResults([]);
      setShowStockDropdown(false);
      setIsSearchingStock(false);
      return;
    }

    setIsSearchingStock(true);
    debounceRef.current = setTimeout(async () => {
      const results = await searchStocks(stockSearchTerm);
      const safeResults = Array.isArray(results) ? results : [];
      setStockSearchResults(safeResults);
      setShowStockDropdown(safeResults.length > 0);
      setIsSearchingStock(false);
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [entryMode, stockSearchTerm]);

  const handleSelectResult = (result: StockSearchResult) => {
    const newStock: Stock = {
      symbol: result.symbol,
      name: result.name,
      price: 0,
      change: 0,
      changePercent: 0,
      volume: 0,
      amount: 0,
      marketCap: '',
      sector: result.industry,
      open: 0,
      high: 0,
      low: 0,
      preClose: 0,
    };
    onAddStock(newStock);
    setStockSearchTerm('');
    setStockSearchResults([]);
    setShowStockDropdown(false);
  };

  const handleSubmitPrimary = () => {
    if (isScreeningMode) {
      if (screeningSubmitting) return;
      onSubmitScreening();
      return;
    }

    if (isSearchingStock) return;
    if (stockSearchResults.length === 1) {
      handleSelectResult(stockSearchResults[0]);
      return;
    }
    setShowStockDropdown(stockSearchResults.length > 0);
  };

  return (
    <div className="h-screen w-screen flex flex-col items-center justify-center fin-app relative" style={{ '--wails-draggable': 'drag' } as React.CSSProperties}>
      {/* 右上角按钮 */}
      <div className="absolute top-3 right-3 flex items-center gap-1" style={{ '--wails-draggable': 'no-drag', WebkitAppRegion: 'no-drag' } as React.CSSProperties}>
        {onOpenSettings && (
          <button
            onClick={onOpenSettings}
            className={`p-1.5 rounded transition-colors ${colors.isDark ? 'text-slate-400 hover:text-white hover:bg-slate-700/50' : 'text-slate-500 hover:text-white hover:bg-slate-400/50'}`}
            title="设置"
          >
            <Settings className="h-4 w-4" />
          </button>
        )}
        <button
          onClick={() => WindowClose()}
          className={`p-1.5 rounded hover:bg-red-500/80 transition-colors ${colors.isDark ? 'text-slate-400 hover:text-white' : 'text-slate-500 hover:text-white'}`}
          title="关闭"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Logo 和标题 */}
      <div className="flex items-center gap-3 mb-8">
        <img src={logo} alt="Logo" className="h-14 w-14 rounded-lg" />
        <div>
          <h1 className={`text-3xl font-bold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>
            散牛盘 <span className="text-accent-2">AI</span>
          </h1>
          <p className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>智能股票分析助手</p>
        </div>
      </div>

      <div className="w-[34rem] max-w-[calc(100vw-3rem)]" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
        <div className="mb-3 flex justify-center">
          <div className={`inline-flex rounded-full border p-1 ${colors.isDark ? 'border-slate-700 bg-slate-900/45' : 'border-slate-300 bg-white/85'}`}>
            <button
              type="button"
              onClick={() => {
                setEntryMode('screening');
                setShowStockDropdown(false);
              }}
              className={`rounded-full px-4 py-2 text-sm font-semibold transition-colors ${
                isScreeningMode
                  ? 'bg-accent text-white'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              AI 筛选
            </button>
            <button
              type="button"
              onClick={() => setEntryMode('watchlist')}
              className={`rounded-full px-4 py-2 text-sm font-semibold transition-colors ${
                !isScreeningMode
                  ? 'bg-accent text-white'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              自选
            </button>
          </div>
        </div>

        <div ref={searchRef} className="relative">
          {isScreeningMode ? (
            <Sparkles className={`absolute left-4 top-4 h-5 w-5 ${colors.isDark ? 'text-amber-300' : 'text-amber-500'}`} />
          ) : (
            <Search className={`absolute left-4 top-4 h-5 w-5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`} />
          )}
          <input
            type="text"
            value={isScreeningMode ? screeningPrompt : stockSearchTerm}
            onChange={(e) => {
              if (isScreeningMode) {
                onScreeningPromptChange(e.target.value);
              } else {
                setStockSearchTerm(e.target.value);
              }
            }}
            onFocus={() => {
              if (!isScreeningMode && stockSearchResults.length > 0) {
                setShowStockDropdown(true);
              }
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSubmitPrimary();
              }
            }}
            placeholder={
              isScreeningMode
                ? '描述你的筛选条件，例如：最近20日放量上涨、非ST、成交额大于5亿'
                : '搜索股票代码或名称，添加到自选'
            }
            className={`w-full rounded-2xl border pl-12 pr-32 py-4 text-base shadow-xl focus:outline-none focus:border-accent focus:ring-2 focus:ring-accent/40 transition-all ${
              colors.isDark
                ? 'bg-slate-800/85 border-slate-600 text-white placeholder-slate-500'
                : 'bg-white/95 border-slate-300 text-slate-800 placeholder-slate-400'
            }`}
            autoFocus
          />
          <button
            type="button"
            onClick={handleSubmitPrimary}
            disabled={isScreeningMode ? (!screeningPrompt.trim() || screeningSubmitting) : !stockSearchTerm.trim()}
            className="absolute right-3 top-2.5 inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)] px-4 py-2.5 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
          >
            {(screeningSubmitting || isSearchingStock) && (
              <span className="h-4 w-4 rounded-full border-2 border-white/80 border-t-transparent animate-spin" />
            )}
            <span>{isScreeningMode ? 'AI 筛选' : '搜索单只股票'}</span>
          </button>

          {!isScreeningMode && showStockDropdown && (
            <div className={`absolute left-0 right-0 top-[4.5rem] z-10 max-h-72 overflow-y-auto rounded-2xl border shadow-2xl ${
              colors.isDark
                ? 'border-slate-600 bg-slate-800'
                : 'border-slate-200 bg-white'
            }`}>
              {stockSearchResults.map((result) => (
                <div
                  key={result.symbol}
                  onClick={() => handleSelectResult(result)}
                  className={`px-4 py-3 cursor-pointer first:rounded-t-2xl last:rounded-b-2xl ${
                    colors.isDark
                      ? 'hover:bg-slate-700 border-b border-slate-700 last:border-b-0'
                      : 'hover:bg-slate-100 border-b border-slate-200 last:border-b-0'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <span className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{result.name}</span>
                      <span className="ml-2 font-mono text-sm text-accent-2">{result.symbol}</span>
                    </div>
                    <span className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{result.market}</span>
                  </div>
                  {result.industry && (
                    <div className={`mt-1 text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{result.industry}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        <div className={`mt-4 flex items-center justify-center gap-2 text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
          <TrendingUp className="h-4 w-4" />
          <span>{isScreeningMode ? '用自然语言描述筛选方式' : '搜索单只股票并加入自选'}</span>
        </div>

        {hasExistingWorkspace && (
          <div className="mt-5 flex items-center justify-center gap-3">
            <button
              type="button"
              onClick={onOpenWatchlist}
              className={`rounded-full border px-4 py-2 text-sm transition-colors ${
                colors.isDark
                  ? 'border-slate-600 bg-slate-800/70 text-slate-200 hover:text-white'
                  : 'border-slate-300 bg-white/85 text-slate-600 hover:text-slate-900'
              }`}
            >
              进入自选
            </button>
            <button
              type="button"
              onClick={onOpenScreening}
              className={`rounded-full border px-4 py-2 text-sm transition-colors ${
                colors.isDark
                  ? 'border-slate-600 bg-slate-800/70 text-slate-200 hover:text-white'
                  : 'border-slate-300 bg-white/85 text-slate-600 hover:text-slate-900'
              }`}
            >
              进入 AI 筛选
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
