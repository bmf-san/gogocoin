// gogocoin Web UI JavaScript

class GogocoinUI {
    constructor() {
        this.updateInterval = null;
        this.errorCount = 0;
        this.baseUpdateInterval = 5000; // 5 seconds
        this.maxUpdateInterval = 60000; // 60 seconds max
        this.selectedSymbol = 'BTC_JPY'; // Default symbol
        this.initialBalance = null;
        this.lastMonitoringPrices = {};

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.startAutoUpdate();
        this.loadInitialData();
        this.setupCleanup();
    }

    // Setup cleanup on page unload
    setupCleanup() {
        window.addEventListener('beforeunload', () => {
            this.cleanup();
        });
    }

    // Cleanup all timers
    cleanup() {
        if (this.updateInterval) {
            clearTimeout(this.updateInterval);
            this.updateInterval = null;
        }
    }

    // Setup event listeners
    setupEventListeners() {
        // Log filters
        document.getElementById('log-level-filter')?.addEventListener('change', () => {
            this.loadLogs();
        });
        document.getElementById('log-category-filter')?.addEventListener('change', () => {
            this.loadLogs();
        });

        // Sidebar navigation - page switching
        document.querySelectorAll('.sidebar-link[data-target]').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const pageId = link.dataset.target;
                // Update active state
                document.querySelectorAll('.sidebar-link').forEach(l => l.classList.remove('active'));
                link.classList.add('active');
                // Close sidebar on mobile
                const toggle = document.getElementById('sidebar-toggle');
                if (toggle) toggle.checked = false;
                // Show target page, hide others
                document.querySelectorAll('.page-section').forEach(p => p.classList.add('hidden'));
                const page = document.getElementById(pageId);
                if (page) {
                    page.classList.remove('hidden');
                    const scrollEl = document.querySelector('.dashboard-main');
                    if (scrollEl) scrollEl.scrollTo({ top: 0 });
                }
            });
        });

        // Trading control button event listeners
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (startBtn) {
            startBtn.addEventListener('click', () => this.startTrading());
        }

        if (stopBtn) {
            stopBtn.addEventListener('click', () => this.stopTrading());
        }
    }

    // Start auto-update with exponential backoff on errors
    startAutoUpdate() {
        const scheduleNextUpdate = () => {
            // Calculate interval with exponential backoff
            const interval = Math.min(
                this.baseUpdateInterval * Math.pow(2, this.errorCount),
                this.maxUpdateInterval
            );

            this.updateInterval = setTimeout(async () => {
                try {
                    await this.loadDashboardData();
                    // Also refresh logs
                    this.loadLogs().catch(err => {
                        console.error('Failed to load logs:', err);
                    });
                    // Reset error count on success
                    this.errorCount = 0;
                } catch (error) {
                    // Increment error count for backoff
                    this.errorCount = Math.min(this.errorCount + 1, 5); // Max 5 for 160s interval
                    console.error('Dashboard update failed, backing off:', error);
                }
                scheduleNextUpdate();
            }, interval);
        };

        scheduleNextUpdate();
    }

    // Load initial data
    async loadInitialData() {
        try {
            // Best-effort: fetch retention setting so the "総損益" caption can
            // honestly reflect that it represents only the retained window.
            this.fetchAPI('/api/config').then(cfg => {
                const days = (cfg && cfg.data_retention && cfg.data_retention.retention_days)
                    || (cfg && cfg.DataRetention && cfg.DataRetention.RetentionDays);
                const initialBalance = (cfg && cfg.trading && cfg.trading.initial_balance)
                    || (cfg && cfg.Trading && cfg.Trading.InitialBalance);
                if (Number.isFinite(Number(initialBalance))) {
                    this.initialBalance = Number(initialBalance);
                }
                if (days && Number.isFinite(days)) {
                    this.retentionDays = days;
                    const cap = document.getElementById('total-pnl-caption');
                    if (cap) cap.textContent = `DB保持${days}日`; 
                }
            }).catch(() => {
                /* optional feature — keep default caption on failure */
            });

            await this.loadDashboardData();
            // Dashboard also shows logs, so load on init
            this.loadLogs().catch(err => {
                console.error('Failed to load initial logs:', err);
            });
        } catch (error) {
            console.error('Failed to load initial data:', error);
        }
    }



    // Load dashboard data
    async loadDashboardData() {
        try {
            // Call each API individually to capture detailed errors.
            // Trades are fetched three times:
            //   - limit=200: recent rows for the trade list & table
            //   - since=today: all of today's rows for accurate "本日の損益"
            //     (the limit=200 slice can truncate heavy scalping days).
            //   - limit=2000: broader window to reconstruct open position for
            //     current unrealized PnL estimation.
            const results = await Promise.allSettled([
                this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`),
                this.fetchAPI('/api/balance'),
                this.fetchAPI('/api/performance'),
                this.fetchAPI('/api/trades?limit=200'),
                this.fetchAPI('/api/trades?since=today'),
                this.fetchAPI('/api/trades?limit=2000'),
                this.fetchAPI(`/api/trades?since=${encodeURIComponent(new Date(Date.now() - (30 * 24 * 60 * 60 * 1000)).toISOString())}`)
            ]);

            const [statusResult, balanceResult, performanceResult, tradesResult, todayTradesResult, wideTradesResult, trades30dResult] = results;

            // Update only successful data (always update what we can)
            if (statusResult.status === 'fulfilled') {
                this.updateSystemStatus(statusResult.value);
            } else {
                console.error('Failed to load status:', statusResult.reason);
            }

            const trades = tradesResult.status === 'fulfilled' ? tradesResult.value : [];
            const todayTrades = todayTradesResult.status === 'fulfilled' ? todayTradesResult.value : trades;
            const wideTrades = wideTradesResult.status === 'fulfilled' ? wideTradesResult.value : trades;
            const trades30d = trades30dResult.status === 'fulfilled' ? trades30dResult.value : [];

            const statusData = statusResult.status === 'fulfilled' ? statusResult.value : null;
            const monitoringPrices = statusData && statusData.monitoring_prices ? statusData.monitoring_prices : {};

            if (balanceResult.status === 'fulfilled') {
                this.updateBalance(balanceResult.value, false, monitoringPrices);
            } else {
                console.error('Failed to load balance:', balanceResult.reason);
                this.updateBalance([], true, monitoringPrices);
            }

            const balanceData = balanceResult.status === 'fulfilled' ? balanceResult.value : [];

            if (performanceResult.status === 'fulfilled') {
                this.updatePerformance(performanceResult.value, false, todayTrades, statusData, wideTrades, trades30d, balanceData);
                this.updatePerformanceTable(performanceResult.value, false, trades30d);
                this.updateAssetHistoryTable(performanceResult.value, false);
                this.updateOpenPositionsTable(wideTrades, statusData, false);
            } else {
                console.error('Failed to load performance:', performanceResult.reason);
                this.updatePerformance(null, true, todayTrades, statusData, wideTrades, trades30d, balanceData);
                this.updatePerformanceTable(null, true, trades30d);
                this.updateAssetHistoryTable(null, true);
                this.updateOpenPositionsTable(wideTrades, statusData, true);
            }

            if (tradesResult.status === 'fulfilled') {
                this.updateTrades(trades, false);
            } else {
                console.error('Failed to load trades:', tradesResult.reason);
                this.updateTrades([], true);
            }

            // Check if status API failed (critical for backoff logic)
            if (statusResult.status === 'rejected') {
                throw new Error('Failed to load status');
            }

        } catch (error) {
            console.error('Failed to load dashboard data:', error);
            throw error; // Re-throw to trigger backoff
        }
    }

    // Update system status
    updateSystemStatus(status) {
        if (!status) {
            console.error('updateSystemStatus called with null/undefined status');
            return;
        }

        const credentialsStatusEl = document.getElementById('credentials-status');
        const strategyEl = document.getElementById('strategy');
        const totalTradesEl = document.getElementById('total-trades');
        const uptimeEl = document.getElementById('uptime');

        if (credentialsStatusEl) credentialsStatusEl.textContent = status.credentials_status || '-';
        if (strategyEl) strategyEl.textContent = status.strategy || '-';
        if (totalTradesEl) totalTradesEl.textContent = status.total_trades || '-';
        if (uptimeEl) uptimeEl.textContent = status.uptime || '-';

        // Update trading status badge and buttons
        const tradingStatusBadge = document.getElementById('trading-status-badge');
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (tradingStatusBadge) {
            const statusText = tradingStatusBadge.querySelector('p.font-semibold');
            if (statusText) {
                if (status.trading_enabled) {
                    statusText.textContent = '取引中';
                    statusText.className = 'text-sm font-semibold leading-tight text-success';
                } else {
                    statusText.textContent = '停止中';
                    statusText.className = 'text-sm font-semibold leading-tight';
                }
            }
        }

        // Update button visibility based on trading status
        if (startBtn && stopBtn) {
            if (status.trading_enabled) {
                // Trading is active - show stop button
                startBtn.disabled = false;
                startBtn.textContent = '開始';
                startBtn.classList.add('hidden');
                stopBtn.disabled = false;
                stopBtn.textContent = '停止';
                stopBtn.classList.remove('hidden');
            } else {
                // Trading is stopped - show start button
                startBtn.disabled = false;
                startBtn.textContent = '開始';
                startBtn.classList.remove('hidden');
                stopBtn.disabled = false;
                stopBtn.textContent = '停止';
                stopBtn.classList.add('hidden');
            }
        }

        // Update monitoring info
        const monitoringPricesContainer = document.getElementById('monitoring-prices');

        if (monitoringPricesContainer && status.monitoring_prices) {
            const priceEntries = Object.entries(status.monitoring_prices).map(([symbol, price]) => {
                const displaySymbol = this.escapeHtml(symbol.replace('_', '/'));
                const formattedPrice = this.escapeHtml(this.formatCurrency(price));
                return `
                    <div class="flex justify-between items-baseline py-1 border-b border-slate-200 last:border-b-0">
                        <span class="text-sm font-bold">${displaySymbol}</span>
                        <span class="text-lg font-black text-primary">${formattedPrice}</span>
                    </div>
                `;
            }).join('');

            monitoringPricesContainer.innerHTML = priceEntries || '<div class="text-center text-secondary py-2 text-xs">価格情報なし</div>';
        }
    }

    // Update balance information
    updateBalance(balances, hasError, monitoringPrices = {}) {
        const tbody = document.getElementById('balance-table');

        if (!tbody) {
            console.error('balance-table element not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-danger py-6">⚠ 取得エラー</td></tr>';
            return;
        }

        if (!balances || balances.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-secondary py-6">残高なし</td></tr>';
            return;
        }

        // Define currency display order (JPY first, then major cryptos, then rest)
        const currencyOrder = ['JPY', 'BTC', 'ETH', 'XRP', 'XLM', 'MONA'];

        // Filter to non-zero balances, plus always show JPY/BTC/ETH/XRP even if zero
        const alwaysShow = new Set(['JPY', 'BTC', 'ETH', 'XRP']);
        const filtered = balances.filter(b => b.amount > 0 || alwaysShow.has(b.currency));

        const sortedBalances = [...filtered].sort((a, b) => {
            const indexA = currencyOrder.indexOf(a.currency);
            const indexB = currencyOrder.indexOf(b.currency);
            if (indexA !== -1 && indexB !== -1) return indexA - indexB;
            if (indexA !== -1) return -1;
            if (indexB !== -1) return 1;
            return a.currency.localeCompare(b.currency);
        });

        const valuationRows = sortedBalances.map(balance => {
            const currency = String(balance.currency || '').toUpperCase();
            const amount = Number(balance.amount);
            let jpyValue = null;

            if (currency === 'JPY') {
                jpyValue = amount;
            } else {
                const px = Number(monitoringPrices[`${currency}_JPY`]);
                if (Number.isFinite(px) && px > 0) {
                    jpyValue = amount * px;
                }
            }

            return { balance, jpyValue };
        });

        const totalValuation = valuationRows.reduce((sum, row) => {
            return sum + (Number.isFinite(row.jpyValue) ? row.jpyValue : 0);
        }, 0);

        tbody.innerHTML = valuationRows.map(({ balance, jpyValue }) => {
            const ratio = Number.isFinite(jpyValue) && totalValuation > 0 ? (jpyValue / totalValuation) * 100 : null;
            return `
            <tr class="hover:bg-base-200 transition-colors">
                <td class="font-bold text-sm uppercase">${this.escapeHtml(balance.currency)}</td>
                <td class="text-right">${this.escapeHtml(this.formatNumber(balance.amount))}</td>
                <td class="text-right text-primary font-semibold">${this.escapeHtml(this.formatNumber(balance.available))}</td>
                <td class="text-right">${Number.isFinite(jpyValue) ? this.escapeHtml(this.formatCurrency(jpyValue)) : '<span class="text-secondary">-</span>'}</td>
                <td class="text-right">${ratio == null ? '<span class="text-secondary">-</span>' : this.escapeHtml(this.formatPercent(ratio))}</td>
            </tr>
        `;
        }).join('');
    }

    // Update performance metrics
    // todayTrades: raw same-day trades for realized PnL
    // allTrades: recent trades used to estimate current open-position unrealized PnL
    updatePerformance(performance, hasError, todayTrades = [], status = null, allTrades = [], trades30d = [], balances = []) {
        const totalPnlEl = document.getElementById('total-pnl');
        const netPnlEl = document.getElementById('net-pnl');
        const realizedAssetEl = document.getElementById('realized-asset');
        const realizedAssetCaptionEl = document.getElementById('realized-asset-caption');
        const accountAssetEl = document.getElementById('account-asset');
        const accountAssetCaptionEl = document.getElementById('account-asset-caption');
        const winRateEl = document.getElementById('win-rate');
        const todayPnlEl = document.getElementById('today-pnl');
        const todayUnrealizedPnlEl = document.getElementById('today-unrealized-pnl');
        const todayUnrealizedCaptionEl = document.getElementById('today-unrealized-caption');
        const sharpeEl = document.getElementById('sharpe-ratio');
        const profitFactorEl = document.getElementById('profit-factor');
        const maxDrawdownEl = document.getElementById('max-drawdown');

        if (hasError) {
            if (totalPnlEl) { totalPnlEl.textContent = '取得エラー'; totalPnlEl.className = 'text-xl font-bold text-danger'; }
            if (netPnlEl) { netPnlEl.textContent = '-'; netPnlEl.className = 'text-lg font-bold text-secondary'; }
            if (realizedAssetEl) { realizedAssetEl.textContent = '-'; realizedAssetEl.className = 'text-lg font-bold text-secondary'; }
            if (realizedAssetCaptionEl) { realizedAssetCaptionEl.textContent = '初期資金 + 累計実現損益'; }
            if (accountAssetEl) { accountAssetEl.textContent = '-'; accountAssetEl.className = 'text-lg font-bold text-secondary'; }
            if (accountAssetCaptionEl) { accountAssetCaptionEl.textContent = 'JPY + 保有資産の時価'; }
            if (winRateEl) { winRateEl.textContent = '-'; winRateEl.className = 'text-lg font-bold text-secondary'; }
            if (todayPnlEl) { todayPnlEl.textContent = '-'; todayPnlEl.className = 'text-lg font-bold text-secondary'; }
            if (todayUnrealizedPnlEl) { todayUnrealizedPnlEl.textContent = '-'; todayUnrealizedPnlEl.className = 'text-lg font-bold text-secondary'; }
            if (todayUnrealizedCaptionEl) { todayUnrealizedCaptionEl.textContent = '建玉ベース'; }
            if (sharpeEl) { sharpeEl.textContent = '-'; }
            if (profitFactorEl) { profitFactorEl.textContent = '-'; }
            if (maxDrawdownEl) { maxDrawdownEl.textContent = '-'; }
            return;
        }

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            // Display default values when no data
            if (totalPnlEl) {
                totalPnlEl.textContent = '¥0';
                totalPnlEl.className = 'text-4xl font-black';
            }
            if (netPnlEl) {
                netPnlEl.textContent = '¥0';
                netPnlEl.className = 'text-2xl font-black';
            }
            if (realizedAssetEl) {
                realizedAssetEl.textContent = '¥0';
                realizedAssetEl.className = 'text-2xl font-black';
            }
            if (realizedAssetCaptionEl) {
                realizedAssetCaptionEl.textContent = '初期資金 + 累計実現損益';
            }
            if (accountAssetEl) {
                accountAssetEl.textContent = '¥0';
                accountAssetEl.className = 'text-2xl font-black';
            }
            if (accountAssetCaptionEl) {
                accountAssetCaptionEl.textContent = 'JPY + 保有資産の時価';
            }
            if (winRateEl) {
                winRateEl.textContent = '0%';
                winRateEl.className = 'text-2xl font-black';
            }
            if (todayPnlEl) {
                todayPnlEl.textContent = '¥0';
                todayPnlEl.className = 'text-2xl font-black';
            }
            if (todayUnrealizedPnlEl) {
                todayUnrealizedPnlEl.textContent = '¥0';
                todayUnrealizedPnlEl.className = 'text-2xl font-black';
            }
            if (todayUnrealizedCaptionEl) {
                todayUnrealizedCaptionEl.textContent = '建玉なし';
            }
            if (sharpeEl) { sharpeEl.textContent = '-'; }
            if (profitFactorEl) { profitFactorEl.textContent = '-'; }
            if (maxDrawdownEl) { maxDrawdownEl.textContent = '-'; }
            return;
        }

        // Use the latest performance data (first element is the latest)
        const latest = performance[0];
        const realized30d = (trades30d || []).reduce((sum, t) => {
            const v = Number(t && t.pnl);
            return Number.isFinite(v) ? sum + v : sum;
        }, 0);
        const has30dData = Array.isArray(trades30d) && trades30d.length > 0;

        const realizedBase = Number(latest.total_pnl) || 0;

        if (totalPnlEl) {
            totalPnlEl.textContent = this.formatCurrency(realizedBase);
            totalPnlEl.className = realizedBase >= 0 ? 'text-2xl font-bold text-success' : 'text-2xl font-bold text-danger';
        }

        if (realizedAssetEl) {
            const strategicAsset = Number.isFinite(this.initialBalance)
                ? (this.initialBalance + (Number(latest.total_pnl) || 0))
                : null;

            if (strategicAsset == null) {
                realizedAssetEl.textContent = '-';
                realizedAssetEl.className = 'text-lg font-bold text-secondary';
                if (realizedAssetCaptionEl) {
                    realizedAssetCaptionEl.textContent = '初期資金を取得中...';
                }
            } else {
                realizedAssetEl.textContent = this.formatCurrency(strategicAsset);
                realizedAssetEl.className = 'text-2xl font-bold';
                if (realizedAssetCaptionEl) {
                    const days = Number(this.retentionDays);
                    if (Number.isFinite(days) && days > 0) {
                        realizedAssetCaptionEl.textContent = `初期資金 + 累計実現損益(DB保持${days}日)`;
                    } else {
                        realizedAssetCaptionEl.textContent = '初期資金 + 累計実現損益';
                    }
                }
            }
        }

        if (accountAssetEl) {
            const valuation = this.computeAccountValuation(balances, status && status.monitoring_prices ? status.monitoring_prices : {});
            accountAssetEl.textContent = this.formatCurrency(valuation.total);
            accountAssetEl.className = 'text-2xl font-bold';

            if (accountAssetCaptionEl) {
                if (valuation.missingCount > 0) {
                    accountAssetCaptionEl.textContent = `JPY + 保有資産の時価(未評価${valuation.missingCount}銘柄)`;
                } else {
                    accountAssetCaptionEl.textContent = 'JPY + 保有資産の時価';
                }
            }
        }
        if (winRateEl) {
            winRateEl.textContent = this.formatPercent(latest.win_rate);
            winRateEl.className = 'text-lg font-bold';
        }
        if (sharpeEl) {
            const v = latest.sharpe_ratio;
            const vValid = v != null && !isNaN(v);
            sharpeEl.textContent = vValid ? parseFloat(v).toFixed(2) : '-';
            sharpeEl.className = 'text-2xl font-bold' + (vValid ? (v >= 1.0 ? ' text-success' : ' text-danger') : '');
        }
        if (profitFactorEl) {
            const v = latest.profit_factor;
            const vValid = v != null && !isNaN(v);
            profitFactorEl.textContent = vValid ? parseFloat(v).toFixed(2) : '-';
            profitFactorEl.className = 'text-2xl font-bold' + (vValid ? (v >= 1.5 ? ' text-success' : ' text-danger') : '');
        }
        if (maxDrawdownEl) {
            const v = latest.max_drawdown;
            const vValid = v != null && !isNaN(v);
            maxDrawdownEl.textContent = vValid ? parseFloat(v).toFixed(2) + '%' : '-';
            maxDrawdownEl.className = 'text-2xl font-bold' + (vValid ? (v <= 10 ? ' text-success' : ' text-danger') : '');
        }

        let unrealizedTotal = 0;

        // Calculate today's PnL from actual trade records (JST date match)
        if (todayPnlEl) {
            const jstOffsetMs = 9 * 60 * 60 * 1000;
            const today = new Date(Date.now() + jstOffsetMs).toISOString().split('T')[0];
            let todayPnL = 0;
            let hasTodayTrades = false;
            (todayTrades || []).forEach(t => {
                if (!t.executed_at) return;
                const jstDate = new Date(new Date(t.executed_at).getTime() + jstOffsetMs)
                    .toISOString().split('T')[0];
                if (jstDate === today && t.pnl !== undefined && t.pnl !== null) {
                    todayPnL += t.pnl;
                    hasTodayTrades = true;
                }
            });
            if (hasTodayTrades) {
                todayPnlEl.textContent = this.formatCurrency(todayPnL);
                todayPnlEl.className = todayPnL >= 0 ? 'text-lg font-bold text-success' : 'text-lg font-bold text-danger';
            } else {
                todayPnlEl.textContent = '¥0';
                todayPnlEl.className = 'text-lg font-bold';
            }
        }

        // Estimate current unrealized PnL from reconstructed open positions.
        if (todayUnrealizedPnlEl) {
            const sourceTrades = (allTrades && allTrades.length > 0) ? allTrades : (todayTrades || []);
            const positions = this.buildOpenPositionsFromTrades(sourceTrades);
            const prices = status && status.monitoring_prices ? status.monitoring_prices : {};

            let openCount = 0;
            let missingPriceCount = 0;

            Object.entries(positions).forEach(([symbol, pos]) => {
                if (!pos || !(pos.qty > 0)) return;
                openCount++;

                const currentPrice = Number(prices[symbol]);
                if (!Number.isFinite(currentPrice) || currentPrice <= 0) {
                    missingPriceCount++;
                    return;
                }

                unrealizedTotal += (currentPrice - pos.avgPrice) * pos.qty;
            });

            todayUnrealizedPnlEl.textContent = this.formatCurrency(unrealizedTotal);
            todayUnrealizedPnlEl.className = unrealizedTotal >= 0 ? 'text-lg font-bold text-success' : 'text-lg font-bold text-danger';

            if (todayUnrealizedCaptionEl) {
                if (openCount === 0) {
                    todayUnrealizedCaptionEl.textContent = '建玉なし';
                } else if (missingPriceCount > 0) {
                    todayUnrealizedCaptionEl.textContent = `建玉 ${openCount}銘柄(うち${missingPriceCount}銘柄は価格未取得)`;
                } else {
                    todayUnrealizedCaptionEl.textContent = `建玉 ${openCount}銘柄`;
                }
            }
        }

        if (netPnlEl) {
            const net = realizedBase + unrealizedTotal;
            netPnlEl.textContent = this.formatCurrency(net);
            netPnlEl.className = net >= 0 ? 'text-lg font-bold text-success' : 'text-lg font-bold text-danger';
        }

        const totalPnlCaptionEl = document.getElementById('total-pnl-caption');
        if (totalPnlCaptionEl) {
            const days = Number(this.retentionDays);
            totalPnlCaptionEl.textContent = Number.isFinite(days) && days > 0
                ? `DB保持${days}日`
                : 'DB保持期間内';
        }
    }

    updateOpenPositionsTable(allTrades = [], status = null, hasError = false) {
        const tbody = document.getElementById('open-positions-table');
        if (!tbody) return;

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center py-4"><span class="text-danger text-sm">⚠ 取得エラー</span></td></tr>';
            return;
        }

        const positions = this.buildOpenPositionsFromTrades(allTrades || []);
        const prices = status && status.monitoring_prices ? status.monitoring_prices : {};
        const rows = Object.entries(positions)
            .filter(([, pos]) => pos && pos.qty > 0)
            .sort((a, b) => b[1].qty - a[1].qty);

        if (rows.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center text-secondary">建玉なし</td></tr>';
            return;
        }

        tbody.innerHTML = rows.map(([symbol, pos]) => {
            const currentPrice = Number(prices[symbol]);
            const hasPrice = Number.isFinite(currentPrice) && currentPrice > 0;
            const unrealized = hasPrice ? (currentPrice - pos.avgPrice) * pos.qty : null;
            const changePct = hasPrice && pos.avgPrice > 0 ? ((currentPrice - pos.avgPrice) / pos.avgPrice) * 100 : null;

            return `
            <tr>
                <td class="font-semibold text-sm">${this.escapeHtml(symbol)}</td>
                <td class="text-right text-sm">${this.escapeHtml(this.formatNumber(pos.qty))}</td>
                <td class="text-right text-sm">${this.escapeHtml(this.formatCurrency(pos.avgPrice))}</td>
                <td class="text-right text-sm">${hasPrice ? this.escapeHtml(this.formatCurrency(currentPrice)) : '<span class="text-secondary">-</span>'}</td>
                <td class="text-right font-semibold ${unrealized == null ? 'text-secondary' : (unrealized >= 0 ? 'text-success' : 'text-danger')}">
                    ${unrealized == null ? '-' : this.escapeHtml(this.formatCurrency(unrealized))}
                </td>
                <td class="text-right ${changePct == null ? 'text-secondary' : (changePct >= 0 ? 'text-success' : 'text-danger')}">
                    ${changePct == null ? '-' : this.escapeHtml(this.formatPercent(changePct))}
                </td>
            </tr>
        `;
        }).join('');
    }

    computeAccountValuation(balances = [], monitoringPrices = {}) {
        let total = 0;
        let missingCount = 0;

        (balances || []).forEach(b => {
            if (!b) return;
            const currency = String(b.currency || '').toUpperCase();
            const amount = Number(b.amount);
            if (!Number.isFinite(amount) || amount <= 0) return;

            if (currency === 'JPY') {
                total += amount;
                return;
            }

            const symbol = `${currency}_JPY`;
            const px = Number(monitoringPrices[symbol]);
            if (!Number.isFinite(px) || px <= 0) {
                missingCount += 1;
                return;
            }

            total += amount * px;
        });

        return { total, missingCount };
    }

    // Reconstruct current long positions from trade executions.
    // BUY increases position and recalculates weighted average entry;
    // SELL reduces quantity (average entry remains for remaining lots).
    buildOpenPositionsFromTrades(trades = []) {
        const positions = {};

        const normalized = [...(trades || [])]
            .filter(t => t && t.symbol && (t.side === 'BUY' || t.side === 'SELL'))
            .map(t => ({
                ...t,
                _size: Number(t.size),
                _price: Number(t.price),
                _ts: Date.parse(t.executed_at || t.created_at || 0)
            }))
            .filter(t => Number.isFinite(t._size) && t._size > 0 && Number.isFinite(t._price) && t._price > 0)
            .sort((a, b) => a._ts - b._ts);

        normalized.forEach(t => {
            if (!positions[t.symbol]) {
                positions[t.symbol] = { qty: 0, avgPrice: 0 };
            }

            const pos = positions[t.symbol];

            if (t.side === 'BUY') {
                const nextQty = pos.qty + t._size;
                pos.avgPrice = nextQty > 0
                    ? ((pos.avgPrice * pos.qty) + (t._price * t._size)) / nextQty
                    : 0;
                pos.qty = nextQty;
                return;
            }

            // SELL: reduce only up to existing quantity.
            const reduceQty = Math.min(pos.qty, t._size);
            pos.qty -= reduceQty;
            if (pos.qty <= 0) {
                pos.qty = 0;
                pos.avgPrice = 0;
            }
        });

        return positions;
    }

    // Update trades (called from dashboard data load)
    updateTrades(trades, hasError) {
        const tbody = document.querySelector('#trades-table');

        if (!tbody) {
            console.error('Trades table not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="7" class="text-center py-4"><span class="text-danger text-sm">⚠ 取得エラー</span></td></tr>';
            return;
        }

        if (!trades || trades.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="text-center text-secondary">取引履歴なし</td></tr>';
            return;
        }

        tbody.innerHTML = trades.map(trade => `
            <tr>
                <td class="text-xs text-secondary">${this.escapeHtml(this.formatShortDateTime(trade.executed_at))}</td>
                <td class="font-semibold text-sm">${this.escapeHtml(trade.symbol || trade.product_code || '-')}</td>
                <td><span class="badge ${trade.side === 'BUY' ? 'badge-success' : 'badge-danger'}">${this.escapeHtml(trade.side)}</span></td>
                <td class="text-right text-sm">${this.escapeHtml(this.formatCurrency(trade.price))}</td>
                <td class="text-right text-sm">${this.escapeHtml(this.formatNumber(trade.size))}</td>
                <td class="text-right text-xs">${this.escapeHtml(this.formatFee(trade.fee))}</td>
                <td class="text-right font-semibold text-sm ${this.escapeHtml(this.tradePnlClass(trade))}" title="${this.escapeHtml(this.tradePnlTitle(trade))}">
                    ${this.escapeHtml(this.tradePnlText(trade))}
                </td>
            </tr>
        `).join('');
    }

    // For BUY rows, trade.pnl mostly reflects entry fee at fill time.
    // For SELL rows, trade.pnl reflects realized round-trip impact.
    tradePnlText(trade) {
        if (!trade) return '-';
        const side = String(trade.side || '').toUpperCase();
        const pnl = Number(trade.pnl);

        if (side === 'BUY') {
            return '-';
        }

        return Number.isFinite(pnl) ? this.formatCurrency(pnl) : '-';
    }

    tradePnlClass(trade) {
        if (!trade) return 'text-secondary';
        const side = String(trade.side || '').toUpperCase();
        if (side === 'BUY') return 'text-secondary';

        const pnl = Number(trade.pnl);
        if (!Number.isFinite(pnl)) return 'text-secondary';
        return pnl >= 0 ? 'text-success' : 'text-danger';
    }

    tradePnlTitle(trade) {
        if (!trade) return '';
        const side = String(trade.side || '').toUpperCase();
        if (side === 'BUY') {
            return 'BUYは建玉作成のため実現損益は表示しない';
        }
        return 'SELLは実現損益(手数料込み)';
    }

    // Load logs
    async loadLogs() {
        try {
            const level = document.getElementById('log-level-filter')?.value || '';
            const category = document.getElementById('log-category-filter')?.value || '';

            // Build query parameters
            let url = `/api/logs?limit=50`;
            if (level) {
                url += `&level=${level}`;
            }
            if (category) {
                url += `&category=${category}`;
            }

            // 30s timeout (log API may be slow)
            const logs = await this.fetchAPI(url, 'GET', null, 30000);

            const container = document.getElementById('logs-container');

            if (!container) {
                console.error('Logs container not found');
                return;
            }

            if (!logs || logs.length === 0) {
                container.innerHTML = '<div class="text-center text-secondary py-4">ログなし</div>';
                return;
            }

            container.innerHTML = logs.map(log => {
                const levelClass = log.level.toLowerCase();
                const levelColor = {
                    'debug': 'text-slate-500',
                    'info': 'text-blue-600',
                    'warn': 'text-yellow-600',
                    'error': 'text-red-600'
                }[levelClass] || 'text-slate-600';

                return `
                    <div class="flex gap-2 p-1 hover:bg-slate-50">
                        <span class="text-slate-400">${this.escapeHtml(this.formatTime(log.timestamp))}</span>
                        <span class="${levelColor} font-semibold">[${this.escapeHtml(log.level)}]</span>
                        <span class="text-slate-600">${this.escapeHtml(log.category)}:</span>
                        <span class="text-slate-800 flex-1">${this.escapeHtml(log.message)}</span>
                </div>
                `;
            }).join('');

        } catch (error) {
            console.error('Failed to load logs:', error);
        }
    }

    // Update daily realized PnL table.
    // tradesForDaily: trade rows used for day aggregation.
    // Exchange-style semantics: realized PnL and win-rate are based on
    // closing executions (SELL in current long-only strategy).
    updatePerformanceTable(performance, hasError, tradesForDaily = []) {
        const tbody = document.querySelector('#pnl-history-table');

        if (!tbody) {
            console.error('PnL history table not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4"><span class="text-danger text-sm">⚠ 取得エラー</span></td></tr>';
            return;
        }

        // Prefer trade-based aggregation when available: this is the most
        // accurate source for day-level realized PnL.
        const jstOffsetMs = 9 * 60 * 60 * 1000;
        const perDay = {};
        (tradesForDaily || []).forEach(t => {
            if (!t || !t.executed_at) return;
            if (String(t.side || '').toUpperCase() !== 'SELL') return;
            const d = new Date(new Date(t.executed_at).getTime() + jstOffsetMs).toISOString().split('T')[0];
            if (!perDay[d]) {
                perDay[d] = { pnl: 0, trades: 0, wins: 0 };
            }
            const pnl = Number(t.pnl);
            if (Number.isFinite(pnl)) {
                perDay[d].pnl += pnl;
                if (pnl > 0.01) perDay[d].wins += 1;
            }
            perDay[d].trades += 1;
        });

        const days = Object.keys(perDay).sort((a, b) => (a < b ? 1 : -1)).slice(0, 10);
        if (days.length > 0) {
            tbody.innerHTML = days.map(d => {
                const row = perDay[d];
                const dailyWinRate = row.trades > 0 ? (row.wins / row.trades) * 100 : 0;
                return `
            <tr>
                <td class="text-sm">${this.escapeHtml(d)}</td>
                <td class="text-right font-semibold ${row.pnl >= 0 ? 'text-success' : 'text-danger'}">
                    ${this.escapeHtml(this.formatCurrency(row.pnl))}
                </td>
                <td class="text-right">${this.escapeHtml(this.formatPercent(dailyWinRate))}</td>
                <td class="text-right">${this.escapeHtml(String(row.trades))}</td>
            </tr>
        `;
            }).join('');
            return;
        }

        // Fallback: if no trade rows are available, keep snapshot-diff behavior.
        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-secondary">損益履歴なし</td></tr>';
            return;
        }

        const byDate = {};
        for (const p of performance) {
            const jstDate = new Date(new Date(p.date).getTime() + jstOffsetMs).toISOString().split('T')[0];
            if (!byDate[jstDate] || new Date(p.date) > new Date(byDate[jstDate].date)) {
                byDate[jstDate] = p;
            }
        }

        const sorted = Object.keys(byDate)
            .sort((a, b) => (a < b ? 1 : -1))
            .map(d => ({ ...byDate[d], _jstDate: d }));

        const recentData = sorted.slice(0, 10);
        tbody.innerHTML = recentData.map((p, i) => {
            const prev = sorted[i + 1];
            if (!prev) {
                return `
            <tr>
                <td class="text-sm">${this.escapeHtml(p._jstDate)}</td>
                <td class="text-right text-secondary">—</td>
                <td class="text-right text-secondary">—</td>
                <td class="text-right text-secondary">—</td>
            </tr>
        `;
            }
            const dailyPnL = p.total_pnl - prev.total_pnl;
            const dailyTrades = (p.total_trades || 0) - (prev.total_trades || 0);
            const dailyWinning = (p.winning_trades || 0) - (prev.winning_trades || 0);
            const dailyWinRate = dailyTrades > 0 ? (dailyWinning / dailyTrades) * 100 : 0;
            return `
            <tr>
                <td class="text-sm">${this.escapeHtml(p._jstDate)}</td>
                <td class="text-right font-semibold ${dailyPnL >= 0 ? 'text-success' : 'text-danger'}">
                    ${this.escapeHtml(this.formatCurrency(dailyPnL))}
                </td>
                <td class="text-right">${this.escapeHtml(this.formatPercent(dailyWinRate))}</td>
                <td class="text-right">${this.escapeHtml(String(dailyTrades))}</td>
            </tr>
        `;
        }).join('');
    }

    // Update total asset trend table.
    // Asset is computed on realized basis: initial_balance + cumulative realized pnl.
    updateAssetHistoryTable(performance, hasError) {
        const tbody = document.querySelector('#asset-history-table');
        if (!tbody) {
            console.error('Asset history table not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center py-4"><span class="text-danger text-sm">⚠ 取得エラー</span></td></tr>';
            return;
        }

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center text-secondary">資産履歴なし</td></tr>';
            return;
        }

        if (!Number.isFinite(this.initialBalance)) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center text-secondary">初期資金を取得中...</td></tr>';
            return;
        }

        const jstOffsetMs = 9 * 60 * 60 * 1000;
        const byDate = {};
        for (const p of performance) {
            const jstDate = new Date(new Date(p.date).getTime() + jstOffsetMs).toISOString().split('T')[0];
            if (!byDate[jstDate] || new Date(p.date) > new Date(byDate[jstDate].date)) {
                byDate[jstDate] = p;
            }
        }

        const sorted = Object.keys(byDate)
            .sort((a, b) => (a < b ? 1 : -1))
            .map(d => ({ ...byDate[d], _jstDate: d }))
            .slice(0, 10);

        tbody.innerHTML = sorted.map((p, i) => {
            const asset = this.initialBalance + (Number(p.total_pnl) || 0);
            const older = sorted[i + 1];
            const dayDiff = older
                ? asset - (this.initialBalance + (Number(older.total_pnl) || 0))
                : null;

            return `
            <tr>
                <td class="text-sm">${this.escapeHtml(p._jstDate)}</td>
                <td class="text-right font-semibold">${this.escapeHtml(this.formatCurrency(asset))}</td>
                <td class="text-right ${dayDiff == null ? 'text-secondary' : (dayDiff >= 0 ? 'text-success' : 'text-danger')}">
                    ${dayDiff == null ? '—' : this.escapeHtml(this.formatCurrency(dayDiff))}
                </td>
            </tr>
        `;
        }).join('');
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Format number helper
    formatNumber(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // Convert -0 to 0
        if (num === 0) return '0';

        return new Intl.NumberFormat('ja-JP', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 8
        }).format(num);
    }

    // Format currency helper
    formatCurrency(value) {
        if (value === null || value === undefined) return '¥0';
        const num = parseFloat(value);
        if (isNaN(num)) return '¥0';

        // Convert -0 to 0
        if (num === 0) return '¥0';

        // Show up to 2 decimal places for market/trade price precision
        return new Intl.NumberFormat('ja-JP', {
            style: 'currency',
            currency: 'JPY',
            minimumFractionDigits: 0,
            maximumFractionDigits: 2
        }).format(num);
    }

    // Format fee helper - show decimal places
    formatFee(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // Convert -0 to 0
        if (num === 0) return '-';

        // Show up to 8 decimal places (for crypto precision)
        return new Intl.NumberFormat('ja-JP', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 8
        }).format(num);
    }

    // Format short datetime (MM/DD HH:mm) for compact table display
    formatShortDateTime(dateString) {
        if (!dateString) return '-';
        const date = new Date(dateString);
        return date.toLocaleString('ja-JP', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit'
        });
    }

    // Format date helper
    formatDate(dateString) {
        if (!dateString) return '-';
        const date = new Date(dateString);
        return date.toLocaleDateString('ja-JP', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit'
        });
    }

    // Format datetime helper
    formatDateTime(dateString) {
        if (!dateString) return '-';
        const date = new Date(dateString);
        return date.toLocaleString('ja-JP', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
    }

    // Format time only (for compact display)
    formatTime(dateString) {
        if (!dateString) return '-';
        const date = new Date(dateString);
        return date.toLocaleTimeString('ja-JP', {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
    }

    // Format percentage helper
    formatPercent(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // Convert -0 to 0
        if (num === 0) return '0%';

        return new Intl.NumberFormat('ja-JP', {
            style: 'percent',
            minimumFractionDigits: 2,
            maximumFractionDigits: 2
        }).format(num / 100);
    }

    // API call helper
    async fetchAPI(url, method = 'GET', body = null, timeout = 10000) {
        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), timeout);

            const options = {
                method: method,
                headers: {
                    'Content-Type': 'application/json',
                },
                signal: controller.signal
            };

            if (body) {
                options.body = JSON.stringify(body);
            }

            const response = await fetch(url, options);
            clearTimeout(timeoutId);

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const data = await response.json();
            return data;
        } catch (error) {
            if (error.name === 'AbortError') {
                console.error(`API call timeout for ${url}`);
                throw new Error('Request timeout');
            }
            console.error(`API call failed for ${url}:`, error);
            throw error;
        }
    }

    // Start trading
    async startTrading() {
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (!startBtn || !stopBtn) return;

        // Disable button and show loading
        startBtn.disabled = true;
        startBtn.textContent = '開始中...';

        try {
            const response = await this.fetchAPI('/api/trading/start', 'POST', null, 10000);
            console.log('Start trading response:', response);

            // Success - fetch latest state from server and update buttons
            try {
                await this.loadDashboardData();
            } catch (dashboardError) {
                // Dashboard load failed; try to at least fetch status
                console.error('Dashboard load failed, fetching status only:', dashboardError);
                try {
                    const status = await this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`);
                    this.updateSystemStatus(status);
                } catch (statusError) {
                    // Dashboard update failure is unrelated to trading start result. Log only.
                    console.error('Status fetch also failed (trading was started successfully):', statusError);
                }
            }
        } catch (error) {
            console.error('Error starting trading:', error);
            alert('取引開始に失敗しました: ' + error.message);

            // Restore button on error
            startBtn.disabled = false;
            startBtn.textContent = '開始';
        }
    }

    // Stop trading
    async stopTrading() {
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (!startBtn || !stopBtn) return;

        // Disable button and show loading
        stopBtn.disabled = true;
        stopBtn.textContent = '停止中...';

        try {
            const response = await this.fetchAPI('/api/trading/stop', 'POST', null, 10000);
            console.log('Stop trading response:', response);

            // Success - fetch latest state from server and update buttons
            try {
                await this.loadDashboardData();
            } catch (dashboardError) {
                // Dashboard load failed; try to at least fetch status
                console.error('Dashboard load failed, fetching status only:', dashboardError);
                try {
                    const status = await this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`);
                    this.updateSystemStatus(status);
                } catch (statusError) {
                    // Dashboard update failure is unrelated to trading stop result. Log only.
                    console.error('Status fetch also failed (trading was stopped successfully):', statusError);
                }
            }
        } catch (error) {
            console.error('Error stopping trading:', error);
            alert('取引停止に失敗しました: ' + error.message);

            // Restore button on error
            stopBtn.disabled = false;
            stopBtn.textContent = '停止';
        }
    }

}

// Initialize application
document.addEventListener('DOMContentLoaded', () => {
    new GogocoinUI();
});
