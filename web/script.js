// gogocoin Web UI JavaScript

class GogocoinUI {
    constructor() {
        this.updateInterval = null;
        this.errorCount = 0;
        this.baseUpdateInterval = 5000; // 5 seconds
        this.maxUpdateInterval = 60000; // 60 seconds max
        this.selectedSymbol = 'BTC_JPY'; // Default symbol

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

        // Sidebar navigation
        document.querySelectorAll('.sidebar-link[data-target]').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                // Update active state
                document.querySelectorAll('.sidebar-link').forEach(l => l.classList.remove('active'));
                link.classList.add('active');
                // Close sidebar on mobile
                const toggle = document.getElementById('sidebar-toggle');
                if (toggle) toggle.checked = false;
                // Scroll to section
                const target = document.getElementById(link.dataset.target);
                if (target) {
                    const scrollEl = document.querySelector('.dashboard-main') || document.documentElement;
                    const offset = target.getBoundingClientRect().top - scrollEl.getBoundingClientRect().top + scrollEl.scrollTop;
                    scrollEl.scrollTo({ top: offset, behavior: 'smooth' });
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
                    // ログも更新
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
            await this.loadDashboardData();
            // ダッシュボードにもログが表示されるので初期ロード
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
            // Call each API individually to capture detailed errors
            const results = await Promise.allSettled([
                this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`),
                this.fetchAPI('/api/balance'),
                this.fetchAPI('/api/performance'),
                this.fetchAPI('/api/trades?limit=200')
            ]);

            const [statusResult, balanceResult, performanceResult, tradesResult] = results;

            // Update only successful data (always update what we can)
            if (statusResult.status === 'fulfilled') {
                this.updateSystemStatus(statusResult.value);
            } else {
                console.error('Failed to load status:', statusResult.reason);
            }

            if (balanceResult.status === 'fulfilled') {
                this.updateBalance(balanceResult.value, false);
            } else {
                console.error('Failed to load balance:', balanceResult.reason);
                this.updateBalance([], true);
            }

            const trades = tradesResult.status === 'fulfilled' ? tradesResult.value : [];

            if (performanceResult.status === 'fulfilled') {
                this.updatePerformance(performanceResult.value, false, trades);
                this.updatePerformanceTable(performanceResult.value, false);
            } else {
                console.error('Failed to load performance:', performanceResult.reason);
                this.updatePerformance(null, true, trades);
                this.updatePerformanceTable(null, true);
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
    updateBalance(balances, hasError) {
        const tbody = document.getElementById('balance-table');

        if (!tbody) {
            console.error('balance-table element not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center text-danger py-6">⚠ 取得エラー</td></tr>';
            return;
        }

        if (!balances || balances.length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center text-secondary py-6">残高なし</td></tr>';
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

        tbody.innerHTML = sortedBalances.map(balance => `
            <tr class="hover:bg-base-200 transition-colors">
                <td class="font-bold text-sm uppercase">${this.escapeHtml(balance.currency)}</td>
                <td class="text-right">${this.escapeHtml(this.formatNumber(balance.amount))}</td>
                <td class="text-right text-primary font-semibold">${this.escapeHtml(this.formatNumber(balance.available))}</td>
            </tr>
        `).join('');
    }

    // Update performance metrics
    // trades: raw trade array used to compute today's PnL accurately
    updatePerformance(performance, hasError, trades = []) {
        const totalPnlEl = document.getElementById('total-pnl');
        const winRateEl = document.getElementById('win-rate');
        const todayPnlEl = document.getElementById('today-pnl');

        if (hasError) {
            if (totalPnlEl) { totalPnlEl.textContent = '取得エラー'; totalPnlEl.className = 'text-xl font-bold text-danger'; }
            if (winRateEl) { winRateEl.textContent = '-'; winRateEl.className = 'text-lg font-bold text-secondary'; }
            if (todayPnlEl) { todayPnlEl.textContent = '-'; todayPnlEl.className = 'text-lg font-bold text-secondary'; }
            return;
        }

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            // Display default values when no data
            if (totalPnlEl) {
                totalPnlEl.textContent = '¥0';
                totalPnlEl.className = 'text-4xl font-black';
            }
            if (winRateEl) {
                winRateEl.textContent = '0%';
                winRateEl.className = 'text-2xl font-black';
            }
            if (todayPnlEl) {
                todayPnlEl.textContent = '¥0';
                todayPnlEl.className = 'text-2xl font-black';
            }
            return;
        }

        // Use the latest performance data (first element is the latest)
        const latest = performance[0];

        if (totalPnlEl) {
            totalPnlEl.textContent = this.formatCurrency(latest.total_pnl);
            totalPnlEl.className = latest.total_pnl >= 0 ? 'text-2xl font-bold text-success' : 'text-2xl font-bold text-danger';
        }
        if (winRateEl) {
            winRateEl.textContent = this.formatPercent(latest.win_rate);
            winRateEl.className = 'text-lg font-bold';
        }

        // Calculate today's PnL from actual trade records (JST date match)
        if (todayPnlEl) {
            const jstOffsetMs = 9 * 60 * 60 * 1000;
            const today = new Date(Date.now() + jstOffsetMs).toISOString().split('T')[0];
            let todayPnL = 0;
            let hasTodayTrades = false;
            (trades || []).forEach(t => {
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
                <td class="text-right font-semibold text-sm ${trade.pnl >= 0 ? 'text-success' : 'text-danger'}">
                    ${this.escapeHtml(this.formatCurrency(trade.pnl))}
                </td>
            </tr>
        `).join('');
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

            // 30秒タイムアウト（ログAPIが遅い可能性がある）
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

            // Reverse to show newest first
            const reversedLogs = logs.reverse();

            container.innerHTML = reversedLogs.map(log => {
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

    // Update PnL history table
    updatePerformanceTable(performance, hasError) {
        const tbody = document.querySelector('#pnl-history-table');

        if (!tbody) {
            console.error('PnL history table not found');
            return;
        }

        if (hasError) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4"><span class="text-danger text-sm">⚠ 取得エラー</span></td></tr>';
            return;
        }

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center text-secondary">損益履歴なし</td></tr>';
            return;
        }

        // Display latest data first (max 10) - API data is already in newest-first order
        const recentData = performance.slice(0, 10);

        tbody.innerHTML = recentData.map(p => `
            <tr>
                <td class="text-sm">${this.escapeHtml(this.formatDate(p.date))}</td>
                <td class="text-right font-semibold ${p.total_pnl >= 0 ? 'text-success' : 'text-danger'}">
                    ${this.escapeHtml(this.formatCurrency(p.total_pnl))}
                </td>
                <td class="text-right">${this.escapeHtml(this.formatPercent(p.win_rate))}</td>
                <td class="text-right">${this.escapeHtml(String(p.total_trades || 0))}</td>
            </tr>
        `).join('');
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // Format number用のヘルパーメソッド
    formatNumber(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // -0を0に変換
        if (num === 0) return '0';

        return new Intl.NumberFormat('ja-JP', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 8
        }).format(num);
    }

    // Format currency用のヘルパーメソッド
    formatCurrency(value) {
        if (value === null || value === undefined) return '¥0';
        const num = parseFloat(value);
        if (isNaN(num)) return '¥0';

        // -0を0に変換
        if (num === 0) return '¥0';

        // 市場価格やトレード価格の精度を保つため、小数点以下2桁まで表示
        return new Intl.NumberFormat('ja-JP', {
            style: 'currency',
            currency: 'JPY',
            minimumFractionDigits: 0,
            maximumFractionDigits: 2
        }).format(num);
    }

    // Format fee (手数料) 用のヘルパーメソッド - 小数点以下も表示
    formatFee(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // -0を0に変換
        if (num === 0) return '-';

        // 小数点以下8桁まで表示（暗号通貨の精度に対応）
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

    // Format date用のヘルパーメソッド
    formatDate(dateString) {
        if (!dateString) return '-';
        const date = new Date(dateString);
        return date.toLocaleDateString('ja-JP', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit'
        });
    }

    // Format datetime用のヘルパーメソッド
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

    // Format percentage用のヘルパーメソッド
    formatPercent(value) {
        if (value === null || value === undefined) return '-';
        const num = parseFloat(value);
        if (isNaN(num)) return '-';

        // -0を0に変換
        if (num === 0) return '0%';

        return new Intl.NumberFormat('ja-JP', {
            style: 'percent',
            minimumFractionDigits: 2,
            maximumFractionDigits: 2
        }).format(num / 100);
    }

    // API呼び出し用のヘルパーメソッド
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

        // ボタンを無効化してローディング表示
        startBtn.disabled = true;
        startBtn.textContent = '開始中...';

        try {
            const response = await this.fetchAPI('/api/trading/start', 'POST', null, 10000);
            console.log('Start trading response:', response);

            // 成功 - サーバーからの最新状態を取得してボタンを更新
            try {
                await this.loadDashboardData();
            } catch (dashboardError) {
                // ダッシュボードデータ取得失敗でも、最低限statusだけは取得
                console.error('Dashboard load failed, fetching status only:', dashboardError);
                try {
                    const status = await this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`);
                    this.updateSystemStatus(status);
                } catch (statusError) {
                    // ダッシュボード更新失敗は取引開始の成否とは無関係。ログにとどめる。
                    console.error('Status fetch also failed (trading was started successfully):', statusError);
                }
            }
        } catch (error) {
            console.error('Error starting trading:', error);
            alert('取引開始に失敗しました: ' + error.message);

            // エラー時はボタンを元に戻す
            startBtn.disabled = false;
            startBtn.textContent = '開始';
        }
    }

    // Stop trading
    async stopTrading() {
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (!startBtn || !stopBtn) return;

        // ボタンを無効化してローディング表示
        stopBtn.disabled = true;
        stopBtn.textContent = '停止中...';

        try {
            const response = await this.fetchAPI('/api/trading/stop', 'POST', null, 10000);
            console.log('Stop trading response:', response);

            // 成功 - サーバーからの最新状態を取得してボタンを更新
            try {
                await this.loadDashboardData();
            } catch (dashboardError) {
                // ダッシュボードデータ取得失敗でも、最低限statusだけは取得
                console.error('Dashboard load failed, fetching status only:', dashboardError);
                const status = await this.fetchAPI(`/api/status?symbol=${this.selectedSymbol}`);
                this.updateSystemStatus(status);
            }
        } catch (error) {
            console.error('Error stopping trading:', error);
            alert('取引停止に失敗しました: ' + error.message);

            // エラー時はボタンを元に戻す
            stopBtn.disabled = false;
            stopBtn.textContent = '停止';
        }
    }

}

// アプリケーション初期化
document.addEventListener('DOMContentLoaded', () => {
    new GogocoinUI();
});
