/**
 * WebSocket管理器 - 处理前端与WebSocket服务器的通信
 */
class AuctionWebSocketManager {
    constructor() {
        this.socket = null;
        this.isConnected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectInterval = 3000; // 3秒
        this.heartbeatInterval = null;
        this.heartbeatTimeout = null;
        this.heartbeatIntervalTime = 30000; // 30秒心跳
        this.heartbeatTimeoutTime = 10000; // 10秒心跳超时（增加超时时间）
        this.messageHandlers = new Map();
        this.connectionCallbacks = [];
        this.isPageVisible = true;
        this.manualDisconnect = false;
        this.connectionInitDelay = 1000; // 延迟1秒初始化连接
        
        // 监听页面可见性变化
        document.addEventListener('visibilitychange', this._handleVisibilityChange.bind(this));
        
        // 监听页面卸载事件
        window.addEventListener('beforeunload', this._handlePageUnload.bind(this));
    }

    /**
     * 连接到WebSocket服务器
     */
    connect() {
        // 如果已经连接，直接返回
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            console.log('WebSocket已连接');
            return;
        }

        // 如果正在连接中，不重复连接
        if (this.socket && this.socket.readyState === WebSocket.CONNECTING) {
            console.log('WebSocket正在连接中...');
            return;
        }

        // 延迟连接，确保页面完全加载
        setTimeout(() => {
            this._doConnect();
        }, this.connectionInitDelay);
    }
    
    /**
     * 实际执行连接操作
     */
    _doConnect() {
        // 确定WebSocket协议
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/auction`;

        console.log(`正在连接到WebSocket服务器: ${wsUrl}`);
        
        try {
            this.manualDisconnect = false;
            this._updateConnectingStatusDisplay();
            this.socket = new WebSocket(wsUrl);

            // 设置事件监听器
            this.socket.onopen = this._onOpen.bind(this);
            this.socket.onmessage = this._onMessage.bind(this);
            this.socket.onclose = this._onClose.bind(this);
            this.socket.onerror = this._onError.bind(this);
        } catch (error) {
            console.error('创建WebSocket连接失败:', error);
            this._updateConnectionStatusDisplay(false);
            this._scheduleReconnect();
        }
    }

    /**
     * 断开WebSocket连接
     */
    disconnect() {
        console.log('手动断开WebSocket连接');
        this.manualDisconnect = true;

        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }

        if (this.heartbeatTimeout) {
            clearTimeout(this.heartbeatTimeout);
            this.heartbeatTimeout = null;
        }

        if (this.socket) {
            // 使用正常关闭码1000
            this.socket.close(1000, '手动断开连接');
            this.socket = null;
        }

        this.isConnected = false;
        this.reconnectAttempts = 0;
    }

    /**
     * 发送消息到WebSocket服务器
     */
    send(message) {
        if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
            console.error('WebSocket未连接，无法发送消息');
            return false;
        }

        try {
            const messageStr = typeof message === 'string' ? message : JSON.stringify(message);
            this.socket.send(messageStr);
            return true;
        } catch (error) {
            console.error('发送WebSocket消息失败:', error);
            return false;
        }
    }

    /**
     * 注册消息处理器
     */
    onMessage(type, handler) {
        if (!this.messageHandlers.has(type)) {
            this.messageHandlers.set(type, []);
        }
        this.messageHandlers.get(type).push(handler);
    }

    /**
     * 移除消息处理器
     */
    offMessage(type, handler) {
        if (this.messageHandlers.has(type)) {
            const handlers = this.messageHandlers.get(type);
            const index = handlers.indexOf(handler);
            if (index > -1) {
                handlers.splice(index, 1);
            }
        }
    }

    /**
     * 添加连接状态变化回调
     */
    onConnectionChange(callback) {
        this.connectionCallbacks.push(callback);
    }

    /**
     * 移除连接状态变化回调
     */
    offConnectionChange(callback) {
        const index = this.connectionCallbacks.indexOf(callback);
        if (index > -1) {
            this.connectionCallbacks.splice(index, 1);
        }
    }

    /**
     * WebSocket连接打开事件处理
     */
    _onOpen(event) {
        console.log('WebSocket连接已建立');
        this.isConnected = true;
        this.isConnecting = false;
        this.reconnectAttempts = 0;

        // 启动心跳
        this._startHeartbeat();

        // 通知连接状态变化
        this._notifyConnectionChange(true);
    }

    /**
     * WebSocket消息接收事件处理
     */
    _onMessage(event) {
        try {
            const message = JSON.parse(event.data);
            console.log('收到WebSocket消息:', message);

            // 处理心跳响应
            if (message.type === 'pong') {
                this._handlePong();
                return;
            }

            // 调用注册的消息处理器
            if (this.messageHandlers.has(message.type)) {
                const handlers = this.messageHandlers.get(message.type);
                handlers.forEach(handler => {
                    try {
                        handler(message.data);
                    } catch (error) {
                        console.error(`处理${message.type}类型消息时出错:`, error);
                    }
                });
            }
        } catch (error) {
            console.error('解析WebSocket消息失败:', error);
        }
    }

    /**
     * WebSocket连接关闭事件处理
     */
    _onClose(event) {
        console.log(`WebSocket连接已关闭: 代码=${event.code}, 原因="${event.reason || '无'}", 是否干净=${event.wasClean}`);
        this.isConnected = false;

        // 清理心跳
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }

        if (this.heartbeatTimeout) {
            clearTimeout(this.heartbeatTimeout);
            this.heartbeatTimeout = null;
        }

        // 通知连接状态变化
        this._notifyConnectionChange(false);

        // 根据关闭码决定是否重连
        if (this.manualDisconnect) {
            console.log('手动断开连接，不尝试重连');
            return;
        }

        // 根据关闭码决定是否重连
        const shouldReconnect = this._shouldReconnect(event.code);
        
        if (shouldReconnect) {
            console.log(`连接异常关闭 (代码: ${event.code}), 将尝试重连`);
            this._scheduleReconnect();
        } else {
            console.log(`连接正常关闭 (代码: ${event.code}), 不进行重连`);
            if (event.code !== 1000) {
                this._notifyConnectionError(`连接关闭: ${this._getCloseCodeDescription(event.code)}`);
            }
        }
    }
    
    /**
     * 判断是否应该重连
     */
    _shouldReconnect(code) {
        // 正常关闭码，不重连
        if (code === 1000) return false;
        
        // 以下关闭码表示异常关闭，应该重连
        const reconnectCodes = [
            1001, // going away
            1005, // no status
            1006, // abnormal closure
            1011, // internal error
            1012, // service restart
            1013, // try again later
            1014, // bad gateway
            1015 // TLS handshake
        ];
        
        return reconnectCodes.includes(code);
    }
    
    /**
     * 获取关闭码描述
     */
    _getCloseCodeDescription(code) {
        const descriptions = {
            1000: '正常关闭',
            1001: '端点离开',
            1002: '协议错误',
            1003: '不支持的数据类型',
            1004: '保留',
            1005: '无状态码',
            1006: '异常关闭',
            1007: '无效数据',
            1008: '策略违规',
            1009: '消息过大',
            1010: '必需扩展',
            1011: '内部错误',
            1012: '服务重启',
            1013: '稍后重试',
            1014: '网关错误',
            1015: 'TLS握手失败'
        };
        
        return descriptions[code] || `未知关闭码: ${code}`;
    }

    /**
     * WebSocket错误事件处理
     */
    _onError(event) {
        console.error('WebSocket错误:', event);
        this.isConnected = false;
        this._notifyConnectionChange(false);
        
        // 根据错误类型进行分类处理
        if (event.message) {
            if (event.message.includes('NetworkError') || event.message.includes('network')) {
                console.error('网络连接错误，可能是网络中断或服务器不可达');
            } else if (event.message.includes('timeout')) {
                console.error('连接超时，可能是服务器响应缓慢');
            } else if (event.message.includes('refused') || event.message.includes('ECONNREFUSED')) {
                console.error('连接被拒绝，可能是服务器未启动或端口被阻止');
            } else {
                console.error('未知WebSocket错误:', event.message);
            }
        } else {
            console.error('WebSocket发生未知错误，无错误详情');
        }
    }

    /**
     * 启动心跳
     */
    _startHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
        }

        this.heartbeatInterval = setInterval(() => {
            if (this.socket && this.socket.readyState === WebSocket.OPEN) {
                this.send({ type: 'ping' });
                
                // 设置心跳超时
                this.heartbeatTimeout = setTimeout(() => {
                    console.warn('心跳超时，关闭连接');
                    this.socket.close();
                }, this.heartbeatTimeoutTime);
            }
        }, this.heartbeatIntervalTime);
    }

    /**
     * 处理心跳响应
     */
    _handlePong() {
        if (this.heartbeatTimeout) {
            clearTimeout(this.heartbeatTimeout);
            this.heartbeatTimeout = null;
        }
    }

    /**
     * 安排重连
     */
    _scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('达到最大重连次数，停止重连');
            // 通知用户连接失败
            this._notifyConnectionError('达到最大重连次数，请检查网络连接后刷新页面');
            return;
        }

        this.reconnectAttempts++;
        // 使用指数退避算法，但限制最大延迟时间
        const baseDelay = this.reconnectInterval;
        const exponentialDelay = Math.min(baseDelay * Math.pow(2, this.reconnectAttempts - 1), 30000);
        const jitter = Math.random() * 1000; // 添加随机抖动避免同时重连
        const delay = exponentialDelay + jitter;
        
        console.log(`${(delay/1000).toFixed(1)}秒后尝试第${this.reconnectAttempts}次重连`);
        
        setTimeout(() => {
            if (!this.isConnected && !this.manualDisconnect) {
                console.log('开始重连尝试...');
                this.connect();
            }
        }, delay);
    }

    /**
     * 更新连接状态显示
     */
    _updateConnectionStatusDisplay(isConnected) {
        const statusDot = document.getElementById('wsStatusDot');
        const statusText = document.getElementById('wsStatusText');
        const statusIndicator = document.getElementById('wsStatusIndicator');
        const reconnectBtn = document.getElementById('wsReconnectBtn');
        
        if (!statusDot || !statusText || !statusIndicator) {
            return; // 元素不存在，可能是其他页面
        }
        
        if (isConnected) {
            // 连接状态
            statusDot.className = 'w-3 h-3 rounded-full bg-green-500';
            statusText.textContent = '已连接';
            statusText.className = 'text-sm font-medium text-green-600';
            statusIndicator.className = 'flex items-center space-x-2 px-3 py-1 rounded-full bg-green-100';
            reconnectBtn.classList.add('hidden');
        } else {
            // 断开状态
            statusDot.className = 'w-3 h-3 rounded-full bg-red-500';
            statusText.textContent = '已断开';
            statusText.className = 'text-sm font-medium text-red-600';
            statusIndicator.className = 'flex items-center space-x-2 px-3 py-1 rounded-full bg-red-100';
            reconnectBtn.classList.remove('hidden');
        }
    }
    
    /**
     * 更新连接中状态显示
     */
    _updateConnectingStatusDisplay() {
        const statusDot = document.getElementById('wsStatusDot');
        const statusText = document.getElementById('wsStatusText');
        const statusIndicator = document.getElementById('wsStatusIndicator');
        const reconnectBtn = document.getElementById('wsReconnectBtn');
        
        if (!statusDot || !statusText || !statusIndicator) {
            return; // 元素不存在，可能是其他页面
        }
        
        // 连接中状态
        statusDot.className = 'w-3 h-3 rounded-full bg-yellow-500 animate-pulse';
        statusText.textContent = '连接中...';
        statusText.className = 'text-sm font-medium text-yellow-600';
        statusIndicator.className = 'flex items-center space-x-2 px-3 py-1 rounded-full bg-yellow-100';
        reconnectBtn.classList.add('hidden');
    }
    
    /**
     * 通知连接状态变化
     */
    _notifyConnectionChange(isConnected) {
        console.log(`通知连接状态变化: ${isConnected ? '已连接' : '已断开'}`);
        this._updateConnectionStatusDisplay(isConnected);
        
        this.connectionCallbacks.forEach(callback => {
            try {
                callback(isConnected);
            } catch (error) {
                console.error('执行连接状态回调时出错:', error);
            }
        });
    }
    
    /**
     * 通知连接错误
     */
    _notifyConnectionError(errorMessage) {
        console.error('连接错误通知:', errorMessage);
        // 可以在这里添加用户通知逻辑，比如显示toast或alert
        if (typeof window.showConnectionError === 'function') {
            window.showConnectionError(errorMessage);
        } else {
            // 备用通知方式
            console.warn('连接错误:', errorMessage);
        }
    }
    
    /**
     * 处理页面可见性变化
     */
    _handleVisibilityChange() {
        const isVisible = !document.hidden;
        console.log(`页面可见性变化: ${isVisible ? '可见' : '隐藏'}`);
        
        this.isPageVisible = isVisible;
        
        // 如果页面变为可见且WebSocket未连接，尝试重连
        if (isVisible && !this.isConnected && !this.manualDisconnect) {
            console.log('页面变为可见，尝试重新连接WebSocket');
            this.reconnectAttempts = 0; // 重置重连尝试次数
            this.connect();
        }
        // 如果页面变为隐藏，暂停心跳但不关闭连接
        else if (!isVisible && this.isConnected) {
            console.log('页面变为隐藏，暂停心跳');
            if (this.heartbeatInterval) {
                clearInterval(this.heartbeatInterval);
                this.heartbeatInterval = null;
            }
            if (this.heartbeatTimeout) {
                clearTimeout(this.heartbeatTimeout);
                this.heartbeatTimeout = null;
            }
        }
        // 如果页面变为可见且WebSocket已连接，恢复心跳
        else if (isVisible && this.isConnected) {
            console.log('页面变为可见，恢复心跳');
            this._startHeartbeat();
        }
    }
    
    /**
     * 处理页面卸载
     */
    _handlePageUnload() {
        console.log('页面即将卸载，断开WebSocket连接');
        this.manualDisconnect = true;
        
        // 同步关闭WebSocket连接，不使用异步操作
        if (this.socket) {
            try {
                this.socket.close(1000, '页面卸载');
            } catch (error) {
                console.error('页面卸载时关闭WebSocket连接失败:', error);
            }
        }
    }
}

// 创建全局WebSocket管理器实例
window.wsManager = new AuctionWebSocketManager();

// 确保在所有资源加载完成后才连接WebSocket
function initializeWebSocket() {
    console.log('初始化WebSocket连接');
    window.wsManager.connect();
}

// 使用多种事件确保连接初始化
if (document.readyState === 'loading') {
    // 如果文档还在加载中，等待DOMContentLoaded事件
    document.addEventListener('DOMContentLoaded', initializeWebSocket);
} else {
    // 如果文档已经加载完成，延迟一点时间再连接
    setTimeout(initializeWebSocket, 500);
}

// 同时监听load事件作为备用
window.addEventListener('load', () => {
    if (!window.wsManager.isConnected) {
        console.log('load事件触发，WebSocket未连接，尝试连接');
        setTimeout(initializeWebSocket, 1000);
    }
});

// 页面卸载时断开连接
window.addEventListener('beforeunload', () => {
    window.wsManager.disconnect();
});

// 添加重新连接按钮事件处理
document.addEventListener('DOMContentLoaded', () => {
    const reconnectBtn = document.getElementById('wsReconnectBtn');
    if (reconnectBtn) {
        reconnectBtn.addEventListener('click', () => {
            console.log('用户点击重新连接按钮');
            window.wsManager.reconnectAttempts = 0; // 重置重连尝试次数
            window.wsManager.connect();
        });
    }
});