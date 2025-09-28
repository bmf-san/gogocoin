// gogocoin Web UI JavaScript

class GogocoinUI {
    constructor() {
        this.currentTab = 'dashboard';
        this.updateInterval = null;
        this.charts = {};

        this.init();
    }

    init() {
        this.setupTabs();
        this.setupEventListeners();
        this.startAutoUpdate();
        this.loadInitialData();
    }

    // Setup tab functionality
    setupTabs() {
        const tabButtons = document.querySelectorAll('.tab-button');
        const tabContents = document.querySelectorAll('.tab-content');

        tabButtons.forEach(button => {
            button.addEventListener('click', () => {
                const tabName = button.dataset.tab;

                // Switch active tab
                tabButtons.forEach(b => b.classList.remove('active'));
                tabContents.forEach(c => c.classList.remove('active'));

                button.classList.add('active');
                document.getElementById(tabName).classList.add('active');

                this.currentTab = tabName;
                this.onTabChange(tabName);
            });
        });
    }

    // Setup event listeners
    setupEventListeners() {
        // Chart hover events
        const canvas = document.getElementById('pnl-chart');
        if (canvas) {
            canvas.addEventListener('mousemove', (e) => this.handleChartHover(e));
            canvas.addEventListener('mouseleave', () => this.hideChartTooltip());
        }

        // Update trade history
        document.getElementById('refresh-trades')?.addEventListener('click', () => {
            this.loadTrades();
        });

        // Log filters
        document.getElementById('log-level-filter')?.addEventListener('change', () => {
            this.loadLogs();
        });
        document.getElementById('log-category-filter')?.addEventListener('change', () => {
            this.loadLogs();
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


        // Change trade history display limit
        document.getElementById('trades-limit')?.addEventListener('change', () => {
            this.loadTrades();
        });

        // Change log display limit
        document.getElementById('logs-limit')?.addEventListener('change', () => {
            this.loadLogs();
        });

        // Change log level
        document.getElementById('logs-level')?.addEventListener('change', () => {
            this.loadLogs();
        });
    }

    // Start auto-update
    startAutoUpdate() {
        // Update every 5 seconds
        this.updateInterval = setInterval(() => {
            if (this.currentTab === 'dashboard') {
                this.loadDashboardData();
                this.loadLogs();
            }
        }, 5000);
    }

    // Tab change handler
    onTabChange(tabName) {
        switch (tabName) {
            case 'dashboard':
                this.loadDashboardData();
                break;
            case 'trades':
                this.loadTrades();
                break;
            case 'logs':
                this.loadLogs();
                break;
        }
    }

    // Load initial data
    async loadInitialData() {
        try {
            console.log('Starting loadInitialData...');
            await this.loadDashboardData();
            console.log('loadDashboardData completed successfully');
            await this.loadLogs();
            console.log('loadLogs completed successfully');
            this.updateStatus('running', 'Running');
        } catch (error) {
            console.error('Failed to load initial data:', error);
            this.updateStatus('error', '接続エラー');
        }
    }

    // Update status
    updateStatus(status, text) {
        const statusIndicator = document.querySelector('.status-indicator');
        const statusText = document.getElementById('status-text');

        if (statusIndicator) {
            statusIndicator.className = `status-indicator ${status === 'running' ? 'status-running' : 'status-stopped'}`;
        }
        if (statusText) {
        statusText.textContent = text;
        }
    }

    // Load dashboard data
    async loadDashboardData() {
        try {
            console.log('Loading dashboard data...');

            // Call each API individually to capture detailed errors
            const results = await Promise.allSettled([
                this.fetchAPI('/api/status'),
                this.fetchAPI('/api/balance'),
                this.fetchAPI('/api/performance'),
                this.fetchAPI('/api/orders?limit=5'),
                this.fetchAPI('/api/trades?limit=50')
            ]);

            const [statusResult, balanceResult, performanceResult, ordersResult, tradesResult] = results;

            // Update only successful data
            if (statusResult.status === 'fulfilled') {
                this.updateSystemStatus(statusResult.value);
            } else {
                console.error('Failed to load status:', statusResult.reason);
            }

            if (balanceResult.status === 'fulfilled') {
                this.updateBalance(balanceResult.value);
            } else {
                console.error('Failed to load balance:', balanceResult.reason);
            }

            if (performanceResult.status === 'fulfilled') {
                this.updatePerformance(performanceResult.value);
                this.updatePerformanceTable(performanceResult.value);
            } else {
                console.error('Failed to load performance:', performanceResult.reason);
            }

            if (ordersResult.status === 'fulfilled') {
                console.log('Orders API response:', ordersResult.value);
                this.updateOrders(ordersResult.value);
            } else {
                console.error('Failed to load orders:', ordersResult.reason);
            }

            if (tradesResult.status === 'fulfilled') {
                console.log('Trades API response:', tradesResult.value);
                this.updateTrades(tradesResult.value);
            } else {
                console.error('Failed to load trades:', tradesResult.reason);
            }

            console.log('Dashboard data loading completed');

        } catch (error) {
            console.error('Failed to load dashboard data:', error);
        }
    }

    // Update system status
    updateSystemStatus(status) {
        const modeEl = document.getElementById('mode');
        const strategyEl = document.getElementById('strategy');
        const totalTradesEl = document.getElementById('total-trades');
        const uptimeEl = document.getElementById('uptime');

        if (modeEl) modeEl.textContent = status.mode || '-';
        if (strategyEl) strategyEl.textContent = status.strategy || '-';
        if (totalTradesEl) totalTradesEl.textContent = status.total_trades || '-';
        if (uptimeEl) uptimeEl.textContent = status.uptime || '-';

        // Update trading state
        const statusIndicator = document.getElementById('status-indicator');
        const statusText = document.getElementById('status-text');
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        if (status.trading_enabled) {
            if (statusIndicator) {
                statusIndicator.classList.add('running');
            }
            if (statusText) statusText.textContent = 'Running';
            if (startBtn) {
                startBtn.disabled = true;
                startBtn.style.display = 'none';
            }
            if (stopBtn) {
                stopBtn.disabled = false;
                stopBtn.style.display = 'inline-block';
            }
        } else {
            if (statusIndicator) {
                statusIndicator.classList.remove('running');
            }
            if (statusText) statusText.textContent = 'Stopped';
            if (startBtn) {
                startBtn.disabled = false;
                startBtn.style.display = 'inline-block';
            }
            if (stopBtn) {
                stopBtn.disabled = true;
                stopBtn.style.display = 'none';
            }
        }
    }

    // Update balance information
    updateBalance(balances) {
        const container = document.getElementById('balance-info');

        if (!balances || balances.length === 0) {
            container.innerHTML = '<div class="text-secondary">No balance information</div>';
            return;
        }

        // Define currency display order (JPY first, then alphabetical)
        const currencyOrder = ['JPY', 'BTC', 'ETH', 'XRP', 'XLM', 'MONA'];

        // Sort balances
        const sortedBalances = [...balances].sort((a, b) => {
            const indexA = currencyOrder.indexOf(a.currency);
            const indexB = currencyOrder.indexOf(b.currency);

            // If both are defined currencies, use defined order
            if (indexA !== -1 && indexB !== -1) {
                return indexA - indexB;
            }
            // If only a is defined, a comes first
            if (indexA !== -1) return -1;
            // If only b is defined, b comes first
            if (indexB !== -1) return 1;
            // If neither is defined, sort alphabetically
            return a.currency.localeCompare(b.currency);
        });

        container.innerHTML = sortedBalances.map(balance => `
            <div class="p-3 bg-body rounded border-l-4 border-primary flex justify-between items-center">
                <span class="font-semibold text-sm">${balance.currency}</span>
                <span class="font-bold">${this.formatNumber(balance.available)}</span>
            </div>
        `).join('');
    }

    // Update performance metrics
    updatePerformance(performance) {
        const totalPnlEl = document.getElementById('total-pnl');
        const winRateEl = document.getElementById('win-rate');

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            // Display default values when no data
            if (totalPnlEl) {
                totalPnlEl.textContent = '¥0';
                totalPnlEl.className = 'performance-value';
            }
            if (winRateEl) {
                winRateEl.textContent = '0%';
                winRateEl.className = 'performance-value';
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
            winRateEl.className = 'text-2xl font-bold';
        }
    }

    // Update order information
    updateOrders(orders) {
        console.log('updateOrders called with:', orders);
        const container = document.getElementById('orders-info');

        if (!orders || orders.length === 0) {
            console.log('No orders found, showing empty message');
            container.innerHTML = '<div class="loading">No recent orders</div>';
            return;
        }

        console.log(`Updating orders display with ${orders.length} orders`);
        container.innerHTML = orders.map(order => `
            <div class="p-3 bg-body rounded border-l-4 ${order.side === 'BUY' ? 'border-success' : 'border-danger'}">
                <div class="flex justify-between items-center mb-2">
                    <span class="font-semibold">${order.symbol}</span>
                    <span class="badge ${order.side === 'BUY' ? 'badge-success' : 'badge-danger'}">${order.side}</span>
                </div>
                <div class="flex justify-between items-center text-sm text-secondary mb-2">
                    <span>Size: ${this.formatNumber(order.size)}</span>
                    <span>¥${this.formatNumber(order.price)}</span>
                </div>
                <div class="text-xs text-secondary">
                    ${this.formatDateTime(order.executed_at)}
                </div>
            </div>
        `).join('');
    }

    // Update trades (called from dashboard data load)
    updateTrades(trades) {
        const tbody = document.querySelector('#trades-table');

        if (!tbody) {
            console.error('Trades table not found');
            return;
        }

            if (!trades || trades.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-secondary">No trade history</td></tr>';
                return;
            }

            tbody.innerHTML = trades.map(trade => `
                <tr>
                <td class="text-sm">${this.formatDateTime(trade.executed_at)}</td>
                <td class="font-semibold">${trade.symbol || trade.product_code || '-'}</td>
                <td><span class="badge ${trade.side === 'BUY' ? 'badge-success' : 'badge-danger'}">${trade.side}</span></td>
                <td class="text-right">${this.formatNumber(trade.size)}</td>
                <td class="text-right font-semibold ${trade.pnl >= 0 ? 'text-success' : 'text-danger'}">
                        ${this.formatCurrency(trade.pnl)}
                    </td>
                </tr>
            `).join('');
    }

    // Load trade history
    async loadTrades() {
        try {
            const limit = document.getElementById('trades-limit')?.value || 50;
            const trades = await this.fetchAPI(`/api/trades?limit=${limit}`);
            this.updateTrades(trades);
        } catch (error) {
            console.error('Failed to load trades:', error);
        }
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

            const logs = await this.fetchAPI(url);

            const container = document.getElementById('logs-container');

            if (!container) {
                console.error('Logs container not found');
                return;
            }

            if (!logs || logs.length === 0) {
                container.innerHTML = '<div class="text-center text-secondary py-4">No logs</div>';
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
                        <span class="text-slate-400">${this.formatTime(log.timestamp)}</span>
                        <span class="${levelColor} font-semibold">[${log.level}]</span>
                        <span class="text-slate-600">${log.category}:</span>
                        <span class="text-slate-800 flex-1">${this.escapeHtml(log.message)}</span>
                </div>
                `;
            }).join('');

        } catch (error) {
            console.error('Failed to load logs:', error);
        }
    }

    // Update PnL history table

    // Update PnL history table
    updatePerformanceTable(performance) {
        console.log('updatePerformanceTable called with:', performance);
        const tbody = document.querySelector('#pnl-history-table');

        if (!tbody) {
            console.error('PnL history table not found');
            return;
        }

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            console.log('No performance data found, showing empty message');
            tbody.innerHTML = '<tr><td colspan="4" class="loading">No PnL history</td></tr>';
            return;
        }

        // Display latest data first (max 10) - API data is already in newest-first order
        const recentData = performance.slice(0, 10);
        console.log(`Updating performance table with ${recentData.length} records`);

        tbody.innerHTML = recentData.map(p => `
            <tr>
                <td class="text-sm">${this.formatDateTime(p.date)}</td>
                <td class="text-right font-semibold ${p.total_pnl >= 0 ? 'text-success' : 'text-danger'}">
                    ${this.formatCurrency(p.total_pnl)}
                </td>
                <td class="text-right">${this.formatPercent(p.win_rate)}</td>
                <td class="text-right">${p.total_trades || 0}</td>
            </tr>
        `).join('');
    }

    // Update PnL chart (old implementation - will be reimplemented when CSS FW is introduced)
    updatePnLChart(performance) {
        const canvas = document.getElementById('pnl-chart');
        if (!canvas) return;

        const ctx = canvas.getContext('2d');
        const width = canvas.width;
        const height = canvas.height;

        // キャンバスをクリア
        ctx.clearRect(0, 0, width, height);

        if (!performance || !Array.isArray(performance) || performance.length === 0) {
            // データがない場合はメッセージを表示
            ctx.fillStyle = '#86868b';
            ctx.font = '16px -apple-system, BlinkMacSystemFont, sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText('損益データがありません', width / 2, height / 2);
            return;
        }

        // データの準備
        const data = performance.map((p, index) => ({
            x: index,
            y: p.total_pnl || 0,
            date: new Date(p.date),
            winRate: p.win_rate || 0,
            trades: p.total_trades || 0
        }));

        if (data.length < 2) {
            ctx.fillStyle = '#86868b';
            ctx.font = '16px -apple-system, BlinkMacSystemFont, sans-serif';
            ctx.textAlign = 'center';
            ctx.fillText('チャート表示には2つ以上のデータポイントが必要です', width / 2, height / 2);
            return;
        }

        // スケール計算
        const margin = { left: 80, right: 20, top: 20, bottom: 40 };
        const chartWidth = width - margin.left - margin.right;
        const chartHeight = height - margin.top - margin.bottom;

        const minY = Math.min(...data.map(d => d.y));
        const maxY = Math.max(...data.map(d => d.y));
        const yRange = maxY - minY || 1; // ゼロ除算を防ぐ

        // Y軸の目盛りを計算（5段階）
        const yTicks = 5;
        const yStep = yRange / (yTicks - 1);

        // 軸を描画
        ctx.strokeStyle = '#d2d2d7';
        ctx.lineWidth = 1;

        // Y軸
        ctx.beginPath();
        ctx.moveTo(margin.left, margin.top);
        ctx.lineTo(margin.left, height - margin.bottom);
        ctx.stroke();

        // Y軸の目盛りとラベル
        ctx.fillStyle = '#86868b';
        ctx.font = '12px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';

        for (let i = 0; i < yTicks; i++) {
            const value = minY + (yStep * i);
            const y = margin.top + chartHeight - ((value - minY) / yRange) * chartHeight;

            // 目盛り線
            ctx.strokeStyle = '#e5e5ea';
            ctx.beginPath();
            ctx.moveTo(margin.left - 5, y);
            ctx.lineTo(margin.left, y);
            ctx.stroke();

            // グリッドライン
            ctx.setLineDash([2, 2]);
            ctx.beginPath();
            ctx.moveTo(margin.left, y);
            ctx.lineTo(width - margin.right, y);
            ctx.stroke();
            ctx.setLineDash([]);

            // ラベル
            ctx.fillText(this.formatCurrency(value), margin.left - 10, y);
        }

        // X軸
        ctx.strokeStyle = '#d2d2d7';
        ctx.beginPath();
        ctx.moveTo(margin.left, height - margin.bottom);
        ctx.lineTo(width - margin.right, height - margin.bottom);
        ctx.stroke();

        // X軸のラベル（データポイント数に応じて表示）
        ctx.fillStyle = '#86868b';
        ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'top';

        // 最大5つのラベルを表示
        const maxLabels = Math.min(5, data.length);
        const labelStep = Math.max(1, Math.floor(data.length / (maxLabels - 1)));

        for (let i = 0; i < data.length; i += labelStep) {
            const d = data[i];
            const x = margin.left + (d.x / (data.length - 1)) * chartWidth;

            // 日付をフォーマット（MM/DD形式）
            const dateStr = `${(d.date.getMonth() + 1).toString().padStart(2, '0')}/${d.date.getDate().toString().padStart(2, '0')}`;

            // 目盛り
            ctx.strokeStyle = '#d2d2d7';
            ctx.beginPath();
            ctx.moveTo(x, height - margin.bottom);
            ctx.lineTo(x, height - margin.bottom + 5);
            ctx.stroke();

            // ラベル
            ctx.fillText(dateStr, x, height - margin.bottom + 8);
        }

        // 最後のデータポイントも必ず表示
        if (data.length > 1) {
            const lastData = data[data.length - 1];
            const lastX = margin.left + chartWidth;
            const dateStr = `${(lastData.date.getMonth() + 1).toString().padStart(2, '0')}/${lastData.date.getDate().toString().padStart(2, '0')}`;

            ctx.strokeStyle = '#d2d2d7';
            ctx.beginPath();
            ctx.moveTo(lastX, height - margin.bottom);
            ctx.lineTo(lastX, height - margin.bottom + 5);
            ctx.stroke();

            ctx.fillText(dateStr, lastX, height - margin.bottom + 8);
        }

        // ゼロライン（強調）
        if (minY < 0 && maxY > 0) {
            const zeroY = margin.top + chartHeight - ((0 - minY) / yRange) * chartHeight;
            ctx.strokeStyle = '#86868b';
            ctx.lineWidth = 1.5;
            ctx.setLineDash([5, 5]);
            ctx.beginPath();
            ctx.moveTo(margin.left, zeroY);
            ctx.lineTo(width - margin.right, zeroY);
            ctx.stroke();
            ctx.setLineDash([]);
            ctx.lineWidth = 1;
        }

        // データライン描画
        ctx.strokeStyle = data[data.length - 1].y >= 0 ? '#34c759' : '#ff3b30';
        ctx.lineWidth = 2;
        ctx.beginPath();

        data.forEach((point, index) => {
            const x = margin.left + (index / (data.length - 1)) * chartWidth;
            const y = margin.top + chartHeight - ((point.y - minY) / yRange) * chartHeight;

            if (index === 0) {
                ctx.moveTo(x, y);
            } else {
                ctx.lineTo(x, y);
            }
        });

        ctx.stroke();

        // データポイント描画とホバーエリア保存
        ctx.fillStyle = data[data.length - 1].y >= 0 ? '#34c759' : '#ff3b30';
        this.chartDataPoints = []; // ホバー用のデータ保存

        data.forEach((point, index) => {
            const x = margin.left + (index / (data.length - 1)) * chartWidth;
            const y = margin.top + chartHeight - ((point.y - minY) / yRange) * chartHeight;

            ctx.beginPath();
            ctx.arc(x, y, 4, 0, 2 * Math.PI);
            ctx.fill();

            // ホバー判定用にデータポイントを保存
            this.chartDataPoints.push({
                x: x,
                y: y,
                data: point
            });
        });

        // チャート情報を保存（ツールチップ用）
        this.chartInfo = {
            margin: margin,
            width: width,
            height: height,
            minY: minY,
            maxY: maxY
        };

        // イベントリスナーが設定されていない場合は設定
        if (!canvas.hasAttribute('data-hover-initialized')) {
            canvas.addEventListener('mousemove', (e) => this.handleChartHover(e));
            canvas.addEventListener('mouseleave', () => this.hideChartTooltip());
            canvas.setAttribute('data-hover-initialized', 'true');
        }

        // 最新値表示
        const latestValue = data[data.length - 1].y;
        ctx.fillStyle = latestValue >= 0 ? '#34c759' : '#ff3b30';
        ctx.font = 'bold 14px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.textAlign = 'left';
        ctx.fillText(`現在: ${this.formatCurrency(latestValue)}`, margin.left + 10, margin.top + 15);
    }

    // チャートホバー処理
    handleChartHover(event) {
        if (!this.chartDataPoints || !this.chartInfo) return;

        const canvas = document.getElementById('pnl-chart');
        const rect = canvas.getBoundingClientRect();
        const mouseX = event.clientX - rect.left;
        const mouseY = event.clientY - rect.top;

        // 最も近いデータポイントを探す
        let closestPoint = null;
        let minDistance = Infinity;

        this.chartDataPoints.forEach(point => {
            const distance = Math.sqrt(
                Math.pow(mouseX - point.x, 2) + Math.pow(mouseY - point.y, 2)
            );
            if (distance < 15 && distance < minDistance) { // 15px以内
                minDistance = distance;
                closestPoint = point;
            }
        });

        if (closestPoint) {
            this.showChartTooltip(event, closestPoint.data);
            canvas.style.cursor = 'pointer';
            } else {
            this.hideChartTooltip();
            canvas.style.cursor = 'default';
        }
    }

    // ツールチップ表示
    showChartTooltip(event, data) {
        let tooltip = document.getElementById('chart-tooltip');
        if (!tooltip) {
            tooltip = document.createElement('div');
            tooltip.id = 'chart-tooltip';
            tooltip.style.cssText = `
                position: absolute;
                background: rgba(0, 0, 0, 0.9);
                color: white;
                padding: 8px 12px;
                border-radius: 6px;
                font-size: 12px;
                pointer-events: none;
                z-index: 1000;
                white-space: nowrap;
                box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
            `;
            document.body.appendChild(tooltip);
        }

        const date = data.date.toLocaleDateString('ja-JP', {
            year: 'numeric',
            month: 'short',
            day: 'numeric'
        });
        const time = data.date.toLocaleTimeString('ja-JP', {
            hour: '2-digit',
            minute: '2-digit'
        });

        tooltip.innerHTML = `
            <div style="font-weight: bold; margin-bottom: 4px;">${date} ${time}</div>
            <div>損益: ${this.formatCurrency(data.y)}</div>
            <div>勝率: ${this.formatPercent(data.winRate)}</div>
            <div>取引数: ${data.trades}回</div>
        `;

        tooltip.style.left = (event.pageX + 10) + 'px';
        tooltip.style.top = (event.pageY - 10) + 'px';
        tooltip.style.display = 'block';
    }

    // ツールチップ非表示
    hideChartTooltip() {
        const tooltip = document.getElementById('chart-tooltip');
        if (tooltip) {
            tooltip.style.display = 'none';
        }
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

        return new Intl.NumberFormat('ja-JP', {
            style: 'currency',
            currency: 'JPY',
            minimumFractionDigits: 0,
            maximumFractionDigits: 0
        }).format(num);
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

    // 値の色を更新するヘルパーメソッド
    updateValueColor(elementId, value) {
        const element = document.getElementById(elementId);
        if (!element) return;

        const num = parseFloat(value);
        if (isNaN(num)) return;

        element.classList.remove('positive', 'negative');
        if (num > 0) {
            element.classList.add('positive');
        } else if (num < 0) {
            element.classList.add('negative');
        }
    }

    // API呼び出し用のヘルパーメソッド
    async fetchAPI(url, method = 'GET', body = null) {
        try {
            const options = {
                method: method,
                headers: {
                    'Content-Type': 'application/json',
                },
            };

            if (body) {
                options.body = JSON.stringify(body);
            }

            const response = await fetch(url, options);

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            return await response.json();
        } catch (error) {
            console.error(`API call failed for ${url}:`, error);
            throw error;
        }
    }

    // Start trading
    async startTrading() {
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        try {
            // ボタンを無効化
            if (startBtn) startBtn.disabled = true;
            if (stopBtn) stopBtn.disabled = true;

            const response = await this.fetchAPI('/api/trading/start', 'POST');

            if (response.status === 'success') {
                console.log('Trading started successfully');
                // ステータスを即座に更新
                this.loadDashboardData();
            } else {
                console.error('Failed to start trading:', response.message);
                alert('取引開始に失敗しました: ' + response.message);
            }
        } catch (error) {
            console.error('Error starting trading:', error);
            alert('取引開始中にエラーが発生しました');
        } finally {
            // ボタンの状態は次のステータス更新で正しく設定される
        }
    }

    // Stop trading
    async stopTrading() {
        const startBtn = document.getElementById('start-trading-btn');
        const stopBtn = document.getElementById('stop-trading-btn');

        try {
            // ボタンを無効化
            if (startBtn) startBtn.disabled = true;
            if (stopBtn) stopBtn.disabled = true;

            const response = await this.fetchAPI('/api/trading/stop', 'POST');

            if (response.status === 'success') {
                console.log('Trading stopped successfully');
                // ステータスを即座に更新
                this.loadDashboardData();
            } else {
                console.error('Failed to stop trading:', response.message);
                alert('取引停止に失敗しました: ' + response.message);
            }
        } catch (error) {
            console.error('Error stopping trading:', error);
            alert('取引停止中にエラーが発生しました');
        } finally {
            // ボタンの状態は次のステータス更新で正しく設定される
        }
    }

}

// アプリケーション初期化
document.addEventListener('DOMContentLoaded', () => {
    new GogocoinUI();
});
