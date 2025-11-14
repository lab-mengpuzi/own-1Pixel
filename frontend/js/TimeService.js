/**
 * 状态页 - 时间服务系统前端脚本
 */
class TimeService {
    constructor() {
        this.generateTabContents();
        this.initTabs();
        this.initEventListeners();
        this.loadInitialData();
    }

    // 动态生成标签页内容
    generateTabContents() {
        // 检查是否已经生成过内容
        const existingContent = document.querySelector('.tab-contents-container');
        if (existingContent) {
            return; // 已存在，不再重复生成
        }
        
        // 创建标签页内容容器
        const tabContentsContainer = document.createElement('div');
        tabContentsContainer.className = 'tab-contents-container';
        
        // 创建时间信息标签页
        const timeInfoTab = this.createTimeInfoTab();
        
        // 创建服务状态标签页
        const statusTab = this.createStatusTab();
        
        // 创建统计信息标签页
        const statsTab = this.createStatsTab();
        
        // 创建熔断器标签页
        const circuitBreakerTab = this.createCircuitBreakerTab();
        
        // 将所有标签页内容添加到容器中
        tabContentsContainer.innerHTML = timeInfoTab + statusTab + statsTab + circuitBreakerTab;
        
        // 将容器插入到导航标签后面
        const navElement = document.querySelector('.border-b.border-gray-200');
        navElement.insertAdjacentElement('afterend', tabContentsContainer);
    }

    // 创建时间信息标签页
    createTimeInfoTab() {
        return `
        <!-- 时间信息标签页 -->
        <div id="time-info" class="tab-content">
            <div class="bg-white rounded-lg shadow-md p-6 mb-6">
                <h2 class="text-xl font-semibold text-gray-800 mb-4">当前时间信息</h2>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">可信时间</h3>
                        <p id="trusted-time" class="text-gray-600 mb-1">加载中...</p>
                        <p id="trusted-timestamp" class="text-sm text-gray-500">时间戳: 加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">系统时间</h3>
                        <p id="system-time" class="text-gray-600 mb-1">加载中...</p>
                        <p class="text-sm text-gray-500">系统当前时间</p>
                    </div>
                </div>
                <div class="mt-6 grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">时间偏移量</h3>
                        <p id="time-offset" class="text-gray-600">加载中...</p>
                        <p class="text-sm text-gray-500">可信基准时间与单调时钟的偏移</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">服务模式</h3>
                        <p id="service-mode" class="text-gray-600">加载中...</p>
                        <p class="text-sm text-gray-500">当前时间服务运行模式</p>
                    </div>
                </div>
                <div class="mt-6">
                    <button id="refresh-time" class="bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded">
                        <i class="fa fa-refresh mr-2"></i>刷新时间
                    </button>
                </div>
            </div>
        </div>`;
    }

    // 创建服务状态标签页
    createStatusTab() {
        return `
        <!-- 服务状态标签页 -->
        <div id="status" class="tab-content hidden">
            <div class="bg-white rounded-lg shadow-md p-6 mb-6">
                <h2 class="text-xl font-semibold text-gray-800 mb-4">时间服务状态</h2>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">初始化状态</h3>
                        <p id="init-status" class="text-gray-600">加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">降级模式</h3>
                        <p id="degraded-status" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6 grid grid-cols-1 md:grid-cols-1 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">最后同步时间</h3>
                        <p id="last-sync-time" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6">
                    <h3 class="text-lg font-medium text-gray-700 mb-2">NTP池状态</h3>
                    <div id="ntp-pool-status">
                        <div class="text-center text-gray-500">
                            <i class="fa fa-spinner fa-spin mr-2"></i>加载NTP池信息...
                        </div>
                    </div>
                </div>
                <div class="mt-6">
                    <button id="refresh-status" class="bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded">
                        <i class="fa fa-refresh mr-2"></i>刷新状态
                    </button>
                </div>
            </div>
        </div>`;
    }

    // 创建统计信息标签页
    createStatsTab() {
        return `
        <!-- 统计信息标签页 -->
        <div id="stats" class="tab-content hidden">
            <div class="bg-white rounded-lg shadow-md p-6 mb-6">
                <h2 class="text-xl font-semibold text-gray-800 mb-4">时间服务统计</h2>
                <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">总同步次数</h3>
                        <p id="total-syncs" class="text-gray-600">加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">成功同步次数</h3>
                        <p id="successful-syncs" class="text-gray-600">加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">失败同步次数</h3>
                        <p id="failed-syncs" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6 grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">偏差</h3>
                        <p id="max-deviation" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6">
                    <button id="refresh-stats" class="bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded">
                        <i class="fa fa-refresh mr-2"></i>刷新统计
                    </button>
                </div>
            </div>
        </div>`;
    }

    // 创建熔断器标签页
    createCircuitBreakerTab() {
        return `
        <!-- 熔断器标签页 -->
        <div id="circuit-breaker" class="tab-content hidden">
            <div class="bg-white rounded-lg shadow-md p-6 mb-6">
                <h2 class="text-xl font-semibold text-gray-800 mb-4">熔断器状态</h2>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">熔断状态</h3>
                        <p id="cb-status" class="text-gray-600">加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">失败计数</h3>
                        <p id="failure-count" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6 grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">成功计数</h3>
                        <p id="success-count" class="text-gray-600">加载中...</p>
                    </div>
                    <div>
                        <h3 class="text-lg font-medium text-gray-700 mb-2">最后失败时间</h3>
                        <p id="last-failure-time" class="text-gray-600">加载中...</p>
                    </div>
                </div>
                <div class="mt-6">
                    <button id="refresh-cb" class="bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded">
                        <i class="fa fa-refresh mr-2"></i>刷新状态
                    </button>
                </div>
            </div>
        </div>`;
    }

    // 初始化标签页
    initTabs() {
        const tabButtons = document.querySelectorAll('.tab-btn');
        
        tabButtons.forEach(button => {
            button.addEventListener('click', () => {
                const tabId = button.getAttribute('data-tab');
                
                // 更新按钮样式
                tabButtons.forEach(btn => {
                    btn.classList.remove('border-blue-500', 'text-blue-600');
                    btn.classList.add('border-transparent', 'text-gray-500');
                });
                button.classList.remove('border-transparent', 'text-gray-500');
                button.classList.add('border-blue-500', 'text-blue-600');
                
                // 获取所有标签页内容
                const tabContents = document.querySelectorAll('.tab-content');
                
                // 隐藏所有标签页内容
                tabContents.forEach(content => {
                    content.classList.add('hidden');
                });
                
                // 显示当前选中的标签页内容
                const currentTab = document.getElementById(tabId);
                if (currentTab) {
                    currentTab.classList.remove('hidden');
                }
                
                // 加载对应数据
                this.loadTabData(tabId);
            });
        });
    }

    // 初始化事件监听器
    initEventListeners() {
        // 刷新时间按钮
        const refreshTimeBtn = document.getElementById('refresh-time');
        if (refreshTimeBtn) {
            refreshTimeBtn.addEventListener('click', () => {
                this.loadTimeInfo();
            });
        }

        // 刷新状态按钮
        const refreshStatusBtn = document.getElementById('refresh-status');
        if (refreshStatusBtn) {
            refreshStatusBtn.addEventListener('click', () => {
                this.loadStatus();
            });
        }

        // 刷新统计按钮
        const refreshStatsBtn = document.getElementById('refresh-stats');
        if (refreshStatsBtn) {
            refreshStatsBtn.addEventListener('click', () => {
                this.loadStats();
            });
        }

        // 刷新熔断器按钮
        const refreshCbBtn = document.getElementById('refresh-cb');
        if (refreshCbBtn) {
            refreshCbBtn.addEventListener('click', () => {
                this.loadCircuitBreakerState();
            });
        }
    }

    // 加载初始数据
    loadInitialData() {
        this.loadTimeInfo();
    }

    // 根据当前标签加载对应数据
    loadTabData(tabId) {
        switch(tabId) {
            case 'time-info':
                this.loadTimeInfo();
                break;
            case 'status':
                this.loadStatus();
                break;
            case 'stats':
                this.loadStats();
                break;
            case 'circuit-breaker':
                this.loadCircuitBreakerState();
                break;
        }
    }

    // 加载时间信息
    async loadTimeInfo() {
        try {
            const response = await fetch('/api/timeservice/time-info');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            
            // 更新时间信息
            document.getElementById('trusted-time').textContent = data.trusted_time;
            document.getElementById('trusted-timestamp').textContent = `时间戳: ${data.trusted_timestamp}`;
            document.getElementById('system-time').textContent = data.system_time;
            document.getElementById('time-offset').textContent = this.formatNanoseconds(data.sync_time_offset);
            
            // 更新服务模式
            const modeElement = document.getElementById('service-mode');
            if (data.is_degraded) {
                modeElement.textContent = '降级模式';
                modeElement.classList.add('text-orange-600', 'font-medium');
            } else {
                modeElement.textContent = '正常模式';
                modeElement.classList.add('text-green-600', 'font-medium');
            }
        } catch (error) {
            console.error('加载时间信息失败:', error);
            this.showError('加载时间信息失败: ' + error.message);
        }
    }

    // 加载服务状态
    async loadStatus() {
        try {
            const response = await fetch('/api/timeservice/status');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            
            // 更新状态信息
            const initStatus = document.getElementById('init-status');
            initStatus.textContent = data.is_initialized ? '已初始化' : '未初始化';
            initStatus.className = data.is_initialized ? 'text-green-600 font-medium' : 'text-red-600 font-medium';
            
            const degradedStatus = document.getElementById('degraded-status');
            degradedStatus.textContent = data.is_degraded ? '降级模式' : '正常模式';
            degradedStatus.className = data.is_degraded ? 'text-orange-600 font-medium' : 'text-green-600 font-medium';
            
            document.getElementById('last-sync-time').textContent = data.last_sync_time;
            
            // 加载NTP池状态
            this.loadNTPPoolStatus();
        } catch (error) {
            console.error('加载服务状态失败:', error);
            this.showError('加载服务状态失败: ' + error.message);
        }
    }

    // 加载NTP池状态
    async loadNTPPoolStatus() {
        try {
            const response = await fetch('/api/timeservice/ntp-pool');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            
            // 更新NTP池状态
            const ntpPoolStatus = document.getElementById('ntp-pool-status');
            if (data.ntp_servers && data.ntp_servers.length > 0) {
                let html = '<div class="space-y-2">';
                
                // 显示所有NTP服务器
                data.ntp_servers.forEach((server, index) => {
                    const serverTypeClass = server.is_domestic ? 'text-green-600' : 'text-blue-600';
                    const selectedClass = server.is_selected ? 'text-green-600' : 'text-gray-400';
                    const selectedText = server.is_selected ? '已选中' : '未选中';
                    
                    html += `
                        <div class="border rounded-lg overflow-hidden">
                            <div class="ntp-server-item bg-white hover:bg-gray-50 p-3 cursor-pointer transition-colors" data-index="${index}">
                                <div class="flex justify-between items-center">
                                    <div class="flex items-center">
                                        <span class="font-medium text-gray-900 mr-3">${server.name}</span>
                                        <span class="text-sm text-gray-600">${server.address}</span>
                                    </div>
                                    <div class="flex items-center">
                                        <span class="text-sm text-gray-600 mr-3">权重: ${server.weight}</span>
                                        <span class="${serverTypeClass} text-sm mr-2">${server.is_domestic ? '国内' : '国外'}</span>
                                        <span class="${selectedClass} text-sm mr-3">${selectedText}</span>
                                        <i class="fa fa-chevron-down text-gray-400 transition-transform ntp-expand-icon"></i>
                                    </div>
                                </div>
                            </div>
                            <div class="ntp-server-details bg-gray-50 border-t hidden" id="ntp-details-${index}">
                                <div class="p-4 grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div>
                                        <h4 class="font-semibold text-sm text-gray-700 mb-2">基本信息</h4>
                                        <div class="space-y-1 text-sm">
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">服务器名称:</span>
                                                <span>${server.name}</span>
                                            </div>
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">服务器地址:</span>
                                                <span>${server.address}</span>
                                            </div>
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">权重:</span>
                                                <span>${server.weight}</span>
                                            </div>
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">服务器类型:</span>
                                                <span class="${serverTypeClass}">${server.is_domestic ? '国内服务器' : '国外服务器'}</span>
                                            </div>
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">选中状态:</span>
                                                <span class="${selectedClass}">${server.is_selected ? '已选中用于时间同步' : '未选中'}</span>
                                            </div>
                                        </div>
                                    </div>
                                    <div>
                                        <h4 class="font-semibold text-sm text-gray-700 mb-2">连接详情</h4>
                                        <div class="space-y-1 text-sm">
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">上次同步:</span>
                                                <span>${server.last_sync_time || '未知'}</span>
                                            </div>
                                            <div class="flex justify-between">
                                                <span class="text-gray-600">最大偏差:</span>
                                                <span>${server.max_deviation ? (server.max_deviation / 1e9).toFixed(7) + 's' : '未知'}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                ${server.samples && server.samples.length > 0 ? `
                                <div class="p-4 border-t">
                                    <h4 class="font-semibold text-sm text-gray-700 mb-2">样本数据 (最近${server.samples.length}次测量)</h4>
                                    <div class="overflow-x-auto">
                                        <table class="min-w-full text-sm">
                                            <thead>
                                                <tr class="bg-gray-100">
                                                    <th class="px-2 py-1 text-left">序号</th>
                                                    <th class="px-2 py-1 text-left">状态</th>
                                                    <th class="px-2 py-1 text-left">时间戳</th>
                                                    <th class="px-2 py-1 text-left">往返延迟</th>
                                                    <th class="px-2 py-1 text-left">时间偏移</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                ${server.samples.map((sample, idx) => `
                                                    <tr class="${idx % 2 === 0 ? 'bg-white' : 'bg-gray-50'}">
                                                        <td class="px-2 py-1">${idx + 1}</td>
                                                        <td class="px-2 py-1">
                                                            ${sample.status === 'Success' ? 
                                                                '<span class="text-green-600 font-medium">Success</span>' : 
                                                                '<span class="text-red-600 font-medium">Failed</span>'}
                                                        </td>
                                                        <td class="px-2 py-1">${new Date(sample.timestamp / 1e6).toLocaleString()}</td>
                                                        <td class="px-2 py-1">${this.formatNanoseconds(sample.delay)}</td>
                                                        <td class="px-2 py-1">${this.formatNanoseconds(sample.offset)}</td>
                                                    </tr>
                                                `).join('')}
                                            </tbody>
                                        </table>
                                    </div>
                                </div>
                                ` : '<p class="p-4 text-sm text-gray-500">暂无样本数据</p>'}
                            </div>
                        </div>
                    `;
                });
                
                html += '</div>';
                ntpPoolStatus.innerHTML = html;
                
                // 添加点击事件监听器
                document.querySelectorAll('.ntp-server-item').forEach(item => {
                    item.addEventListener('click', function() {
                        const index = this.getAttribute('data-index');
                        const details = document.getElementById(`ntp-details-${index}`);
                        const icon = this.querySelector('.ntp-expand-icon');
                        
                        if (details.classList.contains('hidden')) {
                            // 展开详情
                            details.classList.remove('hidden');
                            icon.classList.add('rotate-180');
                        } else {
                            // 收起详情
                            details.classList.add('hidden');
                            icon.classList.remove('rotate-180');
                        }
                    });
                });
            } else {
                ntpPoolStatus.innerHTML = '<div class="text-gray-500">没有可用的NTP服务器</div>';
            }
        } catch (error) {
            console.error('加载NTP池状态失败:', error);
            const ntpPoolStatus = document.getElementById('ntp-pool-status');
            ntpPoolStatus.innerHTML = `<div class="text-red-500">加载失败: ${error.message}</div>`;
        }
    }

    // 加载统计信息
    async loadStats() {
        try {
            const response = await fetch('/api/timeservice/stats');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            
            // 更新统计信息
            document.getElementById('total-syncs').textContent = data.total_syncs;
            document.getElementById('successful-syncs').textContent = data.successful_syncs;
            document.getElementById('failed-syncs').textContent = data.failed_syncs;
            document.getElementById('max-deviation').textContent = this.formatNanoseconds(data.max_deviation);
        } catch (error) {
            console.error('加载统计信息失败:', error);
            this.showError('加载统计信息失败: ' + error.message);
        }
    }

    // 加载熔断器状态
    async loadCircuitBreakerState() {
        try {
            const response = await fetch('/api/timeservice/circuit-breaker');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            
            // 更新熔断器状态
            const cbStatus = document.getElementById('cb-status');
            cbStatus.textContent = data.is_open ? '已熔断' : '正常';
            cbStatus.className = data.is_open ? 'text-red-600 font-medium' : 'text-green-600 font-medium';
            
            document.getElementById('failure-count').textContent = data.failure_count;
            document.getElementById('success-count').textContent = data.success_count;
            document.getElementById('last-failure-time').textContent = data.last_failure_time;
        } catch (error) {
            console.error('加载熔断器状态失败:', error);
            this.showError('加载熔断器状态失败: ' + error.message);
        }
    }

    // 格式化纳秒为可读格式
    formatNanoseconds(nanoseconds) {
        // 处理 null、undefined 或 NaN 的情况
        if (nanoseconds === null || nanoseconds === undefined || isNaN(nanoseconds)) {
            return '未知';
        }
        
        if (nanoseconds === 0) return '0';
        
        const absNs = Math.abs(nanoseconds);
        let value, unit;
        
        if (absNs < 1000) {
            value = nanoseconds;
            unit = 'ns';
        } else if (absNs < 1000000) {
            value = nanoseconds / 1000;
            unit = 'μs';
        } else if (absNs < 1000000000) {
            value = nanoseconds / 1000000;
            unit = 'ms';
        } else {
            value = nanoseconds / 1000000000;
            unit = 's';
        }
        
        // 对于秒单位，使用7位小数精度，格式为2.0000000
        if (unit === 's') {
            return `${value.toFixed(7)} ${unit}`;
        }
        
        // 对于较大的值，使用更合适的精度
        let precision = 2;
        if (unit === 's' && absNs >= 10000000000) { // 10秒以上
            precision = 1;
        }
        
        return `${value.toFixed(precision)} ${unit}`;
    }

    // 显示成功消息
    showSuccess(message) {
        this.showToast(message, 'success');
    }

    // 显示错误消息
    showError(message) {
        this.showToast(message, 'error');
    }

    // 显示提示消息
    showToast(message, type = 'info') {
        // 创建提示元素
        const toast = document.createElement('div');
        toast.className = `fixed top-4 right-4 p-4 rounded-md shadow-lg z-50 max-w-sm ${
            type === 'success' ? 'bg-green-500 text-white' : 
            type === 'error' ? 'bg-red-500 text-white' : 
            'bg-blue-500 text-white'
        }`;
        toast.textContent = message;
        
        // 添加到页面
        document.body.appendChild(toast);
        
        // 3秒后移除
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, 3000);
    }
}

// 页面加载完成后初始化时间服务
document.addEventListener('DOMContentLoaded', () => {
    new TimeService();
});