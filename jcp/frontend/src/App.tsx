import React, { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { StockList } from './components/StockList';
import { StockChartLW, type StockChartHandle } from './components/StockChartLW';
import { OrderBook as OrderBookComponent } from './components/OrderBook';
import { AgentRoom } from './components/AgentRoom';
import { SettingsDialog } from './components/SettingsDialog';
import { PositionDialog } from './components/PositionDialog';
import { HotTrendDialog } from './components/HotTrendDialog';
import { LongHuBangDialog } from './components/LongHuBangDialog';
import { ScreeningResultList } from './components/ScreeningResultList';
import { ScreeningWorkspace } from './components/ScreeningWorkspace';
import { WelcomePage } from './components/WelcomePage';
import { useToast } from './components/Toast';
import { ThemeSwitcher } from './components/ThemeSwitcher';
import { useTheme } from './contexts/ThemeContext';
import { useCandleColor } from './contexts/CandleColorContext';
import { ResizeHandle } from './components/ResizeHandle';
import { getWatchlist, addToWatchlist, removeFromWatchlist } from './services/watchlistService';
import { getKLineData, getOrderBook } from './services/stockService';
import { getOrCreateSession, StockSession, updateStockPosition } from './services/sessionService';
import { cancelScreeningSync, getConfig, getScreeningSyncStatus, getScreeningUniverseSymbols, onScreeningSyncProgress, runScreeningSync, updateConfig, type AppConfig as FrontendAppConfig } from './services/configService';
import { cancelScreeningQuery, deleteScreeningHistoryRun, getScreeningHistoryRun, listScreeningHistory, onScreeningQueryProgress, rerunScreeningHistoryRun, rerunScreeningHistoryRunWithUniverse, runScreeningQuery } from './services/screeningService';
import { useMarketEvents } from './hooks/useMarketEvents';
import { useMarketStatus } from './hooks/useMarketStatus';
import { formatDateDisplay, formatDateTimeDisplay } from './utils/datetime';
import {
  DEFAULT_TEXT_SCALE_PERCENT,
  clampTextScalePercent,
  getNextTextScalePercent,
  resolveTextScaleShortcutDirection,
} from './utils/textScale';
import {
  DEFAULT_KLINE_ZOOM_PERCENT,
  MAX_KLINE_ZOOM_PERCENT,
  MIN_KLINE_ZOOM_PERCENT,
  getNextKlineZoomPercent,
  isEditableTarget,
  resolveKlineKeyboardAction,
  shouldBlurActiveEditableOnPointerDown,
  shouldFocusKeyboardHostOnPointerDown,
} from './utils/klineKeyboard';
import {
  createPendingScreeningSyncStatus,
  formatScreeningSourceName,
  formatScreeningSyncRunStatus,
  mergeScreeningSyncProgress,
  resolveScreeningSyncCoverageStats,
  resolveScreeningPresetFromResult,
  resolveSyncDialogHeaderControls,
  resolveSyncOnlyMinimizedCardState,
  resolveSyncDialogCopy,
  resolveTopbarSyncButtonState,
  shouldContinueAfterScreeningSync,
  type ScreeningSyncDialogMode,
  type SyncButtonTone,
} from './utils/screeningSync';
import { canReuseHistorySql as shouldReuseHistorySql, canShowHistoryRerunLabel } from './utils/screeningHistoryReuse';
import { resolveHistorySelectionAfterDelete } from './utils/screeningHistoryDelete';
import { resolveStartupWindowRestore } from './utils/windowLayout';
import { debugAppEvent, errorAppEvent, infoAppEvent, warnAppEvent } from './utils/appLog';
import { shouldShowKlineRateLimitToast } from './utils/klineFallback';
import {
  AppScreenMode,
  Stock,
  KLineData,
  OrderBook,
  TimePeriod,
  Telegraph,
    MarketIndex,
    ScreeningConfig,
    ScreeningHistoryItem,
    ScreeningQueryResponse,
    ScreeningQueryProgress,
    ScreeningResultMode,
    ScreeningResultPreset,
    ScreeningResultTab,
    ScreeningRunSource,
    ScreeningRunResult,
    ScreeningSyncStatus,
  } from './types';
import { Radio, Settings, List, Minus, Square, X, Copy, Briefcase, Sparkles, Loader2, AlertCircle, CheckCircle2, Clock3 } from 'lucide-react';
import logo from './assets/images/logo.png';
import { GetTelegraphList, OpenURL, WindowMinimize, WindowMaximize, WindowClose } from '../wailsjs/go/main/App';
import { WindowIsMaximised, WindowSetSize, WindowGetSize } from '../wailsjs/runtime/runtime';

// 布局配置常量
const LAYOUT_DEFAULTS = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 180,
};
const LAYOUT_MIN = {
  leftPanelWidth: 280,
  rightPanelWidth: 384,
  bottomPanelHeight: 120,
};
const LAYOUT_MAX = {
  leftPanelWidth: 500,
  rightPanelWidth: 700,
  bottomPanelHeight: 450,
};

type KLineUpdateMode = 'full' | 'incremental' | 'refresh';

const SCREENING_SYNC_TERMINAL_STATUSES = new Set(['completed', 'failed', 'canceled']);
type SyncHealthTone = 'success' | 'warning' | 'danger';

const isScreeningSyncTerminal = (runStatus?: string): boolean => (
  Boolean(runStatus && SCREENING_SYNC_TERMINAL_STATUSES.has(runStatus))
);

const toErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
};

interface ScreeningHistoryReuseBaseline {
  prompt: string;
  marketScope: string;
  resultMode: ScreeningResultMode;
  resultLimit: number;
  usedTestScope: boolean;
  testLimit: number;
}

const resolveScreeningMarketScopeKey = (config: ScreeningConfig | null): string => {
  const values: string[] = [];
  if (config?.markets.shanghai) values.push('shanghai');
  if (config?.markets.shenzhen) values.push('shenzhen');
  if (config?.markets.beijing) values.push('beijing');
  if (config?.markets.indices) values.push('indices');
  return values.join(',');
};

const buildScreeningHistoryReuseBaseline = (response: ScreeningQueryResponse): ScreeningHistoryReuseBaseline => {
  const testLimit = response.universeSymbols?.length ?? 0;
  return {
    prompt: response.prompt?.trim() ?? '',
    marketScope: response.marketScope ?? '',
    resultMode: response.resultMode,
    resultLimit: response.resultLimit ?? 0,
    usedTestScope: testLimit > 0,
    testLimit,
  };
};

const App: React.FC = () => {
  const { colors } = useTheme();
  const cc = useCandleColor();
  const { showToast } = useToast();
  const [watchlist, setWatchlist] = useState<Stock[]>([]);
  const [selectedSymbol, setSelectedSymbol] = useState<string>('');
  const [currentSession, setCurrentSession] = useState<StockSession | null>(null);
  const [timePeriod, setTimePeriod] = useState<TimePeriod>('1m');
  const [kLineData, setKLineData] = useState<KLineData[]>([]);
  const [kLineUpdateMode, setKLineUpdateMode] = useState<KLineUpdateMode>('full');
  const [orderBook, setOrderBook] = useState<OrderBook>({ bids: [], asks: [] });
  const [marketMessage, setMarketMessage] = useState<string>('市场数据加载中...');
  const [telegraphList, setTelegraphList] = useState<Telegraph[]>([]);
  const [showTelegraphList, setShowTelegraphList] = useState(false);
  const [telegraphLoading, setTelegraphLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [showSettings, setShowSettings] = useState(false);
  const [showPosition, setShowPosition] = useState(false);
  const [showHotTrend, setShowHotTrend] = useState(false);
  const [showLongHuBang, setShowLongHuBang] = useState(false);
  const [marketIndices, setMarketIndices] = useState<MarketIndex[]>([]);
  const [isMaximized, setIsMaximized] = useState(false);
  const [screenMode, setScreenMode] = useState<AppScreenMode>('watchlist');
  const [screeningPrompt, setScreeningPrompt] = useState('');
  const [screeningPreset, setScreeningPreset] = useState<ScreeningResultPreset>('50');
  const [screeningResults, setScreeningResults] = useState<ScreeningRunResult[]>([]);
  const [screeningHistory, setScreeningHistory] = useState<ScreeningHistoryItem[]>([]);
  const [screeningResultTab, setScreeningResultTab] = useState<ScreeningResultTab>('current');
  const [screeningSelectedHistoryRunId, setScreeningSelectedHistoryRunId] = useState<number | null>(null);
  const [screeningDeleteTarget, setScreeningDeleteTarget] = useState<ScreeningHistoryItem | null>(null);
  const [screeningDeleteLoading, setScreeningDeleteLoading] = useState(false);
  const [screeningDeleteError, setScreeningDeleteError] = useState('');
  const [screeningRunSource, setScreeningRunSource] = useState<ScreeningRunSource>('ai');
  const [screeningRerunBaseRunId, setScreeningRerunBaseRunId] = useState<number | null>(null);
  const [screeningHistoryReuseBaseline, setScreeningHistoryReuseBaseline] = useState<ScreeningHistoryReuseBaseline | null>(null);
  const [screeningGeneratedSql, setScreeningGeneratedSql] = useState('');
  const [screeningSqlViewerOpen, setScreeningSqlViewerOpen] = useState(false);
  const [screeningTotalCount, setScreeningTotalCount] = useState(0);
  const [screeningLoading, setScreeningLoading] = useState(false);
  const [screeningError, setScreeningError] = useState('');
  const [screeningConfig, setScreeningConfig] = useState<ScreeningConfig | null>(null);
  const [screeningSyncStatus, setScreeningSyncStatus] = useState<ScreeningSyncStatus | null>(null);
  const [screeningConfirmVisible, setScreeningConfirmVisible] = useState(false);
  const [screeningConfirmSyncStarted, setScreeningConfirmSyncStarted] = useState(false);
  const [pendingScreeningPrompt, setPendingScreeningPrompt] = useState('');
  const [welcomeSyncLoading, setWelcomeSyncLoading] = useState(false);
  const [welcomeSyncError, setWelcomeSyncError] = useState('');
  const [syncOnlyDialogVisible, setSyncOnlyDialogVisible] = useState(false);
  const [syncOnlySyncStarted, setSyncOnlySyncStarted] = useState(false);
  const [syncOnlyDialogMinimized, setSyncOnlyDialogMinimized] = useState(false);
  const [syncOnlyLoading, setSyncOnlyLoading] = useState(false);
  const [syncOnlyError, setSyncOnlyError] = useState('');
  const [welcomeSyncTestMode, setWelcomeSyncTestMode] = useState(false);
  const [welcomeSyncTestLimit, setWelcomeSyncTestLimit] = useState(10);
  const [screeningQueryProgress, setScreeningQueryProgress] = useState<ScreeningQueryProgress | null>(null);
  const [screeningSQLTimeoutLabel, setScreeningSQLTimeoutLabel] = useState('不限');
  const [settingsInitialTab, setSettingsInitialTab] = useState<'provider' | 'screening'>('provider');
  const klineRequestIdRef = useRef(0);
  const klineHistoryAvailabilityRef = useRef<Map<string, boolean>>(new Map());
  const screeningCancelRequestedRef = useRef(false);

  // 使用纯前端市场状态判断
  const { status: marketStatus } = useMarketStatus();

  // 布局状态
  const [leftPanelWidth, setLeftPanelWidth] = useState(LAYOUT_DEFAULTS.leftPanelWidth);
  const [rightPanelWidth, setRightPanelWidth] = useState(LAYOUT_DEFAULTS.rightPanelWidth);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(LAYOUT_DEFAULTS.bottomPanelHeight);
  const [textScalePercent, setTextScalePercent] = useState(DEFAULT_TEXT_SCALE_PERCENT);
  const [klineZoomPercent, setKlineZoomPercent] = useState(DEFAULT_KLINE_ZOOM_PERCENT);
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stockChartRef = useRef<StockChartHandle | null>(null);
  const keyboardHostRef = useRef<HTMLDivElement | null>(null);
  const editingModeRef = useRef(false);

  const focusKeyboardHost = useCallback((reason: string) => {
    keyboardHostRef.current?.focus({ preventScroll: true });
    debugAppEvent('kline-shortcut', 'keyboard host focused', {
      reason,
      activeTagName: document.activeElement instanceof HTMLElement ? document.activeElement.tagName : null,
    });
  }, []);

  const screeningStocks = useMemo(
    () => screeningResults.map(mapScreeningResultToStock),
    [screeningResults],
  );

  const stockLookup = useMemo(() => {
    const entries = [...watchlist, ...screeningStocks].map((stock) => [stock.symbol, stock] as const);
    return new Map(entries);
  }, [watchlist, screeningStocks]);

  const screeningStockLookup = useMemo(
    () => new Map(screeningStocks.map((stock) => [stock.symbol, stock] as const)),
    [screeningStocks],
  );

  const selectedStock = useMemo(() => {
    if (screenMode === 'screening') {
      if (selectedSymbol) {
        const matched = screeningStockLookup.get(selectedSymbol);
        if (matched) return matched;
      }
      return screeningStocks[0];
    }
    if (selectedSymbol) {
      const matched = stockLookup.get(selectedSymbol);
      if (matched) return matched;
    }
    return watchlist[0] ?? screeningStocks[0];
  }, [screenMode, screeningStockLookup, screeningStocks, selectedSymbol, stockLookup, watchlist]);

  const watchlistSymbols = useMemo(() => new Set(watchlist.map(stock => stock.symbol)), [watchlist]);

  const screeningSummary = useMemo(() => {
    const markets = screeningConfig?.markets;
    const marketScopeLabel = [
      markets?.shanghai ? '沪市' : null,
      markets?.shenzhen ? '深市' : null,
      markets?.beijing ? '北交所' : null,
      markets?.indices ? '指数' : null,
    ].filter(Boolean).join('、') || '未配置';

    const initialSyncLabel = `最近 ${screeningConfig?.initialSyncDays ?? 30} 天`;
    const retentionLabel = screeningConfig?.retentionMode === 'days'
      ? `仅保留近 ${screeningConfig.retentionDays} 天`
      : '永久保留';
    const autoSyncLabel = screeningConfig?.autoSyncEnabled
      ? `开启，${screeningConfig.autoSyncTime}`
      : '未开启';
    const targetTradeDateLabel = formatDateDisplay(screeningSyncStatus?.targetTradeDate, '未同步');
    const latestSyncedTradeDateLabel = formatDateDisplay(screeningSyncStatus?.latestSyncedTradeDate, '未同步');
    const lastSyncedAtLabel = formatDateTimeDisplay(screeningSyncStatus?.lastSyncedAt, '未同步');
    const coverageStats = resolveScreeningSyncCoverageStats(screeningSyncStatus);
    const syncedToLatestLabel = `${coverageStats.syncedToLatestStocks} 只`;
    const pendingSyncLabel = `${coverageStats.pendingSyncStocks} 只`;
    const marketStockCountLabel = `${coverageStats.marketStockCount} 只`;
    const healthSummary = resolveScreeningSyncHealthSummary(screeningSyncStatus);

    return {
      strategyItems: [
        { label: '市场范围', value: marketScopeLabel },
        { label: '首次同步', value: initialSyncLabel },
        { label: '保留策略', value: retentionLabel },
        { label: '自动同步', value: autoSyncLabel },
        { label: 'SQL 超时', value: screeningSQLTimeoutLabel },
      ],
      dataItems: [
        { label: '最近交易日', value: targetTradeDateLabel },
        { label: '最新已同步交易日', value: latestSyncedTradeDateLabel },
        { label: '最近同步时间', value: lastSyncedAtLabel },
        { label: '已同步到最新', value: syncedToLatestLabel, tone: 'success' as SyncHealthTone },
        { label: '待同步', value: pendingSyncLabel, tone: healthSummary.tone === 'success' ? undefined : healthSummary.tone },
        { label: '股票总数', value: marketStockCountLabel },
      ],
      healthSummary,
    };
  }, [screeningConfig, screeningSQLTimeoutLabel, screeningSyncStatus]);

  const topbarSyncButtonState = useMemo(
    () => resolveTopbarSyncButtonState(screeningSyncStatus),
    [screeningSyncStatus],
  );
  const syncOnlyMinimizedCardState = useMemo(
    () => resolveSyncOnlyMinimizedCardState({
      visible: syncOnlyDialogVisible,
      minimized: syncOnlyDialogMinimized,
      loading: syncOnlyLoading,
      syncStatus: screeningSyncStatus,
    }),
    [screeningSyncStatus, syncOnlyDialogMinimized, syncOnlyDialogVisible, syncOnlyLoading],
  );

  const currentScreeningResultMode = useMemo<ScreeningResultMode>(
    () => screeningPreset === 'unlimited' ? 'unlimited' : 'top_n',
    [screeningPreset],
  );

  const currentScreeningResultLimit = useMemo(
    () => currentScreeningResultMode === 'unlimited' ? 0 : Number(screeningPreset),
    [currentScreeningResultMode, screeningPreset],
  );

  const currentScreeningMarketScope = useMemo(
    () => resolveScreeningMarketScopeKey(screeningConfig),
    [screeningConfig],
  );

  const canReuseHistorySql = useMemo(
    () => (
      screeningRunSource === 'history_sql'
      && screeningRerunBaseRunId !== null
      && screeningHistoryReuseBaseline !== null
      && shouldReuseHistorySql({
        currentPrompt: screeningPrompt,
        currentMarketScope: currentScreeningMarketScope,
        currentResultMode: currentScreeningResultMode,
        currentResultLimit: currentScreeningResultLimit,
        currentTestMode: welcomeSyncTestMode,
        currentTestLimit: welcomeSyncTestMode ? welcomeSyncTestLimit : 0,
        historyPrompt: screeningHistoryReuseBaseline.prompt,
        historyMarketScope: screeningHistoryReuseBaseline.marketScope,
        historyResultMode: screeningHistoryReuseBaseline.resultMode,
        historyResultLimit: screeningHistoryReuseBaseline.resultLimit,
        historyUsedTestScope: screeningHistoryReuseBaseline.usedTestScope,
        historyTestLimit: screeningHistoryReuseBaseline.testLimit,
      })
    ),
    [
      currentScreeningMarketScope,
      currentScreeningResultLimit,
      currentScreeningResultMode,
      screeningHistoryReuseBaseline,
      screeningPrompt,
      screeningRerunBaseRunId,
      screeningRunSource,
      welcomeSyncTestLimit,
      welcomeSyncTestMode,
    ],
  );

  const shouldShowHistoryRerunLabel = useMemo(
    () => (
      screeningRunSource === 'history_sql'
      && screeningRerunBaseRunId !== null
      && screeningHistoryReuseBaseline !== null
      && canShowHistoryRerunLabel({
        currentPrompt: screeningPrompt,
        historyPrompt: screeningHistoryReuseBaseline.prompt,
      })
    ),
    [
      screeningHistoryReuseBaseline,
      screeningPrompt,
      screeningRerunBaseRunId,
      screeningRunSource,
    ],
  );

  const refreshScreeningBootstrapState = useCallback(async () => {
    try {
      const [config, status] = await Promise.all([
        getConfig(),
        getScreeningSyncStatus().catch(() => null),
      ]);
      console.debug('[screening-sync] bootstrap refresh', {
        runStatus: status?.runStatus,
        completedStocks: status?.completedStocks,
        totalStocks: status?.totalStocks,
        syncedToLatestStocks: status?.syncedToLatestStocks,
        marketStockCount: status?.marketStockCount,
        currentStage: status?.currentStage,
        lastMessage: status?.lastMessage,
        error: status?.error,
      });
      setScreeningConfig((config.screening as ScreeningConfig) ?? null);
      setScreeningSQLTimeoutLabel(resolveDefaultScreeningSQLTimeoutLabel(config));
      setScreeningSyncStatus(status);
    } catch (error) {
      console.error('Failed to refresh screening bootstrap state:', error);
    }
  }, []);

  useEffect(() => {
    const unsubscribe = onScreeningQueryProgress((progress) => {
      if (screeningCancelRequestedRef.current) {
        return;
      }
      setScreeningQueryProgress(progress);
    });
    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    const unsubscribe = onScreeningSyncProgress((progress) => {
      console.debug('[screening-sync] progress', {
        runStatus: progress.runStatus,
        currentStage: progress.currentStage,
        completedStocks: progress.completedStocks,
        totalStocks: progress.totalStocks,
        currentSymbol: progress.currentSymbol,
        lastMessage: progress.lastMessage,
        error: progress.error,
      });
      setScreeningSyncStatus(prev => mergeScreeningSyncProgress(prev, progress));

      if (isScreeningSyncTerminal(progress.runStatus)) {
        void getScreeningSyncStatus()
          .then((status) => {
            setScreeningSyncStatus(prev => ({
              ...mergeScreeningSyncProgress(prev, progress),
              ...status,
              events: (status.events && status.events.length > 0) ? status.events : progress.events,
            }));
          })
          .catch(() => undefined);
      }
    });

    return () => {
      unsubscribe();
    };
  }, []);

  const resetScreeningSyncRunState = useCallback(() => {
    setScreeningSyncStatus(prev => {
      if (!prev || prev.runStatus === 'running') return prev;
      return {
        ...prev,
        runStatus: undefined,
        progressPercent: undefined,
        totalStocks: undefined,
        completedStocks: undefined,
        currentSymbol: undefined,
        currentName: undefined,
        currentStage: undefined,
        activeSource: undefined,
        lastMessage: undefined,
        limitStocks: undefined,
        resumeFromCheckpoint: undefined,
        events: [],
        error: undefined,
      };
    });
  }, []);

  const openScreeningConfirm = useCallback((promptText: string) => {
    const normalizedPrompt = promptText.trim();
    if (!normalizedPrompt) return;
    setWelcomeSyncError('');
    resetScreeningSyncRunState();
    setPendingScreeningPrompt(normalizedPrompt);
    setScreeningConfirmSyncStarted(false);
    setScreeningConfirmVisible(true);
  }, [resetScreeningSyncRunState]);

  const closeScreeningConfirm = useCallback(() => {
    if (welcomeSyncLoading) return;
    setScreeningConfirmSyncStarted(false);
    setScreeningConfirmVisible(false);
  }, [welcomeSyncLoading]);

  const handleOpenSyncOnlyDialog = useCallback(() => {
    if (topbarSyncButtonState.disabled) return;
    if (syncOnlyDialogVisible) {
      setSyncOnlyDialogMinimized(false);
      return;
    }
    setSyncOnlyError('');
    resetScreeningSyncRunState();
    setSyncOnlySyncStarted(false);
    setSyncOnlyDialogMinimized(false);
    setSyncOnlyDialogVisible(true);
  }, [resetScreeningSyncRunState, syncOnlyDialogVisible, topbarSyncButtonState.disabled]);

  const closeSyncOnlyDialog = useCallback(() => {
    if (syncOnlyLoading) return;
    setSyncOnlySyncStarted(false);
    setSyncOnlyDialogMinimized(false);
    setSyncOnlyDialogVisible(false);
  }, [syncOnlyLoading]);

  const handleMinimizeSyncOnlyDialog = useCallback(() => {
    setSyncOnlyDialogMinimized(true);
  }, []);

  const handleRestoreSyncOnlyDialog = useCallback(() => {
    setSyncOnlyDialogMinimized(false);
  }, []);

  // 处理股票数据更新（来自后端推送）
  const handleStockUpdate = useCallback((stocks: Stock[]) => {
    if (!stocks || !Array.isArray(stocks)) return;
    setWatchlist(prev => {
      // 更新已有股票的数据
      return prev.map(stock => {
        const updated = stocks.find(s => s.symbol === stock.symbol);
        return updated || stock;
      });
    });
  }, []);

  // 处理盘口数据更新（来自后端推送）
  const handleOrderBookUpdate = useCallback((data: OrderBook) => {
    setOrderBook(data);
  }, []);

  // 处理快讯数据更新（来自后端推送）
  const handleTelegraphUpdate = useCallback((data: Telegraph) => {
    if (data && data.content) {
      setMarketMessage(`[${data.time}] ${data.content}`);
    }
  }, []);

  // 处理大盘指数更新（来自后端推送）
  const handleMarketIndicesUpdate = useCallback((indices: MarketIndex[]) => {
    if (indices) {
      setMarketIndices(indices);
    }
  }, []);

  // 处理K线数据更新（来自后端推送，支持增量）
  const handleKLineUpdate = useCallback((data: { code: string; period: string; data: KLineData[]; incremental?: boolean }) => {
    if (!data || data.code !== selectedSymbol || data.period !== timePeriod) return;

    if (data.incremental && data.data.length > 0) {
      setKLineUpdateMode('incremental');
      // 增量更新：合并最新K线
      setKLineData(prev => {
        if (prev.length === 0) return data.data;
        const newBar = data.data[0];
        const lastIdx = prev.length - 1;
        // 同一时间戳则更新，否则追加
        if (prev[lastIdx].time === newBar.time) {
          const updated = [...prev];
          updated[lastIdx] = newBar;
          return updated;
        }
        return [...prev.slice(-239), newBar]; // 保持240根
      });
    } else {
      // 后端定时推送：用 refresh 模式更新数据但保留用户缩放状态
      if (Array.isArray(data.data) && data.data.length > 0) {
        setKLineUpdateMode('refresh');
        setKLineData(data.data);
      }
    }
  }, [selectedSymbol, timePeriod]);

  const syncWindowMaximizedState = useCallback(async () => {
    try {
      const maximized = await WindowIsMaximised();
      setIsMaximized(maximized);
    } catch {
      // ignore runtime query failures and keep current UI state
    }
  }, []);

  const toggleWindowMaximize = useCallback(async () => {
    try {
      await WindowMaximize();
      await syncWindowMaximizedState();
    } catch {
      // fallback to optimistic toggle when runtime query is unavailable
      setIsMaximized(prev => !prev);
    }
  }, [syncWindowMaximizedState]);

  // 保存布局配置（防抖）
  const saveLayoutConfig = useCallback(async (
    left: number, right: number, bottom: number,
    textScale: number,
    klineZoom: number,
    winWidth?: number, winHeight?: number
  ) => {
    if (saveTimeoutRef.current) {
      clearTimeout(saveTimeoutRef.current);
    }
    saveTimeoutRef.current = setTimeout(async () => {
      try {
        const config = await getConfig();
        const size = await WindowGetSize();
        config.layout = {
          leftPanelWidth: left,
          rightPanelWidth: right,
          bottomPanelHeight: bottom,
          textScalePercent: clampTextScalePercent(textScale),
          klineZoomPercent: Math.max(MIN_KLINE_ZOOM_PERCENT, Math.min(MAX_KLINE_ZOOM_PERCENT, Math.round(klineZoom))),
          windowWidth: winWidth ?? size.w,
          windowHeight: winHeight ?? size.h,
        };
        await updateConfig(config);
      } catch (err) {
        console.error('Failed to save layout config:', err);
      }
    }, 500);
  }, []);

  // 左侧面板 resize
  const handleLeftResize = useCallback((delta: number) => {
    setLeftPanelWidth(prev => {
      const newWidth = Math.max(LAYOUT_MIN.leftPanelWidth, Math.min(LAYOUT_MAX.leftPanelWidth, prev + delta));
      return newWidth;
    });
  }, []);

  // 右侧面板 resize
  const handleRightResize = useCallback((delta: number) => {
    setRightPanelWidth(prev => {
      const newWidth = Math.max(LAYOUT_MIN.rightPanelWidth, Math.min(LAYOUT_MAX.rightPanelWidth, prev - delta));
      return newWidth;
    });
  }, []);

  // 底部面板 resize
  const handleBottomResize = useCallback((delta: number) => {
    setBottomPanelHeight(prev => {
      const newHeight = Math.max(LAYOUT_MIN.bottomPanelHeight, Math.min(LAYOUT_MAX.bottomPanelHeight, prev - delta));
      return newHeight;
    });
  }, []);

  // resize 结束时保存配置
  const handleResizeEnd = useCallback(() => {
    saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight, textScalePercent, klineZoomPercent);
  }, [bottomPanelHeight, klineZoomPercent, leftPanelWidth, rightPanelWidth, saveLayoutConfig, textScalePercent]);

  // 监听窗口 resize 事件
  useEffect(() => {
    const windowResizeTimeoutRef = { current: null as ReturnType<typeof setTimeout> | null };
    const handleWindowResize = () => {
      if (windowResizeTimeoutRef.current) {
        clearTimeout(windowResizeTimeoutRef.current);
      }
      windowResizeTimeoutRef.current = setTimeout(() => {
        saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight, textScalePercent, klineZoomPercent);
      }, 500);
    };
    window.addEventListener('resize', handleWindowResize);
    return () => {
      window.removeEventListener('resize', handleWindowResize);
      if (windowResizeTimeoutRef.current) {
        clearTimeout(windowResizeTimeoutRef.current);
      }
    };
  }, [bottomPanelHeight, klineZoomPercent, leftPanelWidth, rightPanelWidth, saveLayoutConfig, textScalePercent]);

  useEffect(() => {
    document.documentElement.style.fontSize = `${clampTextScalePercent(textScalePercent)}%`;
  }, [textScalePercent]);

  useEffect(() => {
    focusKeyboardHost('mount');
  }, [focusKeyboardHost]);

  useEffect(() => {
    const handleFocusIn = (event: FocusEvent) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      editingModeRef.current = isEditableTarget(target);
      debugAppEvent('kline-shortcut', 'editing mode updated by focusin', {
        editingMode: editingModeRef.current,
        tagName: target?.tagName ?? null,
      });
    };

    const handlePointerDown = (event: PointerEvent) => {
      const activeTarget = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      const pointerTarget = event.target instanceof HTMLElement ? event.target : null;
      if (!shouldBlurActiveEditableOnPointerDown({ activeTarget, pointerTarget })) {
        if (shouldFocusKeyboardHostOnPointerDown(pointerTarget)) {
          editingModeRef.current = false;
          window.requestAnimationFrame(() => {
            focusKeyboardHost('pointerdown-noneditable');
          });
        }
        return;
      }
      activeTarget?.blur();
      debugAppEvent('kline-shortcut', 'editable focus blurred by pointerdown', {
        activeTagName: activeTarget?.tagName ?? null,
        pointerTagName: pointerTarget?.tagName ?? null,
      });
      if (shouldFocusKeyboardHostOnPointerDown(pointerTarget)) {
        editingModeRef.current = false;
        window.requestAnimationFrame(() => {
          focusKeyboardHost('pointerdown-after-blur');
        });
      }
    };

    window.addEventListener('focusin', handleFocusIn, true);
    window.addEventListener('pointerdown', handlePointerDown, true);
    return () => {
      window.removeEventListener('focusin', handleFocusIn, true);
      window.removeEventListener('pointerdown', handlePointerDown, true);
    };
  }, [focusKeyboardHost]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key.startsWith('Arrow')) {
        const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        debugAppEvent('kline-shortcut', 'arrow keydown captured', {
          key: event.key,
          targetTagName: event.target instanceof HTMLElement ? event.target.tagName : null,
          activeTagName: activeElement?.tagName ?? null,
          activeRole: activeElement?.getAttribute('role') ?? null,
          activeDataset: activeElement?.dataset ?? null,
          editingMode: editingModeRef.current,
          metaKey: event.metaKey,
          ctrlKey: event.ctrlKey,
          altKey: event.altKey,
          shiftKey: event.shiftKey,
        });
      }

      const textScaleDirection = resolveTextScaleShortcutDirection(event);
      if (textScaleDirection) {
        event.preventDefault();

        setTextScalePercent((current) => {
          const next = getNextTextScalePercent(current, textScaleDirection);
          if (next !== current) {
            saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight, next, klineZoomPercent);
          }
          return next;
        });
        return;
      }

      const klineAction = resolveKlineKeyboardAction({
        key: event.key,
        metaKey: event.metaKey,
        ctrlKey: event.ctrlKey,
        altKey: event.altKey,
        shiftKey: event.shiftKey,
        editingMode: editingModeRef.current,
        target: document.activeElement instanceof HTMLElement ? document.activeElement : (
          event.target instanceof HTMLElement ? event.target : null
        ),
      });
      if (!klineAction) {
        if (event.key.startsWith('Arrow')) {
          const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
          debugAppEvent('kline-shortcut', 'arrow key ignored', {
            key: event.key,
            tagName: event.target instanceof HTMLElement ? event.target.tagName : null,
            isContentEditable: event.target instanceof HTMLElement ? event.target.isContentEditable : null,
            activeTagName: activeElement?.tagName ?? null,
            editingMode: editingModeRef.current,
            activeIsEditable: isEditableTarget(activeElement),
          });
        }
        return;
      }

      event.preventDefault();
      event.stopPropagation();

      if (klineAction === 'pan_left') {
        debugAppEvent('kline-shortcut', 'pan left requested');
        stockChartRef.current?.panLeft();
        return;
      }

      if (klineAction === 'pan_right') {
        debugAppEvent('kline-shortcut', 'pan right requested');
        stockChartRef.current?.panRight();
        return;
      }

      setKlineZoomPercent((current) => {
        const next = stockChartRef.current?.getNextEffectiveZoomPercent(klineAction, current ?? DEFAULT_KLINE_ZOOM_PERCENT)
          ?? getNextKlineZoomPercent(current, klineAction);
        debugAppEvent('kline-shortcut', 'zoom keydown', {
          action: klineAction,
          current,
          next,
          min: MIN_KLINE_ZOOM_PERCENT,
          max: MAX_KLINE_ZOOM_PERCENT,
          hitBoundary: next === current,
        });
        if (next !== current) {
          saveLayoutConfig(leftPanelWidth, rightPanelWidth, bottomPanelHeight, textScalePercent, next);
        } else {
          debugAppEvent('kline-shortcut', 'zoom percent unchanged after clamp', {
            action: klineAction,
            current,
            min: MIN_KLINE_ZOOM_PERCENT,
            max: MAX_KLINE_ZOOM_PERCENT,
          });
        }
        return next;
      });
    };

    window.addEventListener('keydown', handleKeyDown, true);
    return () => {
      window.removeEventListener('keydown', handleKeyDown, true);
    };
  }, [bottomPanelHeight, klineZoomPercent, leftPanelWidth, rightPanelWidth, saveLayoutConfig, textScalePercent]);

  // 获取快讯列表
  const handleShowTelegraphList = async () => {
    if (!showTelegraphList) {
      setShowTelegraphList(true);
      setTelegraphLoading(true);
      try {
        const list = await GetTelegraphList();
        setTelegraphList(list || []);
      } finally {
        setTelegraphLoading(false);
      }
    } else {
      setShowTelegraphList(false);
    }
  };

  // 打开快讯链接
  const handleOpenTelegraph = (telegraph: Telegraph) => {
    if (telegraph.url) {
      OpenURL(telegraph.url);
    }
    setShowTelegraphList(false);
  };

  // 使用市场事件 Hook
  const { subscribeOrderBook, subscribeKLine } = useMarketEvents({
    onStockUpdate: handleStockUpdate,
    onOrderBookUpdate: handleOrderBookUpdate,
    onTelegraphUpdate: handleTelegraphUpdate,
    onMarketIndicesUpdate: handleMarketIndicesUpdate,
    onKLineUpdate: handleKLineUpdate,
  });

  // Handle Adding Stock
  const handleAddStock = async (newStock: Stock) => {
    if (!watchlist.find(s => s.symbol === newStock.symbol)) {
      await addToWatchlist(newStock);
      setWatchlist(prev => [...prev, newStock]);
      setScreenMode('watchlist');
      // 添加后自动选中新股票并加载数据
      setSelectedSymbol(newStock.symbol);
      // 先清空 session，避免显示旧股票的消息
      setCurrentSession(null);
      subscribeOrderBook(newStock.symbol);
      // 加载 Session 和盘口数据
      const [session, orderBookData] = await Promise.all([
        getOrCreateSession(newStock.symbol, newStock.name),
        getOrderBook(newStock.symbol)
      ]);
      setCurrentSession(session);
      setOrderBook(orderBookData);
    }
  };

  // Handle Removing Stock
  const handleRemoveStock = async (symbol: string) => {
    await removeFromWatchlist(symbol);
    setWatchlist(prev => prev.filter(s => s.symbol !== symbol));
    // 如果删除的是当前选中的股票，切换到第一个
    if (symbol === selectedSymbol) {
      const remaining = watchlist.filter(s => s.symbol !== symbol);
      if (remaining.length > 0) {
        handleSelectStock(remaining[0].symbol);
      }
    }
  };

  const loadSelectedStockContext = useCallback(async (symbol: string, stockName?: string) => {
    setSelectedSymbol(symbol);
    subscribeOrderBook(symbol);

    const resolvedName = stockName || stockLookup.get(symbol)?.name;
    if (!resolvedName) {
      return;
    }

    const [session, orderBookData] = await Promise.all([
      getOrCreateSession(symbol, resolvedName),
      getOrderBook(symbol),
    ]);
    setCurrentSession(session);
    setOrderBook(orderBookData);
  }, [stockLookup, subscribeOrderBook]);

  // Handle Stock Selection - Load Session and sync data
  const handleSelectStock = async (symbol: string) => {
    await loadSelectedStockContext(symbol);
  };

  const applyScreeningResponse = useCallback((
    response: ScreeningQueryResponse,
    options?: {
      source?: ScreeningRunSource;
      historyRunId?: number | null;
      rerunBaseRunId?: number | null;
    },
  ) => {
    setScreeningResults(response.results ?? []);
    setScreeningTotalCount(response.totalCount ?? 0);
    setScreeningError('');
    setScreeningResultTab('current');
    setScreeningRunSource(options?.source ?? 'ai');
    setScreeningSelectedHistoryRunId(options?.historyRunId ?? null);
    setScreeningRerunBaseRunId(options?.rerunBaseRunId ?? null);
    setScreeningHistoryReuseBaseline((options?.source ?? 'ai') === 'history_sql' ? buildScreeningHistoryReuseBaseline(response) : null);
    setScreeningGeneratedSql(response.generatedSql ?? '');
    if (response.prompt) {
      setScreeningPrompt(response.prompt);
    }
    setScreeningPreset(prev => resolveScreeningPresetFromResult({
      resultMode: response.resultMode,
      resultLimit: response.resultLimit,
      fallbackPreset: prev,
    }));
    if (response.results && response.results.length > 0) {
      void loadSelectedStockContext(response.results[0].symbol, response.results[0].name);
    }
  }, [loadSelectedStockContext]);

  const refreshScreeningHistory = useCallback(async () => {
    try {
      const items = await listScreeningHistory(20);
      setScreeningHistory(items);
      if (watchlist.length === 0 && items.length > 0) {
        setScreenMode('screening');
      }
    } catch (error) {
      console.error('Failed to load screening history:', error);
    }
  }, [watchlist.length]);

  const handleRequestDeleteScreeningHistory = useCallback((item: ScreeningHistoryItem) => {
    if (screeningDeleteLoading) return;
    setScreeningDeleteError('');
    setScreeningDeleteTarget(item);
  }, [screeningDeleteLoading]);

  const handleCloseDeleteScreeningHistory = useCallback(() => {
    if (screeningDeleteLoading) return;
    setScreeningDeleteError('');
    setScreeningDeleteTarget(null);
  }, [screeningDeleteLoading]);

  const handleConfirmDeleteScreeningHistory = useCallback(async () => {
    if (!screeningDeleteTarget) return;

    setScreeningDeleteLoading(true);
    setScreeningDeleteError('');
    try {
      await deleteScreeningHistoryRun(screeningDeleteTarget.runId);
      setScreeningHistory(prev => prev.filter(item => item.runId !== screeningDeleteTarget.runId));
      setScreeningSelectedHistoryRunId(prev => resolveHistorySelectionAfterDelete({
        selectedHistoryRunId: prev,
        deletedRunId: screeningDeleteTarget.runId,
      }));
      if (screeningRerunBaseRunId === screeningDeleteTarget.runId) {
        setScreeningRunSource('ai');
        setScreeningRerunBaseRunId(null);
        setScreeningHistoryReuseBaseline(null);
      }
      setScreeningDeleteTarget(null);
      await refreshScreeningHistory();
    } catch (error) {
      setScreeningDeleteError(error instanceof Error ? error.message : '删除历史筛选记录失败');
    } finally {
      setScreeningDeleteLoading(false);
    }
  }, [refreshScreeningHistory, screeningDeleteTarget, screeningRerunBaseRunId]);

  const executeScreening = useCallback(async (
    promptText: string,
    options?: { universeSymbols?: string[] },
  ) => {
    const normalizedPrompt = promptText.trim();
    if (!normalizedPrompt) return;
    screeningCancelRequestedRef.current = false;
    setScreenMode('screening');
    setScreeningLoading(true);
    setScreeningError('');
    setScreeningGeneratedSql('');
    setScreeningSqlViewerOpen(false);
    setScreeningQueryProgress({
      runStatus: 'running',
      currentStage: 'prepare',
      progressPercent: 0,
      message: '准备筛选请求',
      prompt: normalizedPrompt,
      universeCount: options?.universeSymbols?.length,
      logs: [],
    });
    try {
      const response = await runScreeningQuery({
        prompt: normalizedPrompt,
        resultMode: currentScreeningResultMode,
        resultLimit: currentScreeningResultLimit,
        page: 1,
        pageSize: currentScreeningResultMode === 'unlimited' ? 200 : currentScreeningResultLimit,
        universeSymbols: options?.universeSymbols,
      });
      if (screeningCancelRequestedRef.current) {
        return;
      }
      applyScreeningResponse(response, {
        source: 'ai',
        historyRunId: null,
        rerunBaseRunId: null,
      });
      await refreshScreeningHistory();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'AI 筛选失败';
      if (screeningCancelRequestedRef.current || isScreeningQueryCanceledMessage(message)) {
        setScreeningError('');
        setScreeningQueryProgress(prev => prev ? {
          ...prev,
          runStatus: 'canceled',
          message: '已取消筛选',
          error: undefined,
        } : prev);
        return;
      }
      setScreeningError(error instanceof Error ? error.message : 'AI 筛选失败');
      setScreeningQueryProgress(prev => prev ? {
        ...prev,
        runStatus: 'failed',
        error: message,
      } : prev);
    } finally {
      setScreeningLoading(false);
    }
  }, [applyScreeningResponse, currentScreeningResultLimit, currentScreeningResultMode, refreshScreeningHistory]);

  const executeHistoryRerun = useCallback(async (
    runId: number,
    options?: { universeSymbols?: string[] },
  ) => {
    screeningCancelRequestedRef.current = false;
    setScreenMode('screening');
    setScreeningLoading(true);
    setScreeningError('');
    setScreeningGeneratedSql('');
    setScreeningSqlViewerOpen(false);
    setScreeningQueryProgress({
      runStatus: 'running',
      currentStage: 'prepare',
      progressPercent: 0,
      message: '准备根据历史 SQL 重新筛选',
      prompt: screeningPrompt.trim(),
      logs: [],
    });
    try {
      const response = options?.universeSymbols && options.universeSymbols.length > 0
        ? await rerunScreeningHistoryRunWithUniverse(runId, options.universeSymbols, 1, 200)
        : await rerunScreeningHistoryRun(runId, 1, 200);
      if (screeningCancelRequestedRef.current) {
        return;
      }
      applyScreeningResponse(response, {
        source: 'history_sql',
        historyRunId: response.runId ?? runId,
        rerunBaseRunId: response.runId ?? runId,
      });
      await refreshScreeningHistory();
    } catch (error) {
      const message = error instanceof Error ? error.message : '根据历史 SQL 重跑失败';
      if (screeningCancelRequestedRef.current || isScreeningQueryCanceledMessage(message)) {
        setScreeningError('');
        setScreeningQueryProgress(prev => prev ? {
          ...prev,
          runStatus: 'canceled',
          message: '已取消筛选',
          error: undefined,
        } : prev);
        return;
      }
      setScreeningError(message);
      setScreeningQueryProgress(prev => prev ? {
        ...prev,
        runStatus: 'failed',
        error: message,
      } : prev);
    } finally {
      setScreeningLoading(false);
    }
  }, [applyScreeningResponse, refreshScreeningHistory, screeningPrompt]);

  const handleCancelRunningScreening = useCallback(async () => {
    if (!screeningLoading) return;
    screeningCancelRequestedRef.current = true;
    setScreeningLoading(false);
    setScreeningError('');
    setScreeningQueryProgress(prev => prev ? {
      ...prev,
      runStatus: 'canceled',
      message: '已取消筛选',
      error: undefined,
    } : {
      runStatus: 'canceled',
      currentStage: 'prepare',
      progressPercent: 0,
      message: '已取消筛选',
      prompt: '',
      logs: [],
    });
    try {
      await cancelScreeningQuery();
    } catch (error) {
      console.error('Cancel screening query failed:', error);
    }
  }, [screeningLoading]);

  const handleRunScreening = useCallback(async () => {
    openScreeningConfirm(screeningPrompt);
  }, [openScreeningConfirm, screeningPrompt]);

  const handleOpenScreeningSettings = useCallback(() => {
    setSettingsInitialTab('screening');
    setShowSettings(true);
  }, []);

  const handleOpenGeneralSettings = useCallback(() => {
    setSettingsInitialTab('provider');
    setShowSettings(true);
  }, []);

  const handleCloseSettings = useCallback(() => {
    setShowSettings(false);
    setSettingsInitialTab('provider');
    window.setTimeout(() => {
      void refreshScreeningBootstrapState();
    }, 700);
  }, [refreshScreeningBootstrapState]);

  const handleWelcomeScreeningSubmit = useCallback(async () => {
    openScreeningConfirm(screeningPrompt);
  }, [openScreeningConfirm, screeningPrompt]);

  const handleWelcomeSyncOnly = useCallback(async () => {
    const normalizedPrompt = pendingScreeningPrompt.trim();
    if (!normalizedPrompt) return;

    setScreeningConfirmSyncStarted(true);
    setWelcomeSyncLoading(true);
    setWelcomeSyncError('');
    setScreeningSyncStatus(prev => createPendingScreeningSyncStatus(prev, {
      initialSyncDays: screeningConfig?.initialSyncDays ?? 30,
      retentionMode: screeningConfig?.retentionMode ?? 'forever',
      retentionDays: screeningConfig?.retentionDays ?? 60,
      limitStocks: welcomeSyncTestMode ? welcomeSyncTestLimit : 0,
      message: '准备启动同步任务...',
    }));
    try {
      console.debug('[screening-sync] start screening-dialog sync-only', {
        limitStocks: welcomeSyncTestMode ? welcomeSyncTestLimit : 0,
        prompt: normalizedPrompt,
      });

      const status = await runScreeningSync({
        mode: 'manual',
        limitStocks: welcomeSyncTestMode ? welcomeSyncTestLimit : 0,
      });
      console.debug('[screening-sync] screening-dialog sync-only finished', {
        runStatus: status.runStatus,
        completedStocks: status.completedStocks,
        totalStocks: status.totalStocks,
        syncedToLatestStocks: status.syncedToLatestStocks,
        lastMessage: status.lastMessage,
        error: status.error,
      });
      setScreeningSyncStatus(status);
      await refreshScreeningBootstrapState();
      if (!shouldContinueAfterScreeningSync(status)) {
        setScreeningConfirmSyncStarted(false);
        setWelcomeSyncError(
          status.runStatus === 'canceled'
            ? '同步已取消'
            : (status.error || '同步失败'),
        );
        return;
      }

      setScreeningConfirmSyncStarted(false);
      setScreeningConfirmVisible(false);
    } catch (error) {
      setWelcomeSyncError(error instanceof Error ? error.message : '同步失败');
    } finally {
      setWelcomeSyncLoading(false);
    }
  }, [
    canReuseHistorySql,
    executeHistoryRerun,
    pendingScreeningPrompt,
    refreshScreeningBootstrapState,
    screeningConfig,
    welcomeSyncTestLimit,
    welcomeSyncTestMode,
  ]);

  const handleWelcomeStartScreening = useCallback(async () => {
    const normalizedPrompt = pendingScreeningPrompt.trim();
    if (!normalizedPrompt) return;

    setWelcomeSyncLoading(true);
    setWelcomeSyncError('');

    try {
      let universeSymbols: string[] | undefined;
      if (welcomeSyncTestMode) {
        universeSymbols = await getScreeningUniverseSymbols(welcomeSyncTestLimit);
        if (!universeSymbols || universeSymbols.length === 0) {
          setWelcomeSyncError('测试范围已开启，但未获取到可用于 AI 筛选的股票范围');
          return;
        }
      }

      setScreeningConfirmSyncStarted(false);
      setScreeningConfirmVisible(false);

      if (canReuseHistorySql && screeningRerunBaseRunId) {
        await executeHistoryRerun(screeningRerunBaseRunId, { universeSymbols });
        return;
      }
      await executeScreening(normalizedPrompt, { universeSymbols });
    } catch (error) {
      setWelcomeSyncError(error instanceof Error ? error.message : '筛选失败');
    } finally {
      setWelcomeSyncLoading(false);
    }
  }, [
    canReuseHistorySql,
    executeHistoryRerun,
    executeScreening,
    pendingScreeningPrompt,
    screeningRerunBaseRunId,
    welcomeSyncTestLimit,
    welcomeSyncTestMode,
  ]);

  const handleCancelScreeningConfirm = useCallback(async () => {
    const isSyncRunning = welcomeSyncLoading || screeningSyncStatus?.runStatus === 'running';
    if (!isSyncRunning) {
      setScreeningConfirmSyncStarted(false);
      setScreeningConfirmVisible(false);
      return;
    }

    try {
      console.debug('[screening-sync] cancel sync-and-screen requested', {
        loading: welcomeSyncLoading,
        runStatus: screeningSyncStatus?.runStatus,
      });
      const canceled = await cancelScreeningSync();
      if (!canceled) {
        setWelcomeSyncError('当前没有正在执行的同步任务');
        return;
      }
      setWelcomeSyncError('');
      setScreeningSyncStatus(prev => prev ? {
        ...prev,
        lastMessage: '已发送取消请求，将在当前股票处理完成后停止',
      } : prev);
      window.setTimeout(() => {
        void refreshScreeningBootstrapState();
      }, 1200);
    } catch (error) {
      setWelcomeSyncError(error instanceof Error ? error.message : '取消同步失败');
    }
  }, [refreshScreeningBootstrapState, screeningSyncStatus?.runStatus, welcomeSyncLoading]);

  const handleStartSyncOnly = useCallback(async () => {
    setSyncOnlySyncStarted(true);
    setSyncOnlyLoading(true);
    setSyncOnlyError('');
    setScreeningSyncStatus(prev => createPendingScreeningSyncStatus(prev, {
      initialSyncDays: screeningConfig?.initialSyncDays ?? 30,
      retentionMode: screeningConfig?.retentionMode ?? 'forever',
      retentionDays: screeningConfig?.retentionDays ?? 60,
      limitStocks: 0,
      message: '准备启动同步任务...',
    }));
    try {
      console.debug('[screening-sync] start sync-only');
      const status = await runScreeningSync({
        mode: 'manual',
        limitStocks: 0,
      });
      console.debug('[screening-sync] sync-only finished', {
        runStatus: status.runStatus,
        completedStocks: status.completedStocks,
        totalStocks: status.totalStocks,
        syncedToLatestStocks: status.syncedToLatestStocks,
        lastMessage: status.lastMessage,
        error: status.error,
      });
      setScreeningSyncStatus(status);
      await refreshScreeningBootstrapState();
      if (status.error) {
        setSyncOnlyError(status.error);
        return;
      }
      if (status.runStatus === 'failed') {
        setSyncOnlyError('同步失败');
        return;
      }
      if (status.runStatus === 'canceled') {
        setSyncOnlyError('同步已取消');
        return;
      }
      if (!syncOnlyDialogMinimized) {
        setSyncOnlySyncStarted(false);
        setSyncOnlyDialogVisible(false);
      }
    } catch (error) {
      setSyncOnlyError(error instanceof Error ? error.message : '同步失败');
    } finally {
      setSyncOnlyLoading(false);
    }
  }, [refreshScreeningBootstrapState, screeningConfig, syncOnlyDialogMinimized]);

  const handleCancelSyncOnly = useCallback(async () => {
    const isSyncRunning = syncOnlyLoading || screeningSyncStatus?.runStatus === 'running';
    if (!isSyncRunning) {
      setSyncOnlySyncStarted(false);
      setSyncOnlyDialogVisible(false);
      return;
    }

    try {
      console.debug('[screening-sync] cancel sync-only requested', {
        loading: syncOnlyLoading,
        runStatus: screeningSyncStatus?.runStatus,
      });
      const canceled = await cancelScreeningSync();
      if (!canceled) {
        setSyncOnlyError('当前没有正在执行的同步任务');
        return;
      }
      setSyncOnlyError('');
      setScreeningSyncStatus(prev => prev ? {
        ...prev,
        lastMessage: '已发送取消请求，将在当前股票处理完成后停止',
      } : prev);
      window.setTimeout(() => {
        void refreshScreeningBootstrapState();
      }, 1200);
    } catch (error) {
      setSyncOnlyError(error instanceof Error ? error.message : '取消同步失败');
    }
  }, [refreshScreeningBootstrapState, screeningSyncStatus?.runStatus, syncOnlyLoading]);

  const handleSelectScreeningHistory = useCallback(async (runId: number) => {
    setScreenMode('screening');
    setScreeningLoading(true);
    setScreeningError('');
    setScreeningGeneratedSql('');
    setScreeningSqlViewerOpen(false);
    setScreeningQueryProgress(null);
    try {
      const response = await getScreeningHistoryRun(runId, 1, 200);
      applyScreeningResponse(response, {
        source: 'history_sql',
        historyRunId: runId,
        rerunBaseRunId: runId,
      });
    } catch (error) {
      setScreeningError(error instanceof Error ? error.message : '加载历史筛选失败');
    } finally {
      setScreeningLoading(false);
    }
  }, [applyScreeningResponse]);

  // Load watchlist on mount
  useEffect(() => {
    const loadWatchlist = async () => {
      try {
        // 加载布局配置
        const appConfig = await getConfig();
        if (appConfig.layout) {
          if (appConfig.layout.leftPanelWidth > 0) setLeftPanelWidth(appConfig.layout.leftPanelWidth);
          if (appConfig.layout.rightPanelWidth > 0) setRightPanelWidth(appConfig.layout.rightPanelWidth);
          if (appConfig.layout.bottomPanelHeight > 0) setBottomPanelHeight(appConfig.layout.bottomPanelHeight);
          if (appConfig.layout.textScalePercent > 0) setTextScalePercent(clampTextScalePercent(appConfig.layout.textScalePercent));
          if (appConfig.layout.klineZoomPercent > 0) {
            setKlineZoomPercent(
              Math.max(
                MIN_KLINE_ZOOM_PERCENT,
                Math.min(MAX_KLINE_ZOOM_PERCENT, Math.round(appConfig.layout.klineZoomPercent)),
              ),
            );
          }
          const maximized = await WindowIsMaximised().catch(() => false);
          setIsMaximized(maximized);
          const restoreDecision = resolveStartupWindowRestore(
            appConfig.layout.windowWidth,
            appConfig.layout.windowHeight,
            maximized,
          );
          infoAppEvent('window', 'startup layout restore evaluated', {
            module: 'window',
            action: 'startup.restore_layout',
            isMaximized: maximized,
            restoreWindowSizeSkipped: !restoreDecision.shouldRestore,
            restoreReason: restoreDecision.reason,
            savedWindowWidth: restoreDecision.savedWindowWidth,
            savedWindowHeight: restoreDecision.savedWindowHeight,
          });
          if (restoreDecision.shouldRestore) {
            WindowSetSize(restoreDecision.savedWindowWidth, restoreDecision.savedWindowHeight);
          }
        }

        const [status, list, history] = await Promise.all([
          getScreeningSyncStatus().catch(() => null),
          getWatchlist(),
          listScreeningHistory(20).catch(() => []),
        ]);
        setScreeningConfig((appConfig.screening as ScreeningConfig) ?? null);
        setScreeningSQLTimeoutLabel(resolveDefaultScreeningSQLTimeoutLabel(appConfig));
        setScreeningSyncStatus(status);
        setWatchlist(list);
        setScreeningHistory(history);
        if (list.length === 0 && history.length > 0) {
          setScreenMode('screening');
        }
        if (list.length > 0) {
          setSelectedSymbol(list[0].symbol);
          // 订阅第一个股票的盘口推送
          subscribeOrderBook(list[0].symbol);
          // 加载第一个股票的Session
          const session = await getOrCreateSession(list[0].symbol, list[0].name);
          setCurrentSession(session);
        }
        // 主动获取一次快讯数据（解决启动时后端推送早于前端监听注册的时序问题）
        const telegraphs = await GetTelegraphList();
        if (telegraphs && telegraphs.length > 0) {
          const latest = telegraphs[0];
          setMarketMessage(`[${latest.time}] ${latest.content}`);
        }
      } catch (err) {
        console.error('Failed to load watchlist:', err);
      } finally {
        setLoading(false);
      }
    };
    loadWatchlist();
  }, [subscribeOrderBook]);

  // Load K-line data when symbol or period changes
  useEffect(() => {
    if (!selectedSymbol) return;
    const requestId = ++klineRequestIdRef.current;
    const dataLen = timePeriod === '1m' ? 250 : 240;
    const maxRetries = 2;
    // 切换时先切回 full 模式，等待全量数据到达
    setKLineUpdateMode('full');
    // 清空旧数据，避免切换期间出现“新股票 + 旧K线”错配
    setKLineData([]);
    // 订阅K线推送
    subscribeKLine(selectedSymbol, timePeriod);
    debugAppEvent('kline', 'frontend request scheduled', {
      module: 'market',
      action: 'kline.request',
      symbol: selectedSymbol,
      period: timePeriod,
      days: dataLen,
      requestId,
      attempt: 1,
    });

    const loadKLineData = async () => {
      for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
        if (requestId !== klineRequestIdRef.current) return;
        const attemptIndex = attempt + 1;
        try {
          debugAppEvent('kline', 'frontend request start', {
            module: 'market',
            action: 'kline.request',
            symbol: selectedSymbol,
            period: timePeriod,
            days: dataLen,
            requestId,
            attempt: attemptIndex,
          });
          const data = await getKLineData(selectedSymbol, timePeriod, dataLen, {
            requestId: `kline-${requestId}`,
          });
          if (requestId !== klineRequestIdRef.current) return;
          if (Array.isArray(data) && data.length > 0) {
            setKLineUpdateMode('full');
            setKLineData(data);
            klineHistoryAvailabilityRef.current.set(selectedSymbol, true);
            debugAppEvent('kline', 'frontend request success', {
              module: 'market',
              action: 'kline.success',
              symbol: selectedSymbol,
              period: timePeriod,
              days: dataLen,
              requestId,
              attempt: attemptIndex,
              resultLen: data.length,
            });
            return;
          }
          warnAppEvent('kline', 'frontend empty response', {
            module: 'market',
            action: 'kline.empty',
            symbol: selectedSymbol,
            period: timePeriod,
            days: dataLen,
            requestId,
            attempt: attemptIndex,
            resultLen: Array.isArray(data) ? data.length : 0,
          });
        } catch (err) {
          if (requestId !== klineRequestIdRef.current) return;
          errorAppEvent('kline', 'frontend request failed', {
            module: 'market',
            action: 'kline.failed',
            symbol: selectedSymbol,
            period: timePeriod,
            days: dataLen,
            requestId,
            attempt: attemptIndex,
            err: toErrorMessage(err),
          });
        }
        if (attempt < maxRetries) {
          await new Promise(resolve => setTimeout(resolve, 600));
        }
      }
      if (requestId !== klineRequestIdRef.current) return;
      warnAppEvent('kline', 'frontend retries exhausted', {
        module: 'market',
        action: 'kline.retries_exhausted',
        symbol: selectedSymbol,
        period: timePeriod,
        days: dataLen,
        requestId,
        attempt: maxRetries + 1,
      });
      if (shouldShowKlineRateLimitToast({
        retriesExhausted: true,
        hasAnyCachedData: klineHistoryAvailabilityRef.current.get(selectedSymbol) === true,
      })) {
        showToast('error', '当前请求速度太快，触发了平台的风控限制，请稍后再试。', 3000);
      }
    };

    void loadKLineData();
  }, [selectedSymbol, timePeriod, subscribeKLine]);

  // 初始化窗口最大化状态
  useEffect(() => {
    void syncWindowMaximizedState();
  }, [syncWindowMaximizedState]);

  if (loading) return <div className="h-screen w-screen flex items-center justify-center fin-app text-white">加载中...</div>;

  const shouldShowHomePage = screenMode === 'home' || (
    watchlist.length === 0 &&
    screeningHistory.length === 0 &&
    screeningResults.length === 0 &&
    screenMode !== 'screening' &&
    !screeningLoading
  );

  if (shouldShowHomePage) {
    return (
      <>
        <WelcomePage
          screeningPrompt={screeningPrompt}
          onScreeningPromptChange={(value) => {
            setScreeningPrompt(value);
            if (screeningConfirmVisible) {
              setScreeningConfirmVisible(false);
            }
            if (welcomeSyncError) {
              setWelcomeSyncError('');
            }
          }}
          onSubmitScreening={() => { void handleWelcomeScreeningSubmit(); }}
          screeningSubmitting={screeningLoading}
          onAddStock={handleAddStock}
          hasExistingWorkspace={watchlist.length > 0 || screeningHistory.length > 0 || screeningResults.length > 0}
          onOpenWatchlist={() => setScreenMode('watchlist')}
          onOpenScreening={() => setScreenMode('screening')}
          onOpenSettings={handleOpenGeneralSettings}
        />
        <SettingsDialog isOpen={showSettings} onClose={handleCloseSettings} initialTab={settingsInitialTab} />
        <DeleteScreeningHistoryDialog
          visible={screeningDeleteTarget !== null}
          item={screeningDeleteTarget}
          loading={screeningDeleteLoading}
          error={screeningDeleteError}
          onClose={handleCloseDeleteScreeningHistory}
          onConfirm={() => { void handleConfirmDeleteScreeningHistory(); }}
        />
        <SyncActionDialog
          mode="screening"
          visible={screeningConfirmVisible}
          syncStarted={screeningConfirmSyncStarted}
          prompt={pendingScreeningPrompt}
          syncSummary={screeningSummary}
          syncStatus={screeningSyncStatus}
          loading={welcomeSyncLoading}
          error={welcomeSyncError}
          testMode={welcomeSyncTestMode}
          testLimit={welcomeSyncTestLimit}
          onTestModeChange={setWelcomeSyncTestMode}
          onTestLimitChange={setWelcomeSyncTestLimit}
          onClose={closeScreeningConfirm}
          onCancelSync={() => { void handleCancelScreeningConfirm(); }}
          onConfirm={() => { void handleWelcomeStartScreening(); }}
          onSecondaryAction={() => { void handleWelcomeSyncOnly(); }}
          onOpenSyncSettings={handleOpenScreeningSettings}
        />
      </>
    );
  }

  return (
    <div
      ref={keyboardHostRef}
      tabIndex={-1}
      className="flex flex-col h-screen text-slate-100 font-sans fin-app outline-none"
    >
      {/* Top Navbar */}
      <header
        className="h-14 fin-panel border-b fin-divider flex items-center px-4 justify-between shrink-0 z-20"
        style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
        onDoubleClick={(e) => {
          // 排除 no-drag 区域的双击
          const target = e.target as HTMLElement;
          if (target.closest('[style*="no-drag"]') || target.closest('button') || target.closest('input')) return;
          void toggleWindowMaximize();
        }}
      >
        <div className="flex items-center gap-2" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <img src={logo} alt="logo" className="h-8 w-8 rounded-lg" />
          <span className={`font-bold text-lg tracking-tight ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>散牛盘 <span className="text-accent-2">AI</span></span>
          <div className={`ml-4 inline-flex rounded-full border p-1 ${colors.isDark ? 'border-slate-700 bg-slate-900/50' : 'border-slate-300 bg-white/70'}`}>
            <button
              onClick={() => setScreenMode('home')}
              className={`rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              首页
            </button>
            <button
              onClick={() => setScreenMode('watchlist')}
              className={`rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                screenMode === 'watchlist'
                  ? 'bg-accent text-white shadow-sm'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              自选
            </button>
            <button
              onClick={() => setScreenMode('screening')}
              className={`inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-xs font-semibold transition-colors ${
                screenMode === 'screening'
                  ? 'bg-accent text-white shadow-sm'
                  : `${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'}`
              }`}
            >
              <Sparkles className="h-3.5 w-3.5" />
              AI 筛选
            </button>
          </div>
        </div>

        <div className="flex items-center gap-4 fin-panel-soft px-4 py-1.5 rounded-full border fin-divider relative" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <Radio className="h-3 w-3 animate-pulse text-accent-2" />
          <span className={`text-xs font-mono w-96 truncate text-center ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
            实时快讯: {marketMessage}
          </span>
          <button
            onClick={handleShowTelegraphList}
            className={`p-1 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400' : 'hover:bg-slate-200/50 text-slate-500'} hover:text-accent-2`}
            title="查看快讯列表"
          >
            <List className="h-4 w-4" />
          </button>

          {/* 快讯下拉列表 */}
          {showTelegraphList && (
            <div
              className="absolute top-full left-0 right-0 mt-2 fin-panel border fin-divider rounded-lg shadow-xl z-50 max-h-96 overflow-y-auto fin-scrollbar text-left"
              onMouseLeave={() => setShowTelegraphList(false)}
            >
              <div className={`p-2 border-b fin-divider text-xs font-medium ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                财联社快讯
              </div>
              {telegraphLoading ? (
                <div className={`p-4 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>加载中...</div>
              ) : telegraphList.length === 0 ? (
                <div className={`p-4 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>暂无快讯</div>
              ) : (
                telegraphList.map((tg, idx) => (
                  <div
                    key={idx}
                    onClick={() => handleOpenTelegraph(tg)}
                    className={`p-3 border-b fin-divider last:border-b-0 cursor-pointer transition-colors ${colors.isDark ? 'hover:bg-slate-800/50' : 'hover:bg-slate-100/80'}`}
                  >
                    <div className="flex items-start gap-2">
                      <span className="text-xs text-accent-2 font-mono shrink-0">{tg.time}</span>
                      <span className={`text-xs line-clamp-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>{tg.content}</span>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        <div className="flex items-center gap-3" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
          <button
            onClick={handleOpenSyncOnlyDialog}
            disabled={topbarSyncButtonState.disabled}
            className={`inline-flex h-10 min-w-[164px] items-center justify-between gap-3 rounded-xl border px-4 text-sm font-medium transition-colors ${
              getTopbarSyncButtonToneStyles(topbarSyncButtonState.tone, colors.isDark, topbarSyncButtonState.disabled)
            }`}
            title={topbarSyncButtonState.label}
          >
            <div className="flex min-w-0 items-center gap-2">
              {topbarSyncButtonState.loading && (
                <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
              )}
              <span className="truncate">{topbarSyncButtonState.label}</span>
            </div>
            <span className="shrink-0 text-sm opacity-80">{topbarSyncButtonState.detail}</span>
          </button>
          <button
            onClick={() => setShowLongHuBang(true)}
            className={`rounded-xl border px-4 py-2 text-sm font-medium fin-panel fin-divider transition-colors ${
              colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'
            } hover:border-red-400/40`}
            title="龙虎榜"
          >
            龙虎榜
          </button>
          <button
            onClick={() => setShowHotTrend(true)}
            className={`rounded-xl border px-4 py-2 text-sm font-medium fin-panel fin-divider transition-colors ${
              colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'
            } hover:border-orange-400/40`}
            title="全网热点"
          >
            全网热点
          </button>
          <ThemeSwitcher />
          <button
            onClick={handleOpenGeneralSettings}
            className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${colors.isDark ? 'text-slate-300 hover:text-white' : 'text-slate-600 hover:text-slate-900'} hover:border-accent/40`}
          >
            <Settings className="h-4 w-4" />
          </button>
          <div className="text-xs text-right hidden md:block">
            <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>市场状态</div>
            <div className={`font-bold ${
              marketStatus?.status === 'trading' ? 'text-green-500' :
              marketStatus?.status === 'pre_market' ? 'text-yellow-500' :
              marketStatus?.status === 'lunch_break' ? 'text-orange-500' :
              colors.isDark ? 'text-slate-500' : 'text-slate-400'
            }`}>
              {marketStatus?.statusText || '加载中...'}
            </div>
          </div>
          {/* 窗口控制按钮 */}
          <div className="flex items-center ml-2 border-l fin-divider pl-3">
            <button
              onClick={() => WindowMinimize()}
              className={`p-1.5 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-900'}`}
              title="最小化"
            >
              <Minus className="h-4 w-4" />
            </button>
            <button
              onClick={() => { void toggleWindowMaximize(); }}
              className={`p-1.5 rounded transition-colors ${colors.isDark ? 'hover:bg-slate-700/50 text-slate-400 hover:text-white' : 'hover:bg-slate-200/50 text-slate-500 hover:text-slate-900'}`}
              title={isMaximized ? "还原" : "最大化"}
            >
              {isMaximized ? <Copy className="h-3.5 w-3.5" /> : <Square className="h-3.5 w-3.5" />}
            </button>
            <button
              onClick={() => WindowClose()}
              className={`p-1.5 rounded hover:bg-red-500/80 hover:text-white transition-colors ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}
              title="关闭"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
      </header>

      {/* Main Content Grid */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Sidebar: Watchlist */}
        <div style={{ width: leftPanelWidth }} className="shrink-0 fin-panel overflow-hidden">
          {screenMode === 'watchlist' ? (
            <StockList
              stocks={watchlist}
              selectedSymbol={selectedSymbol}
              onSelect={handleSelectStock}
              onAddStock={handleAddStock}
              onRemoveStock={handleRemoveStock}
              marketIndices={marketIndices}
            />
          ) : (
            <div className="flex h-full flex-col">
              <div className="min-h-0 flex-1">
                <ScreeningResultList
                  activeTab={screeningResultTab}
                  results={screeningResults}
                  history={screeningHistory}
                  generatedSql={screeningGeneratedSql}
                  selectedSymbol={selectedSymbol}
                  selectedHistoryRunId={screeningSelectedHistoryRunId}
                  totalCount={screeningTotalCount}
                  isLoading={screeningLoading}
                  queryProgress={screeningQueryProgress}
                  error={screeningError}
                  watchlistSymbols={watchlistSymbols}
                  deletingHistoryRunId={screeningDeleteLoading ? screeningDeleteTarget?.runId ?? null : null}
                  onTabChange={setScreeningResultTab}
                  onSelectHistory={(runId) => { void handleSelectScreeningHistory(runId); }}
                  onRequestDeleteHistory={handleRequestDeleteScreeningHistory}
                  onSelect={(symbol) => { void handleSelectStock(symbol); }}
                  onAddToWatchlist={handleAddStock}
                />
              </div>
              <div className="shrink-0 border-t fin-divider-soft">
                <ScreeningWorkspace
                  prompt={screeningPrompt}
                  resultPreset={screeningPreset}
                  loading={screeningLoading}
                  showHistoryRerunLabel={shouldShowHistoryRerunLabel}
                  canCancel={screeningLoading}
                  generatedSql={screeningGeneratedSql}
                  onPromptChange={(value) => {
                    setScreeningPrompt(value);
                    setScreeningGeneratedSql('');
                    setScreeningSqlViewerOpen(false);
                  }}
                  onResultPresetChange={(value) => setScreeningPreset(value as ScreeningResultPreset)}
                  onRun={() => { void handleRunScreening(); }}
                  onCancel={() => { void handleCancelRunningScreening(); }}
                  onViewSql={() => setScreeningSqlViewerOpen(true)}
                />
              </div>
            </div>
          )}
        </div>

        {/* Left Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleLeftResize} onResizeEnd={handleResizeEnd} />

        {/* Center Panel: Charts & Data */}
        <div className="flex-1 flex flex-col min-w-0 fin-panel-center">
          {!selectedStock ? (
            <div className={`flex h-full items-center justify-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              从左侧自选或 AI 筛选结果中选择一只股票查看详情。
            </div>
          ) : (
            <>
          {/* Stock Header - A股风格 */}
          <div className="px-6 py-3 shrink-0 border-b fin-divider-soft">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className={`text-lg font-bold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{selectedStock.name}</span>
                <span className={`text-sm font-mono ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{selectedStock.symbol}</span>
                <button
                  onClick={() => setShowPosition(true)}
                  className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${colors.isDark ? 'text-slate-400 hover:bg-slate-700/50' : 'text-slate-500 hover:bg-slate-200/50'} hover:text-accent-2`}
                  title="持仓设置"
                >
                  <Briefcase className="h-3.5 w-3.5" />
                  {currentSession?.position && currentSession.position.shares > 0 ? (
                    (() => {
                      const pos = currentSession.position;
                      const marketValue = pos.shares * selectedStock.price;
                      const costAmount = pos.shares * pos.costPrice;
                      const profitLoss = marketValue - costAmount;
                      const profitPercent = costAmount > 0 ? (profitLoss / costAmount) * 100 : 0;
                      const isProfit = profitLoss >= 0;
                      return (
                        <span className={isProfit ? cc.upClass : cc.downClass}>
                          {pos.shares}股 {isProfit ? '+' : ''}{profitLoss.toFixed(0)} ({isProfit ? '+' : ''}{profitPercent.toFixed(2)}%)
                        </span>
                      );
                    })()
                  ) : (
                    <span>设置持仓</span>
                  )}
                </button>
              </div>
              <div className={`text-3xl font-mono font-bold ${cc.getColorClass(selectedStock.change >= 0)}`}>
                {selectedStock.price.toFixed(2)}
              </div>
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4 text-sm">
                <span className={`font-mono ${cc.getColorClass(selectedStock.change >= 0)}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.change.toFixed(2)}
                </span>
                <span className={`font-mono ${cc.getColorClass(selectedStock.change >= 0)}`}>
                  {selectedStock.change >= 0 ? '+' : ''}{selectedStock.changePercent.toFixed(2)}%
                </span>
              </div>
              <div className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                {new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </div>
            </div>
          </div>

          {/* A股传统行情数据 */}
          <div className="grid grid-cols-4 gap-px p-2 border-b fin-divider-soft shrink-0 text-xs">
            <AStockStatItem label="今开" value={selectedStock.open} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="最高" value={selectedStock.high} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="成交量" value={formatVolume(selectedStock.volume)} isPlain isDark={colors.isDark} />
            <AStockStatItem label="昨收" value={selectedStock.preClose} isPlain isDark={colors.isDark} />
            <AStockStatItem label="最低" value={selectedStock.low} preClose={selectedStock.preClose} isDark={colors.isDark} />
            <AStockStatItem label="成交额" value={formatAmount(selectedStock.amount)} isPlain isDark={colors.isDark} />
            <AStockStatItem label="振幅" value={selectedStock.preClose > 0 ? ((selectedStock.high - selectedStock.low) / selectedStock.preClose * 100).toFixed(2) + '%' : '--'} isPlain isDark={colors.isDark} />
          </div>

          <div className="flex-1 flex flex-col min-h-0">
             {/* Chart Section */}
            <div className="flex-1 p-1 relative min-h-0">
               <StockChartLW
                  ref={stockChartRef}
                  data={kLineData}
                  updateMode={kLineUpdateMode}
                  klineZoomPercent={klineZoomPercent}
                  period={timePeriod}
                  onPeriodChange={setTimePeriod}
                  stock={selectedStock}
               />
            </div>

            {/* Bottom Resize Handle */}
            <ResizeHandle direction="vertical" onResize={handleBottomResize} onResizeEnd={handleResizeEnd} />

            {/* Bottom Info Panel: Order Book Only */}
            <div style={{ height: bottomPanelHeight }} className="border-t fin-divider-soft flex shrink-0">
               <div className="flex-1 overflow-hidden relative">
                  <OrderBookComponent data={orderBook} />
               </div>
            </div>
          </div>
            </>
          )}
        </div>

        {/* Right Resize Handle */}
        <ResizeHandle direction="horizontal" onResize={handleRightResize} onResizeEnd={handleResizeEnd} />

        {/* Right Panel: AI Agents */}
        <div style={{ width: rightPanelWidth }} className="shrink-0 fin-panel overflow-hidden">
          {selectedStock ? (
            <AgentRoom
              stock={selectedStock}
              kLineData={kLineData}
              session={currentSession}
              onSessionUpdate={setCurrentSession}
            />
          ) : (
            <div className={`flex h-full items-center justify-center px-6 text-center text-sm ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              选择股票后，这里会继续复用现有 AI 分析工作区。
            </div>
          )}
        </div>
      </div>
      <SettingsDialog isOpen={showSettings} onClose={handleCloseSettings} initialTab={settingsInitialTab} />
      <DeleteScreeningHistoryDialog
        visible={screeningDeleteTarget !== null}
        item={screeningDeleteTarget}
        loading={screeningDeleteLoading}
        error={screeningDeleteError}
        onClose={handleCloseDeleteScreeningHistory}
        onConfirm={() => { void handleConfirmDeleteScreeningHistory(); }}
      />
      {selectedStock && (
        <PositionDialog
          isOpen={showPosition}
          onClose={() => setShowPosition(false)}
          stockCode={selectedStock.symbol}
          stockName={selectedStock.name}
          currentPrice={selectedStock.price}
          position={currentSession?.position}
          onSave={async (shares, costPrice) => {
            await updateStockPosition(selectedStock.symbol, shares, costPrice);
            const session = await getOrCreateSession(selectedStock.symbol, selectedStock.name);
            setCurrentSession(session);
          }}
        />
      )}
      <HotTrendDialog isOpen={showHotTrend} onClose={() => setShowHotTrend(false)} />
      <LongHuBangDialog isOpen={showLongHuBang} onClose={() => setShowLongHuBang(false)} />
      <ScreeningSQLDialog
        isOpen={screeningSqlViewerOpen}
        sql={screeningGeneratedSql}
        onClose={() => setScreeningSqlViewerOpen(false)}
      />
      <SyncActionDialog
        mode="screening"
        visible={screeningConfirmVisible}
        syncStarted={screeningConfirmSyncStarted}
        prompt={pendingScreeningPrompt}
        syncSummary={screeningSummary}
        syncStatus={screeningSyncStatus}
        loading={welcomeSyncLoading}
        error={welcomeSyncError}
        testMode={welcomeSyncTestMode}
        testLimit={welcomeSyncTestLimit}
        onTestModeChange={setWelcomeSyncTestMode}
        onTestLimitChange={setWelcomeSyncTestLimit}
        onClose={closeScreeningConfirm}
        onCancelSync={() => { void handleCancelScreeningConfirm(); }}
        onConfirm={() => { void handleWelcomeStartScreening(); }}
        onSecondaryAction={() => { void handleWelcomeSyncOnly(); }}
        onOpenSyncSettings={handleOpenScreeningSettings}
      />
      <SyncActionDialog
        mode="sync-only"
        visible={syncOnlyDialogVisible && !syncOnlyDialogMinimized}
        syncStarted={syncOnlySyncStarted}
        syncSummary={screeningSummary}
        syncStatus={screeningSyncStatus}
        loading={syncOnlyLoading}
        error={syncOnlyError}
        onClose={closeSyncOnlyDialog}
        onCancelSync={() => { void handleCancelSyncOnly(); }}
        onConfirm={() => { void handleStartSyncOnly(); }}
        minimizable
        onMinimize={handleMinimizeSyncOnlyDialog}
        onOpenSyncSettings={handleOpenScreeningSettings}
      />
      {syncOnlyMinimizedCardState.visible && (
        <div className={`fixed bottom-4 right-4 z-[125] w-[320px] rounded-2xl border shadow-2xl ${
          colors.isDark ? 'border-slate-700 bg-slate-900/95' : 'border-slate-200 bg-white/95'
        }`}>
          <div className="flex items-start justify-between gap-3 px-4 py-3">
            <div className="min-w-0 flex-1">
              <div className={`flex items-center gap-2 text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                {(syncOnlyLoading || screeningSyncStatus?.runStatus === 'running') && (
                  <Loader2 className="h-4 w-4 animate-spin text-[var(--accent)]" />
                )}
                <span>{syncOnlyMinimizedCardState.title}</span>
              </div>
              <div className={`mt-1 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                {syncOnlyMinimizedCardState.detail || '可恢复查看完整同步面板'}
              </div>
            </div>
            <button
              type="button"
              onClick={handleRestoreSyncOnlyDialog}
              className={`rounded-lg p-2 transition-colors ${
                colors.isDark ? 'text-slate-400 hover:bg-slate-800 hover:text-white' : 'text-slate-500 hover:bg-slate-100 hover:text-slate-800'
              }`}
              title="恢复"
            >
              <Square className="h-4 w-4" />
            </button>
          </div>
          <div className="px-4 pb-4">
            <div className="flex items-center justify-between text-xs">
              <span className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>同步进度</span>
              <span className={`font-semibold ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}>
                {syncOnlyMinimizedCardState.progressLabel}
              </span>
            </div>
            <div className={`mt-2 h-2 overflow-hidden rounded-full ${colors.isDark ? 'bg-slate-800' : 'bg-slate-200'}`}>
              <div
                className="h-full rounded-full bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)] transition-all"
                style={{ width: `${syncOnlyMinimizedCardState.progressPercent}%` }}
              />
            </div>
            <div className={`mt-2 text-right text-xs font-semibold ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              {syncOnlyMinimizedCardState.progressPercent}%
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

interface ScreeningSQLDialogProps {
  isOpen: boolean;
  sql: string;
  onClose: () => void;
}

interface DeleteScreeningHistoryDialogProps {
  visible: boolean;
  item: ScreeningHistoryItem | null;
  loading: boolean;
  error: string;
  onClose: () => void;
  onConfirm: () => void;
}

interface SyncActionDialogProps {
  mode: ScreeningSyncDialogMode;
  visible: boolean;
  syncStarted: boolean;
  prompt?: string;
  syncSummary: {
    strategyItems: Array<{ label: string; value: string }>;
    dataItems: Array<{ label: string; value: string; tone?: SyncHealthTone }>;
    healthSummary: {
      tone: SyncHealthTone;
      title: string;
      detail: string;
    };
  };
  syncStatus: ScreeningSyncStatus | null;
  loading: boolean;
  error: string;
  testMode?: boolean;
  testLimit?: number;
  onTestModeChange?: (value: boolean) => void;
  onTestLimitChange?: (value: number) => void;
  onClose: () => void;
  onCancelSync: () => void;
  onConfirm: () => void;
  onSecondaryAction?: () => void;
  minimizable?: boolean;
  onMinimize?: () => void;
  onOpenSyncSettings: () => void;
}

const DeleteScreeningHistoryDialog: React.FC<DeleteScreeningHistoryDialogProps> = ({
  visible,
  item,
  loading,
  error,
  onClose,
  onConfirm,
}) => {
  const { colors } = useTheme();

  if (!visible || !item) return null;

  return (
    <div className="fixed inset-0 z-[130] flex items-center justify-center bg-slate-950/45 px-4 backdrop-blur-sm">
      <div className={`w-full max-w-md rounded-3xl border shadow-2xl ${
        colors.isDark ? 'border-slate-700 bg-slate-900/95' : 'border-slate-200 bg-white/95'
      }`}>
        <div className="border-b fin-divider-soft px-6 py-5">
          <div className={`text-lg font-semibold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>
            删除历史筛选记录
          </div>
          <div className={`mt-1 text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            删除后无法恢复，确认删除这条历史筛选记录吗？
          </div>
        </div>

        <div className="px-6 py-5">
          <div className={`rounded-2xl border px-4 py-4 text-sm ${
            colors.isDark ? 'border-slate-700 bg-slate-950/40 text-slate-200' : 'border-slate-200 bg-slate-50/80 text-slate-700'
          }`}>
            {item.prompt}
          </div>
          {error && (
            <div className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-400">
              {error}
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-3 border-t fin-divider-soft px-6 py-4">
          <button
            type="button"
            onClick={onClose}
            disabled={loading}
            className={`rounded-xl border px-4 py-2 text-sm ${
              colors.isDark ? 'border-slate-700 text-slate-300 hover:bg-slate-800/70' : 'border-slate-300 text-slate-600 hover:bg-slate-100'
            } disabled:opacity-50`}
          >
            取消
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-xl bg-red-500 px-4 py-2 text-sm font-semibold text-white hover:bg-red-600 disabled:opacity-60"
          >
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            <span>{loading ? '删除中...' : '确认删除'}</span>
          </button>
        </div>
      </div>
    </div>
  );
};

const SyncActionDialog: React.FC<SyncActionDialogProps> = ({
  mode,
  visible,
  syncStarted,
  prompt,
  syncSummary,
  syncStatus,
  loading,
  error,
  testMode = false,
  testLimit = 10,
  onTestModeChange,
  onTestLimitChange,
  onClose,
  onCancelSync,
  onConfirm,
  onSecondaryAction,
  minimizable = false,
  onMinimize,
  onOpenSyncSettings,
}) => {
  const { colors } = useTheme();
  const copy = resolveSyncDialogCopy(mode);
  const headerControls = resolveSyncDialogHeaderControls({ minimizable, loading });
  const isScreeningMode = mode === 'screening';
  const syncProgressPercent = Math.max(0, Math.min(100, Math.round(syncStatus?.progressPercent ?? 0)));
  const syncRecentEvents = (syncStatus?.events ?? []).slice(-3).reverse();
  const isSyncRunning = loading || syncStatus?.runStatus === 'running';
  const shouldShowSyncProgress = syncStarted && (
    loading
    || syncStatus?.runStatus === 'running'
    || syncStatus?.runStatus === 'failed'
    || syncStatus?.runStatus === 'canceled'
    || (mode === 'sync-only' && syncStatus?.runStatus === 'completed')
    || Boolean(syncStatus?.error)
    || Boolean(error)
  );

  if (!visible) return null;

  const healthToneStyles = getSyncHealthToneStyles(syncSummary.healthSummary.tone, colors.isDark);

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/45 px-4 backdrop-blur-sm">
      <div className={`w-full max-w-2xl rounded-3xl border shadow-2xl ${
        colors.isDark ? 'border-slate-700 bg-slate-900/95' : 'border-slate-200 bg-white/95'
      }`}>
        <div className="flex items-start justify-between gap-4 border-b fin-divider-soft px-6 py-5">
          <div className="min-w-0 flex-1 text-left">
            <div className={`text-lg font-semibold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>
              {copy.title}
            </div>
            <div className={`mt-1 text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
              {copy.description}
            </div>
          </div>
          <div className="flex items-center gap-1">
            {headerControls.showMinimize && onMinimize && (
              <button
                type="button"
                onClick={onMinimize}
                disabled={headerControls.minimizeDisabled}
                className={`rounded-full p-2 ${colors.isDark ? 'text-slate-400 hover:bg-slate-800/80 hover:text-white' : 'text-slate-500 hover:bg-slate-100 hover:text-slate-800'} disabled:opacity-50`}
                title="最小化"
              >
                <Minus className="h-4 w-4" />
              </button>
            )}
            <button
              onClick={onClose}
              disabled={headerControls.closeDisabled}
              className={`rounded-full p-2 ${colors.isDark ? 'text-slate-400 hover:bg-slate-800/80 hover:text-white' : 'text-slate-500 hover:bg-slate-100 hover:text-slate-800'} disabled:opacity-50`}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        <div className="space-y-4 px-6 py-5">
          {!syncStarted && (
            <>
              {isScreeningMode && (
                <div className={`rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-950/40' : 'border-slate-200 bg-slate-50/80'}`}>
                  <div className={`text-left text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>本次筛选条件</div>
                  <div className={`mt-2 whitespace-pre-wrap text-left text-sm ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>{prompt}</div>
                </div>
              )}

              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div className={`rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-950/40' : 'border-slate-200 bg-slate-50/80'}`}>
                  <div className={`text-left text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>当前同步策略</div>
                  <div className="mt-3 space-y-2">
                    {syncSummary.strategyItems.map((item) => (
                      <div key={item.label} className="flex items-start justify-between gap-3 text-sm">
                        <span className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>{item.label}</span>
                        <span className={`text-right font-medium ${colors.isDark ? 'text-slate-200' : 'text-slate-700'}`}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                </div>

                <div className={`rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-950/40' : 'border-slate-200 bg-slate-50/80'}`}>
                  <div className={`text-left text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>当前数据状态</div>
                  <div className={`mt-3 rounded-2xl border px-3 py-3 ${healthToneStyles.panel}`}>
                    <div className="flex items-start gap-3">
                      <div className={`mt-0.5 ${healthToneStyles.icon}`}>
                        {syncSummary.healthSummary.tone === 'success' ? (
                          <CheckCircle2 className="h-4 w-4" />
                        ) : syncSummary.healthSummary.tone === 'warning' ? (
                          <Clock3 className="h-4 w-4" />
                        ) : (
                          <AlertCircle className="h-4 w-4" />
                        )}
                      </div>
                      <div className="text-left">
                        <div className={`text-sm font-semibold ${healthToneStyles.text}`}>{syncSummary.healthSummary.title}</div>
                        <div className={`mt-1 text-xs ${healthToneStyles.subtext}`}>{syncSummary.healthSummary.detail}</div>
                      </div>
                    </div>
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-3">
                    {syncSummary.dataItems.map((item) => (
                      <div key={item.label} className={`rounded-xl border px-3 py-3 ${colors.isDark ? 'border-slate-800 bg-slate-900/70' : 'border-slate-200 bg-white/70'}`}>
                        <div className={`text-[11px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{item.label}</div>
                        <div className={`mt-1 text-sm font-semibold ${resolveSyncDataItemToneClass(item.tone, colors.isDark)}`}>{item.value}</div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {isScreeningMode && (
                <div className={`rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-950/40' : 'border-slate-200 bg-slate-50/80'}`}>
                  <div className={`text-left text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>测试范围</div>
                  <label className="mt-3 flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={testMode}
                      disabled={loading}
                      onChange={(e) => onTestModeChange?.(e.target.checked)}
                      className="h-4 w-4 rounded border-slate-400 text-[var(--accent)] focus:ring-[var(--accent)]"
                    />
                    <span className={colors.isDark ? 'text-slate-200' : 'text-slate-700'}>只同步并筛选前</span>
                    <select
                      value={String(testLimit)}
                      disabled={!testMode || loading}
                      onChange={(e) => onTestLimitChange?.(Number(e.target.value))}
                      className={`rounded-xl border px-3 py-2 text-sm ${colors.isDark ? 'border-slate-600 bg-slate-800/70 text-white disabled:text-slate-500' : 'border-slate-300 bg-white text-slate-800 disabled:text-slate-400'}`}
                    >
                      <option value="10">10 只</option>
                      <option value="20">20 只</option>
                      <option value="50">50 只</option>
                      <option value="100">100 只</option>
                    </select>
                    <span className={colors.isDark ? 'text-slate-200' : 'text-slate-700'}>股票</span>
                  </label>
                  <div className={`mt-2 text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
                    {testMode
                      ? '勾选后，本次同步或筛选都只处理这批股票。'
                      : '不勾选时，本次会基于当前市场范围执行全量同步或全量筛选。'}
                  </div>
                </div>
              )}
            </>
          )}

          {shouldShowSyncProgress && (
            <div className={`rounded-2xl border px-4 py-4 ${colors.isDark ? 'border-slate-700 bg-slate-950/40' : 'border-slate-200 bg-slate-50/80'}`}>
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 flex-1 text-left">
                  <div className={`text-sm font-semibold ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>同步进度</div>
                  <div className={`mt-1 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                    {formatScreeningSyncRunStatus(syncStatus?.runStatus)}
                    {syncStatus?.lastMessage ? ` · ${syncStatus.lastMessage}` : ''}
                  </div>
                </div>
                <div className={`text-2xl font-semibold tabular-nums ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                  {syncProgressPercent}%
                </div>
              </div>
              <div className={`mt-3 h-2 overflow-hidden rounded-full ${colors.isDark ? 'bg-slate-800' : 'bg-slate-200'}`}>
                <div
                  className="h-full rounded-full bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)] transition-all"
                  style={{ width: `${syncProgressPercent}%` }}
                />
              </div>
              <div className="mt-3 grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
                <div className={`rounded-xl border px-3 py-3 ${colors.isDark ? 'border-slate-700/70 bg-slate-900/70' : 'border-slate-200 bg-white/70'}`}>
                  <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>当前股票</div>
                  <div className={`mt-1 font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                    {syncStatus?.currentSymbol
                      ? `${syncStatus.currentSymbol}${syncStatus.currentName ? ` · ${syncStatus.currentName}` : ''}`
                      : '--'}
                  </div>
                </div>
                <div className={`rounded-xl border px-3 py-3 ${colors.isDark ? 'border-slate-700/70 bg-slate-900/70' : 'border-slate-200 bg-white/70'}`}>
                  <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>已完成 / 总数</div>
                  <div className={`mt-1 font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                    {(syncStatus?.completedStocks ?? 0)} / {(syncStatus?.totalStocks ?? '--')}
                  </div>
                </div>
                <div className={`rounded-xl border px-3 py-3 ${colors.isDark ? 'border-slate-700/70 bg-slate-900/70' : 'border-slate-200 bg-white/70'}`}>
                  <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>当前数据源</div>
                  <div className={`mt-1 font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                    {formatScreeningSourceName(syncStatus?.activeSource)}
                  </div>
                </div>
                <div className={`rounded-xl border px-3 py-3 ${colors.isDark ? 'border-slate-700/70 bg-slate-900/70' : 'border-slate-200 bg-white/70'}`}>
                  <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>当前阶段</div>
                  <div className={`mt-1 font-medium ${colors.isDark ? 'text-slate-100' : 'text-slate-800'}`}>
                    {formatScreeningSyncStage(syncStatus?.currentStage)}
                  </div>
                </div>
              </div>
              {syncRecentEvents.length > 0 && (
                <div className="mt-3 space-y-2">
                  {syncRecentEvents.map((event, index) => (
                    <div
                      key={`${event.time}-${event.symbol}-${event.source}-${index}`}
                      className={`rounded-xl border px-3 py-2 text-xs ${colors.isDark ? 'border-slate-700/70 bg-slate-900/60 text-slate-300' : 'border-slate-200 bg-white/70 text-slate-600'}`}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <span>{event.time || '--'}</span>
                        <span>{formatScreeningSourceName(event.source)}</span>
                      </div>
                      <div className="mt-1">
                        {event.message || syncStatus?.lastMessage || '--'}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {error && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-400">
              {error}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-3 border-t fin-divider-soft px-6 py-4">
          <button
            type="button"
            onClick={onOpenSyncSettings}
            className={`rounded-xl border px-4 py-2 text-sm transition-colors ${
              colors.isDark
                ? 'border-slate-600 bg-slate-800/70 text-slate-200 hover:text-white'
                : 'border-slate-300 bg-white text-slate-600 hover:text-slate-900'
            }`}
          >
            查看同步设置
          </button>
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={isSyncRunning ? onCancelSync : onClose}
              className={`rounded-xl border px-4 py-2 text-sm ${colors.isDark ? 'border-slate-700 text-slate-300 hover:bg-slate-800/70' : 'border-slate-300 text-slate-600 hover:bg-slate-100'} disabled:opacity-50`}
            >
              {isSyncRunning ? '取消同步' : '取消'}
            </button>
            {isScreeningMode && !syncStarted && copy.secondaryActionLabel && (
              <button
                type="button"
                onClick={onSecondaryAction}
                disabled={loading}
                className={`inline-flex items-center gap-2 rounded-xl border px-4 py-2 text-sm font-semibold disabled:opacity-60 ${
                  colors.isDark
                    ? 'border-cyan-500/50 bg-cyan-500/10 text-cyan-200 hover:bg-cyan-500/20'
                    : 'border-cyan-500/40 bg-cyan-50 text-cyan-700 hover:bg-cyan-100'
                }`}
              >
                {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                <span>{copy.secondaryActionLabel}</span>
              </button>
            )}
            <button
              type="button"
              onClick={onConfirm}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)] px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              {loading && <Loader2 className="h-4 w-4 animate-spin" />}
              <span>{loading ? (isScreeningMode ? '处理中...' : '同步中...') : copy.primaryActionLabel}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

const resolveScreeningSyncHealthSummary = (status: ScreeningSyncStatus | null): {
  tone: SyncHealthTone;
  title: string;
  detail: string;
} => {
  const targetTradeDate = status?.targetTradeDate || '';
  const latestSyncedTradeDate = status?.latestSyncedTradeDate || status?.lastTradeDate || '';
  const syncedToLatest = status?.syncedToLatestStocks ?? 0;
  const pendingSync = status?.pendingSyncStocks ?? 0;
  const totalStocks = status?.marketStockCount ?? 0;

  if (!targetTradeDate) {
    return {
      tone: 'danger',
      title: '尚未建立最新同步状态',
      detail: '当前还无法判断最近交易日是否已经同步，请先执行一次同步。',
    };
  }

  if (latestSyncedTradeDate && latestSyncedTradeDate < targetTradeDate) {
    return {
      tone: 'danger',
      title: '需要同步最新交易日数据',
      detail: `最近交易日为 ${formatDateDisplay(targetTradeDate)}，当前库内最新只到 ${formatDateDisplay(latestSyncedTradeDate)}。`,
    };
  }

  if (pendingSync > 0) {
    return {
      tone: 'warning',
      title: '最新交易日只完成了部分同步',
      detail: `最近交易日 ${formatDateDisplay(targetTradeDate)} 已同步 ${syncedToLatest} 只，待同步 ${pendingSync} 只，共 ${totalStocks} 只。`,
    };
  }

  return {
    tone: 'success',
    title: '最新交易日已同步完成',
    detail: `最近交易日 ${formatDateDisplay(targetTradeDate)} 已完成全市场同步，共 ${totalStocks} 只股票。`,
  };
};

const getSyncHealthToneStyles = (tone: SyncHealthTone, isDark: boolean) => {
  if (tone === 'success') {
    return {
      panel: isDark ? 'border-emerald-500/20 bg-emerald-500/10' : 'border-emerald-200 bg-emerald-50',
      icon: isDark ? 'text-emerald-300' : 'text-emerald-600',
      text: isDark ? 'text-emerald-200' : 'text-emerald-700',
      subtext: isDark ? 'text-emerald-100/80' : 'text-emerald-700/80',
    };
  }
  if (tone === 'warning') {
    return {
      panel: isDark ? 'border-amber-500/20 bg-amber-500/10' : 'border-amber-200 bg-amber-50',
      icon: isDark ? 'text-amber-300' : 'text-amber-600',
      text: isDark ? 'text-amber-200' : 'text-amber-700',
      subtext: isDark ? 'text-amber-100/80' : 'text-amber-700/80',
    };
  }
  return {
    panel: isDark ? 'border-red-500/20 bg-red-500/10' : 'border-red-200 bg-red-50',
    icon: isDark ? 'text-red-300' : 'text-red-600',
    text: isDark ? 'text-red-200' : 'text-red-700',
    subtext: isDark ? 'text-red-100/80' : 'text-red-700/80',
  };
};

const resolveSyncDataItemToneClass = (tone: SyncHealthTone | undefined, isDark: boolean): string => {
  if (tone === 'success') {
    return isDark ? 'text-emerald-300' : 'text-emerald-700';
  }
  if (tone === 'warning') {
    return isDark ? 'text-amber-300' : 'text-amber-700';
  }
  if (tone === 'danger') {
    return isDark ? 'text-red-300' : 'text-red-700';
  }
  return isDark ? 'text-slate-200' : 'text-slate-700';
};

const getTopbarSyncButtonToneStyles = (
  tone: SyncButtonTone,
  isDark: boolean,
  disabled: boolean,
): string => {
  const disabledClass = disabled ? 'cursor-not-allowed' : '';
  if (tone === 'success') {
    return `${disabledClass} ${
      isDark
        ? 'border-emerald-500/30 bg-emerald-500/12 text-emerald-200'
        : 'border-emerald-300 bg-emerald-50 text-emerald-700'
    }`;
  }
  if (tone === 'warning') {
    return `${disabledClass} ${
      isDark
        ? 'border-amber-500/30 bg-amber-500/12 text-amber-200 hover:border-amber-400/50 hover:text-amber-100'
        : 'border-amber-300 bg-amber-50 text-amber-700 hover:border-amber-400 hover:text-amber-800'
    }`;
  }
  return `${disabledClass} ${
    isDark
      ? 'border-red-500/30 bg-red-500/12 text-red-200 hover:border-red-400/50 hover:text-red-100'
      : 'border-red-300 bg-red-50 text-red-700 hover:border-red-400 hover:text-red-800'
  }`;
};

const ScreeningSQLDialog: React.FC<ScreeningSQLDialogProps> = ({
  isOpen,
  sql,
  onClose,
}) => {
  if (!isOpen || !sql.trim()) return null;

  return (
    <div className="fixed inset-0 z-[130] flex items-center justify-center bg-slate-950/55 px-4 backdrop-blur-sm">
      <div className="w-full max-w-3xl overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl">
        <div className="flex items-start justify-between gap-4 border-b border-slate-200 px-6 py-5">
          <div>
            <div className="text-lg font-semibold text-slate-800">当前筛选 SQL</div>
            <div className="mt-1 text-sm text-slate-500">只读展示当前筛选结果对应的 SQL 语句。</div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-800"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="px-6 py-5">
          <pre className="max-h-[60vh] overflow-auto rounded-2xl bg-slate-950 px-4 py-4 font-mono text-xs leading-6 text-slate-100">
            <code>{sql}</code>
          </pre>

          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent/90"
            >
              关闭
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

const formatScreeningSyncStage = (stage?: string): string => {
  switch ((stage || '').toLowerCase()) {
    case 'prepare':
      return '准备中';
    case 'syncing':
      return '同步中';
    case 'completed':
      return '已完成';
    case 'failed':
      return '失败';
    case 'canceled':
      return '已取消';
    default:
      return stage || '--';
  }
};

// A股行情数据项组件
interface AStockStatItemProps {
  label: string;
  value: number | string;
  preClose?: number;
  isPlain?: boolean;
  isDark?: boolean;
}

const AStockStatItem: React.FC<AStockStatItemProps> = ({ label, value, preClose, isPlain, isDark = true }) => {
  const cc = useCandleColor();
  let colorClass = isDark ? 'text-slate-100' : 'text-slate-700';
  let displayValue = typeof value === 'string' ? value : value.toFixed(2);

  if (!isPlain && typeof value === 'number' && preClose) {
    if (value > preClose) colorClass = cc.upClass;
    else if (value < preClose) colorClass = cc.downClass;
  }

  return (
    <div className="flex justify-between items-center px-3 py-1.5">
      <span className={isDark ? 'text-slate-500' : 'text-slate-400'}>{label}</span>
      <span className={`font-mono ${colorClass}`}>{displayValue}</span>
    </div>
  );
};

// 格式化成交量
const formatVolume = (vol: number): string => {
  if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿';
  if (vol >= 10000) return (vol / 10000).toFixed(2) + '万';
  return vol.toString();
};

// 格式化成交额
const formatAmount = (amount: number): string => {
  if (amount >= 100000000) return (amount / 100000000).toFixed(2) + '亿';
  if (amount >= 10000) return (amount / 10000).toFixed(2) + '万';
  return amount.toFixed(2);
};

const mapScreeningResultToStock = (result: ScreeningRunResult): Stock => ({
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

const resolveDefaultScreeningSQLTimeoutLabel = (config: FrontendAppConfig): string => {
  const timeout = config.screening?.sqlTimeoutSeconds ?? 0;
  if (timeout <= 0) {
    return '不限';
  }
  return `${timeout} 秒`;
};

const isScreeningQueryCanceledMessage = (message: string): boolean => {
  const normalized = message.toLowerCase();
  return normalized.includes('context canceled') || normalized.includes('cancelled') || normalized.includes('canceled');
};

export default App;
