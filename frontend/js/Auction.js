/**
 * 拍卖页 - 荷兰钟拍卖系统前端脚本
 */
document.addEventListener('DOMContentLoaded', () => {
    // 初始化页面
    loadBalance();
    loadAuctions();
    loadSellerAuctions();

    // 设置WebSocket消息处理器
    function setupWebSocketHandlers() {
        // 检查WebSocket管理器是否可用
        if (!window.wsManager) {
            console.error('WebSocket管理器未找到');
            return;
        }

        // 注册拍卖更新消息处理器
        window.wsManager.onMessage('auction_update', (data) => {
            console.log('收到拍卖更新:', data);
            handleAuctionUpdate(data);
        });

        // 注册价格更新消息处理器
        window.wsManager.onMessage('auction_price_update', (data) => {
            console.log('收到价格更新:', data);
            handleAuctionPriceUpdate(data);
        });

        // 注册拍卖状态更新消息处理器
        window.wsManager.onMessage('auction_status', (data) => {
            console.log('收到拍卖状态更新:', data);
            handleAuctionStatusUpdate(data);
        });

        // 注册连接状态变化处理器
        window.wsManager.onConnectionChange((isConnected) => {
            console.log(`WebSocket连接状态: ${isConnected ? '已连接' : '已断开'}`);

            // 连接恢复时重新加载数据
            if (isConnected) {
                loadAuctions();
                loadSellerAuctions();
                loadBalance();
            }
        });
    }

    // 处理拍卖更新
    function handleAuctionUpdate(data) {
        if (!data || !data.auction) return;

        const auction = data.auction;

        // 更新买家区域的拍卖卡片
        updateAuctionCard(auction, 'buyer');

        // 更新卖家区域的拍卖卡片
        updateAuctionCard(auction, 'seller');

        // 显示通知
        if (data.action === 'started') {
            showNotification(`拍卖 #${auction.id} 已开始`);
        } else if (data.action === 'cancelled') {
            showNotification(`拍卖 #${auction.id} 已取消`);
        } else if (data.action === 'paused') {
            showNotification(`拍卖 #${auction.id} 已暂停`);
        } else if (data.action === 'ended') {
            showNotification(`拍卖 #${auction.id} 已结束`);
        }
    }

    // 处理价格更新
    function handleAuctionPriceUpdate(data) {
        if (!data) return;

        const auctionId = data.auctionId;
        const oldPrice = data.oldPrice;
        const newPrice = data.newPrice;
        const timeRemaining = data.timeRemaining;

        console.log(`收到价格更新: 拍卖ID ${auctionId}, 价格 ${oldPrice} -> ${newPrice}, 剩余时间 ${timeRemaining}秒`);

        // 验证价格单调递减（第一层防护）
        if (oldPrice && newPrice && newPrice > oldPrice) {
            console.error(`价格验证失败: 新价格 (${newPrice}) 高于旧价格 (${oldPrice})，荷兰钟拍卖价格不应上涨`);
            return;
        }

        // 获取当前显示的价格，用于本地验证
        const buyerPriceElement = document.querySelector(`#auctionsList [data-auction-id="${auctionId}"] .price-countdown`);
        const sellerPriceElement = document.querySelector(`#sellerAuctionsList [data-auction-id="${auctionId}"] .price-countdown`);
        const currentDisplayPrice = buyerPriceElement ? parseFloat(buyerPriceElement.textContent.replace(/[^\d.]/g, '')) : 
                                    (sellerPriceElement ? parseFloat(sellerPriceElement.textContent.replace(/[^\d.]/g, '')) : null);

        // 本地价格验证：检查价格变化方向是否正确（荷兰钟拍卖价格应该只递减）
        if (currentDisplayPrice !== null && !isNaN(currentDisplayPrice)) {
            if (newPrice > currentDisplayPrice) {
                console.error(`本地价格验证失败: 新价格 (${newPrice}) 高于当前显示价格 (${currentDisplayPrice})，荷兰钟拍卖价格不应上涨`);
                return;
            }
            
            // 验证价格变化幅度是否合理（防止异常跳变）
            const priceDifference = Math.abs(newPrice - currentDisplayPrice);
            const maxReasonableChange = currentDisplayPrice * 0.5; // 允许最大50%的变化
            
            if (priceDifference > maxReasonableChange && currentDisplayPrice > 0) {
                console.error(`本地价格验证失败: 价格变化幅度过大 (${currentDisplayPrice} -> ${newPrice})，可能是异常波动`);
                return;
            }
        }

        // 防抖机制：如果最近已经更新过这个拍卖的价格，延迟更新
        if (window.priceUpdateDebounce && window.priceUpdateDebounce[auctionId]) {
            clearTimeout(window.priceUpdateDebounce[auctionId]);
        }
        
        // 初始化防抖对象
        if (!window.priceUpdateDebounce) {
            window.priceUpdateDebounce = {};
        }
        
        // 设置防抖延迟（200毫秒）
        window.priceUpdateDebounce[auctionId] = setTimeout(() => {
            // 获取拍卖数据用于服务器端验证
            fetch('/api/auction/get', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    auction_id: parseInt(auctionId)
                })
            })
                .then(response => response.json())
                .then(result => {
                    if (result.auction) {
                        const auction = result.auction;

                        // 服务器端价格验证
                        if (!validateAuctionPrice(auction, newPrice)) {
                            console.error(`服务器价格验证失败，拒绝更新价格: 拍卖ID ${auctionId}, 价格 ${newPrice}`);
                            return;
                        }

                        // 验证剩余时间是否合理
                        if (!validateAuctionTimeRemaining(auction, timeRemaining)) {
                            console.error(`时间验证失败，拒绝更新时间: 拍卖ID ${auctionId}, 时间 ${timeRemaining}`);
                            return;
                        }

                        // 验证通过，更新价格显示
                        updateAuctionPrice(auctionId, 'buyer', newPrice);
                        updateAuctionPrice(auctionId, 'seller', newPrice);

                        // 更新拍卖卡片内的可视化器
                        if (window.auctionVisualizers && window.auctionVisualizers[auctionId]) {
                            try {
                                const visualizer = window.auctionVisualizers[auctionId];
                                visualizer.updatePrice(newPrice);
                                visualizer.updateRemainingTime(timeRemaining);
                            } catch (error) {
                                console.error(`更新拍卖 ${auctionId} 的可视化器价格失败:`, error);
                            }
                        }

                        // 如果价格已达到最低价，停止相关定时器
                        // 注意：这里不直接结束拍卖，因为后端会发送拍卖状态更新消息
                        if (timeRemaining <= 0) {
                            console.log(`拍卖 ${auctionId} 已达到最低价，等待后端处理拍卖结束`);
                        }
                    }
                })
                .catch(error => {
                    console.error('获取拍卖数据失败:', error);
                    // 如果获取数据失败，仍然进行本地验证后再更新价格显示
                    if (currentDisplayPrice !== null && !isNaN(currentDisplayPrice)) {
                        // 本地验证通过，更新价格
                        updateAuctionPrice(auctionId, 'buyer', newPrice);
                        updateAuctionPrice(auctionId, 'seller', newPrice);
                    } else {
                        console.warn('无法验证价格，不更新显示');
                    }
                })
                .finally(() => {
                    // 清除防抖定时器引用
                    if (window.priceUpdateDebounce && window.priceUpdateDebounce[auctionId]) {
                        delete window.priceUpdateDebounce[auctionId];
                    }
                });
        }, 200); // 200毫秒防抖延迟
    }

    // 处理拍卖状态更新
    function handleAuctionStatusUpdate(data) {
        if (!data || !data.auction) return;

        const auction = data.auction;

        // 更新拍卖卡片状态
        updateAuctionCardStatus(auction.id, auction.status, 'buyer');
        updateAuctionCardStatus(auction.id, auction.status, 'seller');

        // 更新拍卖卡片内的可视化器
        if (window.auctionVisualizers && window.auctionVisualizers[auction.id]) {
            try {
                const visualizer = window.auctionVisualizers[auction.id];
                
                // 根据拍卖状态控制动画
                if (auction.status === 'active') {
                    visualizer.resumeAnimation();
                } else {
                    visualizer.pauseAnimation();
                }
            } catch (error) {
                console.error(`更新拍卖 ${auction.id} 的可视化器状态失败:`, error);
            }
        }
    }

    // 更新拍卖卡片
    function updateAuctionCard(auction, view) {
        const containerId = view === 'seller' ? 'sellerAuctionsList' : 'auctionsList';
        const container = document.getElementById(containerId);

        if (!container) return;

        const card = container.querySelector(`[data-auction-id="${auction.id}"]`);
        if (!card) return;

        // 更新价格显示
        const priceElement = card.querySelector('.price-countdown');
        if (priceElement) {
            priceElement.textContent = `¥ ${(auction.currentPrice || 0).toFixed(2)}`;
        }

        // 更新拍卖卡片内的可视化器
        if (window.auctionVisualizers && window.auctionVisualizers[auction.id]) {
            try {
                const visualizer = window.auctionVisualizers[auction.id];
                visualizer.updatePrice(auction.currentPrice || 0);
                
                // 计算剩余时间
                const timeRemaining = calculateAuctionRemainingSeconds(auction);
                visualizer.updateRemainingTime(timeRemaining);
                
                // 根据拍卖状态控制动画
                if (auction.status === 'active') {
                    visualizer.resumeAnimation();
                } else {
                    visualizer.pauseAnimation();
                }
            } catch (error) {
                console.error(`更新拍卖 ${auction.id} 的可视化器失败:`, error);
            }
        }

        // 更新状态显示
        const statusElement = card.querySelector('.auction-status');
        if (statusElement) {
            const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
                auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
                    auction.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                        'bg-gray-100 text-gray-800';

            const statusText = auction.status === 'active' ? '进行中' :
                auction.status === 'pending' ? '待开始' :
                    auction.status === 'completed' ? '已完成' : '已取消';

            statusElement.className = `px-2 py-1 text-xs rounded-full ${statusClass} auction-status`;
            statusElement.textContent = statusText;
        }

        // 更新按钮状态
        const startButton = card.querySelector('.start-auction-btn');
        const pauseButton = card.querySelector('.pause-auction-btn');
        const cancelButton = card.querySelector('.cancel-auction-btn');
        const bidButton = card.querySelector('.bid-button');

        if (startButton) {
            startButton.style.display = auction.status === 'pending' ? 'inline-block' : 'none';
        }

        if (pauseButton) {
            pauseButton.style.display = auction.status === 'active' ? 'inline-block' : 'none';
        }

        if (cancelButton) {
            cancelButton.style.display = (auction.status === 'pending' || auction.status === 'active') ? 'inline-block' : 'none';
        }

        if (bidButton) {
            bidButton.disabled = auction.status !== 'active';
            bidButton.textContent = auction.status === 'active' ? '立即竞拍' :
                auction.status === 'pending' ? '尚未开始' :
                    auction.status === 'completed' ? '已结束' : '已取消';
        }
    }

    // 初始化荷兰钟可视化器
    initAuctionVisualizerClock();

    // 设置WebSocket消息处理器
    setupWebSocketHandlers();

    // 创建拍卖表单提交
    document.getElementById('createAuctionForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await createAuction();
    });

    // 竞价表单提交
    document.getElementById('bidForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await handleAuctionBid();
    });

    // 取消竞价按钮
    document.getElementById('cancelBid').addEventListener('click', () => {
        document.getElementById('bidModal').classList.add('hidden');
    });

    // 刷新卖家拍卖列表按钮
    document.getElementById('refreshSellerAuctions').addEventListener('click', () => {
        loadSellerAuctions();
    });
});

// 初始化荷兰钟可视化器
function initAuctionVisualizerClock() {
    try {
        // 初始化存储拍卖卡片可视化器的全局对象
        if (!window.auctionVisualizers) {
            window.auctionVisualizers = {};
        }
        
        console.log('荷兰钟可视化器初始化成功');
    } catch (error) {
        console.error('初始化荷兰钟可视化器失败:', error);
    }
}

// 更新价格显示
function updateAuctionPrice(auctionId, region, price) {
    // 验证价格参数是否提供
    if (price === undefined || price === null) {
        console.error(`未提供价格参数: 拍卖ID ${auctionId}`);
        return;
    }
    
    // 验证价格是否有效
    if (typeof price !== 'number' || isNaN(price) || price < 0) {
        console.error(`无效的价格数据: 拍卖ID ${auctionId}, 价格 ${price}`);
        return;
    }

    // 更新可视化器中的价格
    if (window.auctionVisualizers && window.auctionVisualizers[auctionId]) {
        try {
            window.auctionVisualizers[auctionId].updatePrice(price);
        } catch (error) {
            console.error(`更新可视化器价格失败:`, error);
        }
    }

    // 更新价格显示
    const containerId = region === 'seller' ? 'sellerAuctionsList' : 'auctionsList';
    const container = document.getElementById(containerId);
    
    if (!container) {
        console.error(`找不到容器元素: ${containerId}`);
        return;
    }

    // 根据区域使用不同的查找方式
    let card;
    if (region === 'seller') {
        // 卖家区域使用ID查找
        card = container.querySelector(`#seller-auction-${auctionId}`);
    } else {
        // 买家区域使用data-auction-id属性查找
        card = container.querySelector(`[data-auction-id="${auctionId}"]`);
    }
    
    if (!card) {
        console.error(`找不到拍卖卡片: 拍卖ID ${auctionId}, 区域: ${region}`);
        return;
    }

    // 更新价格显示
    const priceElement = card.querySelector('.price-countdown');
    if (priceElement) {
        priceElement.textContent = `¥ ${price.toFixed(2)}`;
    }
}

// 更新剩余时间显示
function updateAuctionTimeRemaining(timeRemaining) {
    // 验证剩余时间是否有效
    if (typeof timeRemaining !== 'number' || isNaN(timeRemaining) || timeRemaining < 0) {
        console.error(`无效的剩余时间数据: ${timeRemaining}`);
        return;
    }
}

// 更新拍卖状态
function updateAuctionStatus(auctionId, status) {
    // 获取拍卖数据
    fetch('/api/auction/get', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            auction_id: parseInt(auctionId)
        })
    })
        .then(response => response.json())
        .then(data => {
            const auction = data.auction;
            if (!auction || !auction.id) return;

            // 更新卖家区域
            const sellerElement = document.getElementById(`seller-auction-${auctionId}`);
            if (sellerElement) {
                updateAuctionCardStatus(sellerElement, status);
            }

            // 更新买家区域
            const buyerElement = document.getElementById(`auction-${auctionId}`);
            if (buyerElement) {
                updateAuctionCardStatus(buyerElement, status);
            }
        })
        .catch(error => {
            console.error('更新拍卖状态失败:', error);
        });
}

// 更新拍卖卡片状态
function updateAuctionCardStatus(element, status) {
    if (!element) return;

    const statusElement = element.querySelector('.px-2.py-1.text-xs.rounded-full');
    if (statusElement) {
        // 移除所有状态类
        statusElement.classList.remove('bg-green-100', 'text-green-800',
            'bg-yellow-100', 'text-yellow-800',
            'bg-blue-100', 'text-blue-800',
            'bg-gray-100', 'text-gray-800');

        // 添加新状态类
        if (status === 'active') {
            statusElement.classList.add('bg-green-100', 'text-green-800');
            statusElement.textContent = '进行中';
        } else if (status === 'pending') {
            statusElement.classList.add('bg-yellow-100', 'text-yellow-800');
            statusElement.textContent = '待开始';
        } else if (status === 'completed') {
            statusElement.classList.add('bg-blue-100', 'text-blue-800');
            statusElement.textContent = '已完成';
        } else {
            statusElement.classList.add('bg-gray-100', 'text-gray-800');
            statusElement.textContent = '已取消';
        }
    }

    // 更新按钮区域
    const buttonContainer = element.querySelector('.mt-4.flex, .mt-4.flex-col');
    if (buttonContainer) {
        if (status === 'completed' || status === 'cancelled') {
            buttonContainer.innerHTML = `
                <div class="w-full text-center text-gray-500 py-2">
                    拍卖已结束
                </div>
            `;
        }
    }
}

// 加载卖家拍卖列表
async function loadSellerAuctions() {
    try {
        const response = await fetch('/api/auction/seller-list');

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '加载卖家拍卖列表失败');
        }

        const data = await response.json();

        const sellerAuctionsList = document.getElementById('sellerAuctionsList');
        const noSellerAuctions = document.getElementById('noSellerAuctions');

        // 检查元素是否存在
        if (!sellerAuctionsList || !noSellerAuctions) {
            console.error('卖家拍卖列表元素未找到');
            return;
        }

        sellerAuctionsList.innerHTML = '';

        if (data.auctions && data.auctions.length > 0) {
            noSellerAuctions.classList.add('hidden');
            data.auctions.forEach(auction => {
                const auctionCard = createSellerAuctionCard(auction);
                sellerAuctionsList.appendChild(auctionCard);
            });
        } else {
            noSellerAuctions.classList.remove('hidden');
        }
    } catch (error) {
        console.error('加载卖家拍卖列表失败:', error);
        showNotification(`加载卖家拍卖列表失败: ${error.message}`, 'error');
    }
}

// 创建卖家拍卖卡片
function createSellerAuctionCard(auction) {
    const card = document.createElement('div');
    card.className = 'auction-card bg-white rounded-lg shadow-md overflow-hidden border border-gray-200';
    card.id = `seller-auction-${auction.id}`;

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
            auction.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                'bg-gray-100 text-gray-800';

    const statusText = auction.status === 'active' ? '进行中' :
        auction.status === 'pending' ? '待开始' :
            auction.status === 'completed' ? '已完成' : '已取消';

    card.innerHTML = `
        <div class="p-4">
            <div class="flex justify-between items-start mb-2">
                <h3 class="text-lg font-semibold">${auction.itemType === 'apple' ? '苹果' : '木材'}</h3>
                <span class="px-2 py-1 text-xs rounded-full ${statusClass}">${statusText}</span>
            </div>
            <div class="space-y-2">
                <div class="flex justify-between">
                    <span class="text-gray-600">数量:</span>
                    <span class="font-medium">${auction.quantity}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">当前价格:</span>
                    <span class="price-countdown font-bold text-lg text-indigo-600">¥ ${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">¥${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">¥${(auction.priceDecrement || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减间隔:</span>
                    <span class="font-medium">${auction.decrementInterval}秒</span>
                </div>
            </div>
            <div class="mt-4 flex flex-col space-y-2">
                ${auction.status === 'pending' ?
            `<button class="w-full bg-green-600 text-white py-2 px-4 rounded-md hover:bg-green-700 transition" 
                            onclick="startAuction(${auction.id})">
                        上架拍卖
                    </button>
                    <button class="w-full bg-red-600 text-white py-2 px-4 rounded-md hover:bg-red-700 transition" 
                            onclick="cancelAuction(${auction.id})">
                        取消拍卖
                    </button>` : ''}
                ${auction.status === 'active' ?
            `<button class="w-full bg-yellow-600 text-white py-2 px-4 rounded-md hover:bg-yellow-700 transition" 
                            onclick="pauseAuction(${auction.id})">
                        下架拍卖
                    </button>` : ''}
                ${auction.status === 'completed' || auction.status === 'cancelled' ?
            `<div class="w-full text-center text-gray-500 py-2">
                        拍卖已结束
                    </div>` : ''}
            </div>
        </div>
    `;

    return card;
}

// 加载拍卖列表
async function loadAuctions() {
    try {
        const response = await fetch('/api/auction/list');

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '加载拍卖列表失败');
        }

        const data = await response.json();

        const auctionsList = document.getElementById('auctionsList');

        // 检查元素是否存在
        if (!auctionsList) {
            console.error('拍卖列表元素未找到');
            return;
        }

        auctionsList.innerHTML = '';

        if (data.auctions && data.auctions.length > 0) {
            data.auctions.forEach(auction => {
                const auctionCard = createAuctionCard(auction);
                auctionsList.appendChild(auctionCard);
            });
        } else {
            auctionsList.innerHTML = '<p class="col-span-full text-center text-gray-500">当前没有进行中的拍卖</p>';
        }
    } catch (error) {
        console.error('加载拍卖列表失败:', error);
        showNotification(`加载拍卖列表失败: ${error.message}`, 'error');
    }
}

// 创建拍卖卡片
function createAuctionCard(auction) {
    const card = document.createElement('div');
    card.className = 'auction-card bg-white rounded-lg shadow-md overflow-hidden';
    card.id = `auction-${auction.id}`;
    card.setAttribute('data-auction-id', auction.id);

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
            'bg-gray-100 text-gray-800';

    const statusText = auction.status === 'active' ? '进行中' :
        auction.status === 'pending' ? '待开始' :
            auction.status === 'completed' ? '已完成' : '已取消';

    // 为每个拍卖卡片创建唯一的canvas ID
    const canvasId = `dutch-clock-canvas-${auction.id}`;

    card.innerHTML = `
        <div class="p-4">
            <div class="flex justify-between items-start mb-2">
                <h3 class="text-lg font-semibold">${auction.itemType === 'apple' ? '苹果' : '木材'}</h3>
                <span class="px-2 py-1 text-xs rounded-full ${statusClass}">${statusText}</span>
            </div>
            <!-- 荷兰钟可视化区域 -->
            <div class="mb-4 flex flex-col items-center">
                <span class="mb-2">荷兰钟可视化</span>
                <div class="bg-gray-50 rounded-lg p-2">
                    <canvas id="${canvasId}" width="200" height="200" class="border border-gray-300 rounded-lg shadow-inner"></canvas>
                </div>
            </div>
            <div class="space-y-2">
                <div class="flex justify-between">
                    <span class="text-gray-600">数量:</span>
                    <span class="font-medium">${auction.quantity}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">当前价格:</span>
                    <span class="price-countdown font-bold text-lg text-indigo-600">¥ ${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">¥ ${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">¥ ${(auction.priceDecrement || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减间隔:</span>
                    <span class="font-medium">${auction.decrementInterval}秒</span>
                </div>
            </div>
            <div class="mt-4 flex space-x-2">
                ${auction.status === 'active' ?
            `<button class="bid-button flex-1 bg-indigo-600 text-white py-2 px-4 rounded-md hover:bg-indigo-700 transition" 
                            onclick="openAuctionBidModal(${auction.id}, '${auction.itemType}', ${auction.currentPrice}, ${auction.minPrice}, ${auction.quantity})">
                        竞价
                    </button>` : ''}
                ${auction.status === 'pending' ?
            `<div class="flex-1 text-center text-gray-500 py-2">
                        拍卖尚未开始
                    </div>` : ''}
                ${auction.status === 'completed' || auction.status === 'cancelled' ?
            `<div class="flex-1 text-center text-gray-500 py-2">
                        拍卖已结束
                    </div>` : ''}
            </div>
        </div>
    `;

    // 在卡片添加到DOM后初始化荷兰钟可视化
    setTimeout(() => {
        try {
            // 确保canvas元素存在
            const canvasElement = document.getElementById(canvasId);
            if (!canvasElement) {
                console.error(`Canvas元素 ${canvasId} 不存在，无法初始化荷兰钟可视化`);
                return;
            }
            
            // 创建荷兰钟可视化器实例
            const visualizer = new AuctionClockVisualizer(canvasId);
            
            // 确保拍卖对象有正确的当前价格
            if (!auction.currentPrice) {
                auction.currentPrice = auction.initialPrice || auction.startPrice || 0;
            }
            
            // 计算并设置剩余时间
            auction.timeRemaining = calculateAuctionRemainingSeconds(auction);
            
            // 设置拍卖数据到可视化器
            visualizer.setAuction(auction);
            
            // 存储可视化器实例以便后续更新
            if (!window.auctionVisualizers) {
                window.auctionVisualizers = {};
            }
            window.auctionVisualizers[auction.id] = visualizer;
            
            // 如果拍卖状态为活跃，启动动画
            if (auction.status === 'active') {
                visualizer.resumeAnimation();
            } else {
                visualizer.pauseAnimation();
            }
        } catch (error) {
            console.error(`初始化拍卖 ${auction.id} 的荷兰钟可视化失败:`, error);
        }
    }, 100);

    return card;
}

// 创建拍卖
async function createAuction() {
    const itemTypeElement = document.getElementById('itemType');
    const startPriceElement = document.getElementById('startPrice');
    const minPriceElement = document.getElementById('minPrice');
    const quantityElement = document.getElementById('quantity');
    const priceDecrementIntervalElement = document.getElementById('priceDecrementInterval');
    const priceDecrementAmountElement = document.getElementById('priceDecrementAmount');

    // 检查元素是否存在
    if (!itemTypeElement || !startPriceElement || !minPriceElement || !quantityElement ||
        !priceDecrementIntervalElement || !priceDecrementAmountElement) {
        console.error('创建拍卖表单元素未找到');
        return;
    }

    const itemType = itemTypeElement.value;
    const startPrice = parseInt(startPriceElement.value);
    const minPrice = parseInt(minPriceElement.value);
    const quantity = parseInt(quantityElement.value);
    const priceDecrementInterval = parseInt(priceDecrementIntervalElement.value);
    const priceDecrementAmount = parseInt(priceDecrementAmountElement.value);

    try {
        const response = await fetch('/api/auction/create', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                itemType: itemType,
                initialPrice: startPrice,
                minPrice: minPrice,
                quantity: quantity,
                decrementInterval: priceDecrementInterval,
                priceDecrement: priceDecrementAmount
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖创建成功');
            const createAuctionForm = document.getElementById('createAuctionForm');
            if (createAuctionForm) {
                createAuctionForm.reset();
            }
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '创建拍卖失败', 'error');
        }
    } catch (error) {
        console.error('创建拍卖失败:', error);
        showNotification('创建拍卖失败', 'error');
    }
}

// 暂停拍卖（下架）
async function pauseAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/pause', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '下架拍卖失败');
        }

        const data = await response.json();
        showNotification(data.message || '拍卖已成功下架', 'success');

        // 清除递减价格计时器
        if (window.priceTimers && window.priceTimers[auctionId]) {
            clearInterval(window.priceTimers[auctionId]);
            delete window.priceTimers[auctionId];
        }

        // 刷新拍卖列表
        loadAuctions();
        loadSellerAuctions();
    } catch (error) {
        console.error('下架拍卖失败:', error);
        showNotification(`下架拍卖失败: ${error.message}`, 'error');
    }
}

// 开始拍卖
async function startAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖已开始');
            loadAuctions();
            loadSellerAuctions();

            // 启动价格递减定时器
            // 重新获取拍卖数据以获取最新的价格信息
            try {
                const auctionResponse = await fetch('/api/auction/get', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        auction_id: parseInt(auctionId)
                    })
                });
                if (auctionResponse.ok) {
                    const auctionData = await auctionResponse.json();
                    if (auctionData.auction && auctionData.auction.status === 'active') {
                        startAuctionPriceDecrementTimer(auctionData.auction);
                    }
                }
            } catch (error) {
                console.error('获取拍卖数据失败:', error);
            }
        } else {
            showNotification(data.message || '开始拍卖失败', 'error');
        }
    } catch (error) {
        console.error('开始拍卖失败:', error);
        showNotification('开始拍卖失败', 'error');
    }
}

// 取消拍卖
async function cancelAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/cancel', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖已取消');

            // 清除递减价格计时器
            if (window.priceTimers && window.priceTimers[auctionId]) {
                clearInterval(window.priceTimers[auctionId]);
                delete window.priceTimers[auctionId];
            }

            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '取消拍卖失败', 'error');
        }
    } catch (error) {
        console.error('取消拍卖失败:', error);
        showNotification('取消拍卖失败', 'error');
    }
}

// 打开竞价模态框
function openAuctionBidModal(auctionId, itemType, currentPrice, minPrice, quantity) {
    const bidModal = document.getElementById('bidModal');
    const bidInfo = document.getElementById('bidInfo');
    const bidAmount = document.getElementById('bidAmount');

    // 检查元素是否存在
    if (!bidModal || !bidInfo || !bidAmount) {
        console.error('竞价模态框元素未找到');
        return;
    }

    bidInfo.innerHTML = `
        <div class="space-y-2">
            <div class="flex justify-between">
                <span class="text-gray-600">物品:</span>
                <span class="font-medium">${itemType === 'apple' ? '苹果' : '木材'}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">数量:</span>
                <span class="font-medium">${quantity}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">当前价格:</span>
                <span class="font-bold text-indigo-600">¥ ${(currentPrice || 0).toFixed(2)}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">最低价格:</span>
                <span class="font-medium">¥ ${(minPrice || 0).toFixed(2)}</span>
            </div>
        </div>
    `;

    bidAmount.min = minPrice;
    bidAmount.max = currentPrice;
    bidAmount.value = currentPrice;

    bidModal.dataset.auctionId = auctionId;
    bidModal.classList.remove('hidden');
}

// 提交竞价
async function handleAuctionBid() {
    const bidModal = document.getElementById('bidModal');

    // 检查元素是否存在
    if (!bidModal) {
        console.error('竞价模态框元素未找到');
        return;
    }

    const auctionId = parseInt(bidModal.dataset.auctionId);
    const bidAmount = parseInt(document.getElementById('bidAmount').value);

    try {
        const response = await fetch('/api/auction/bid', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId,
                bid_amount: bidAmount
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('竞价成功');
            bidModal.classList.add('hidden');

            // 清除可视化递减价格计时器
            if (window.visualPriceTimer) {
                clearInterval(window.visualPriceTimer);
            }

            loadBalance();
            loadAuctions();
            loadSellerAuctions();

            // 如果当前在可视化器中显示的是这个拍卖，更新其状态
            const visualAuctionSelect = document.getElementById('visualAuctionSelect');
            if (visualAuctionSelect && visualAuctionSelect.value == auctionId) {
                loadAuctionVisualizer(auctionId);
            }
        } else {
            showNotification(data.message || '竞价失败', 'error');
        }
    } catch (error) {
        console.error('竞价失败:', error);
        showNotification('竞价失败', 'error');
    }
}

// 初始化价格递减定时器管理器
window.priceTimers = window.priceTimers || {};

// 启动拍卖价格递减定时器
function startAuctionPriceDecrementTimer(auction) {
    // 检查拍卖是否有效
    if (!auction || !auction.id) {
        console.error('无效的拍卖数据');
        return;
    }

    // 清除已存在的定时器
    if (window.priceTimers[auction.id]) {
        clearInterval(window.priceTimers[auction.id]);
    }
}

// 计算剩余时间
function calculateAuctionRemainingSeconds(auction) {
    const currentPriceOffset = (auction.currentPrice || 0) - (auction.minPrice || 0);
    const remainingDecrementSteps = Math.ceil(currentPriceOffset / (auction.priceDecrement || 1));
    return remainingDecrementSteps * (auction.decrementInterval || 1);
}

// 处理快速竞拍
async function handleQuickBid(auctionId, bidAmount) {
    try {
        const response = await fetch('/api/auction/bid', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: parseInt(auctionId),
                bid_amount: bidAmount
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('竞价成功');

            // 刷新页面数据
            loadBalance();
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '竞价失败', 'error');
        }
    } catch (error) {
        console.error('快速竞价失败:', error);
        showNotification('竞价失败', 'error');
    }
}

// 加载现金余额
async function loadBalance() {
    try {
        const response = await fetch('/api/cash/balance');
        if (response.ok) {
            const data = await response.json();

            // 检查响应数据结构
            if (!data.success || !data.balance) {
                throw new Error('Invalid balance data response');
            }

            const balance = data.balance;

            // 更新余额显示
            const balanceAmount = document.getElementById('balanceAmount');
            if (balanceAmount) {
                balanceAmount.textContent = formatCurrency(balance.amount || 0);
            }

            // 更新最后更新时间
            const lastUpdated = document.getElementById('lastUpdated');
            if (lastUpdated) {
                const date = new Date(balance.updated_at);
                lastUpdated.textContent = `最后更新: ${formatDateTime(date)}`;
            }
        }
    } catch (error) {
        console.error('Error loading balance:', error);
        showNotification('加载余额失败', 'error');
    }
}

// 加载卖家拍卖列表
async function loadSellerAuctions() {
    try {
        const response = await fetch('/api/auction/seller-list');

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '加载卖家拍卖列表失败');
        }

        const data = await response.json();

        const sellerAuctionsList = document.getElementById('sellerAuctionsList');
        const noSellerAuctions = document.getElementById('noSellerAuctions');

        // 检查元素是否存在
        if (!sellerAuctionsList || !noSellerAuctions) {
            console.error('卖家拍卖列表元素未找到');
            return;
        }

        sellerAuctionsList.innerHTML = '';

        if (data.auctions && data.auctions.length > 0) {
            noSellerAuctions.classList.add('hidden');
            data.auctions.forEach(auction => {
                const auctionCard = createSellerAuctionCard(auction);
                sellerAuctionsList.appendChild(auctionCard);

                // 如果拍卖已开始且未结束，启动递减价格计时器
                if (auction.status === 'active') {
                    startAuctionPriceDecrementTimer(auction);
                }
            });
        } else {
            noSellerAuctions.classList.remove('hidden');
        }
    } catch (error) {
        console.error('加载卖家拍卖列表失败:', error);
        showNotification(`加载卖家拍卖列表失败: ${error.message}`, 'error');
    }
}

// 创建卖家拍卖卡片
function createSellerAuctionCard(auction) {
    const card = document.createElement('div');
    card.className = 'auction-card bg-white rounded-lg shadow-md overflow-hidden border border-gray-200';
    card.id = `seller-auction-${auction.id}`;

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
            auction.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                'bg-gray-100 text-gray-800';

    const statusText = auction.status === 'active' ? '进行中' :
        auction.status === 'pending' ? '待开始' :
            auction.status === 'completed' ? '已完成' : '已取消';

    card.innerHTML = `
        <div class="p-4">
            <div class="flex justify-between items-start mb-2">
                <h3 class="text-lg font-semibold">${auction.itemType === 'apple' ? '苹果' : '木材'}</h3>
                <span class="px-2 py-1 text-xs rounded-full ${statusClass}">${statusText}</span>
            </div>
            <div class="space-y-2">
                <div class="flex justify-between">
                    <span class="text-gray-600">数量:</span>
                    <span class="font-medium">${auction.quantity}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">当前价格:</span>
                    <span class="price-countdown font-bold text-lg text-indigo-600">¥ ${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">¥ ${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">¥ ${(auction.priceDecrement || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减间隔:</span>
                    <span class="font-medium">${auction.decrementInterval}秒</span>
                </div>
            </div>
            <div class="mt-4 flex flex-col space-y-2">
                ${auction.status === 'pending' ?
            `<button class="w-full bg-green-600 text-white py-2 px-4 rounded-md hover:bg-green-700 transition" 
                            onclick="startAuction(${auction.id})">
                        上架拍卖
                    </button>
                    <button class="w-full bg-red-600 text-white py-2 px-4 rounded-md hover:bg-red-700 transition" 
                            onclick="cancelAuction(${auction.id})">
                        取消拍卖
                    </button>` : ''}
                ${auction.status === 'active' ?
            `<button class="w-full bg-yellow-600 text-white py-2 px-4 rounded-md hover:bg-yellow-700 transition" 
                            onclick="pauseAuction(${auction.id})">
                        下架拍卖
                    </button>` : ''}
                ${auction.status === 'completed' || auction.status === 'cancelled' ?
            `<div class="w-full text-center text-gray-500 py-2">
                        拍卖已结束
                    </div>` : ''}
            </div>
        </div>
    `;

    return card;
}

// 加载拍卖列表
async function loadAuctions() {
    try {
        const response = await fetch('/api/auction/list');

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '加载拍卖列表失败');
        }

        const data = await response.json();

        const auctionsList = document.getElementById('auctionsList');

        // 检查元素是否存在
        if (!auctionsList) {
            console.error('拍卖列表元素未找到');
            return;
        }

        auctionsList.innerHTML = '';

        if (data.auctions && data.auctions.length > 0) {
            data.auctions.forEach(auction => {
                const auctionCard = createAuctionCard(auction);
                auctionsList.appendChild(auctionCard);

                // 如果拍卖已开始且未结束，启动递减价格计时器
                if (auction.status === 'active') {
                    startAuctionPriceDecrementTimer(auction);
                }
            });
        } else {
            auctionsList.innerHTML = '<p class="col-span-full text-center text-gray-500">当前没有进行中的拍卖</p>';
        }

    } catch (error) {
        console.error('加载拍卖列表失败:', error);
        showNotification(`加载拍卖列表失败: ${error.message}`, 'error');
    }
}



// 创建拍卖
async function createAuction() {
    const itemType = document.getElementById('itemType').value;
    const startPrice = parseInt(document.getElementById('startPrice').value);
    const minPrice = parseInt(document.getElementById('minPrice').value);
    const quantity = parseInt(document.getElementById('quantity').value);
    const priceDecrementInterval = parseInt(document.getElementById('priceDecrementInterval').value);
    const priceDecrementAmount = parseInt(document.getElementById('priceDecrementAmount').value);

    try {
        const response = await fetch('/api/auction/create', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                itemType: itemType,
                initialPrice: startPrice,
                minPrice: minPrice,
                quantity: quantity,
                decrementInterval: priceDecrementInterval,
                priceDecrement: priceDecrementAmount
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖创建成功');
            document.getElementById('createAuctionForm').reset();
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '创建拍卖失败', 'error');
        }
    } catch (error) {
        console.error('创建拍卖失败:', error);
        showNotification('创建拍卖失败', 'error');
    }
}

// 暂停拍卖（下架）
async function pauseAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/pause', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '下架拍卖失败');
        }

        const data = await response.json();
        showNotification(data.message || '拍卖已成功下架', 'success');

        // 刷新拍卖列表
        loadAuctions();
        loadSellerAuctions();
    } catch (error) {
        console.error('下架拍卖失败:', error);
        showNotification(`下架拍卖失败: ${error.message}`, 'error');
    }
}

// 开始拍卖
async function startAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/start', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖已开始');
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '开始拍卖失败', 'error');
        }
    } catch (error) {
        console.error('开始拍卖失败:', error);
        showNotification('开始拍卖失败', 'error');
    }
}

// 取消拍卖
async function cancelAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/cancel', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖已取消');

            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '取消拍卖失败', 'error');
        }
    } catch (error) {
        console.error('取消拍卖失败:', error);
        showNotification('取消拍卖失败', 'error');
    }
}

// 打开竞价模态框
function openAuctionBidModal(auctionId, itemType, currentPrice, minPrice, quantity) {
    const bidModal = document.getElementById('bidModal');
    const bidInfo = document.getElementById('bidInfo');
    const bidAmount = document.getElementById('bidAmount');

    // 检查元素是否存在
    if (!bidModal || !bidInfo || !bidAmount) {
        console.error('竞价模态框元素未找到');
        return;
    }

    bidInfo.innerHTML = `
        <div class="space-y-2">
            <div class="flex justify-between">
                <span class="text-gray-600">物品:</span>
                <span class="font-medium">${itemType === 'apple' ? '苹果' : '木材'}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">数量:</span>
                <span class="font-medium">${quantity}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">当前价格:</span>
                <span class="font-bold text-indigo-600">¥ ${(currentPrice || 0).toFixed(2)}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">最低价格:</span>
                <span class="font-medium">¥ ${(minPrice || 0).toFixed(2)}</span>
            </div>
        </div>
    `;

    bidAmount.min = minPrice;
    bidAmount.max = currentPrice;
    bidAmount.value = currentPrice;

    bidModal.dataset.auctionId = auctionId;
    bidModal.classList.remove('hidden');
}

// 提交竞价
async function handleAuctionBid() {
    const bidModal = document.getElementById('bidModal');

    // 检查元素是否存在
    if (!bidModal) {
        console.error('竞价模态框元素未找到');
        return;
    }

    const auctionId = parseInt(bidModal.dataset.auctionId);
    const bidAmount = parseInt(document.getElementById('bidAmount').value);

    try {
        const response = await fetch('/api/auction/bid', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId,
                bid_amount: bidAmount
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('竞价成功');
            bidModal.classList.add('hidden');

            // 使用统一定时器管理系统清除所有相关定时器
            if (window.timerManager) {
                // 清除卖家区域的价格递减定时器
                window.timerManager.stopSellerPriceDecrementTimer(auctionId);
                // 清除买家区域的价格递减定时器
                window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
            }

            loadBalance();
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '竞价失败', 'error');
        }
    } catch (error) {
        console.error('竞价失败:', error);
        showNotification('竞价失败', 'error');
    }
}

// 结束拍卖
async function endAuction(auctionId) {
    try {
        const response = await fetch('/api/auction/end', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: parseInt(auctionId)
            })
        });

        const data = await response.json();

        if (data.success) {
            showNotification('拍卖已结束');

            // 使用统一定时器管理系统清除所有相关定时器
            if (window.timerManager) {
                // 清除卖家区域的价格递减定时器
                window.timerManager.stopSellerPriceDecrementTimer(auctionId);
                // 清除买家区域的价格递减定时器
                window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
            }

            // 刷新拍卖列表
            loadAuctions();
            loadSellerAuctions();
        } else {
            showNotification(data.message || '结束拍卖失败', 'error');
        }
    } catch (error) {
        console.error('结束拍卖失败:', error);
        showNotification('结束拍卖失败', 'error');
    }
}

// 显示通知
function showNotification(message, type = 'success') {
    const notification = document.getElementById('notification');
    const notificationMessage = document.getElementById('notificationMessage');

    // 检查元素是否存在
    if (!notification || !notificationMessage) {
        console.error('通知元素未找到');
        return;
    }

    notificationMessage.textContent = message;

    if (type === 'error') {
        notification.className = 'fixed bottom-4 right-4 bg-red-500 text-white px-6 py-3 rounded-md shadow-lg transform transition-transform duration-300';
    } else {
        notification.className = 'fixed bottom-4 right-4 bg-green-500 text-white px-6 py-3 rounded-md shadow-lg transform transition-transform duration-300';
    }

    // 显示通知
    setTimeout(() => {
        notification.classList.remove('translate-y-20');
    }, 10);

    // 3秒后隐藏通知
    setTimeout(() => {
        notification.classList.add('translate-y-20');
    }, 3000);
}

// 格式化日期 (YYYY-MM-DD)
function formatDate(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

// 格式化日期时间 (YYYY-MM-DD HH:MM)
function formatDateTime(date) {
    const formattedDate = formatDate(date);
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${formattedDate} ${hours}:${minutes}`;
}

// 格式化货币，添加货币符号
function formatCurrency(amount) {
    const currencySymbol = '¥';
    let formattedAmount = '';

    // 检查 amount 是否为 undefined 或 null
    if (amount === undefined || amount === null) {
        return `${currencySymbol} 0.00`;
    }

    if (amount > 0) {
        formattedAmount = `${formatNumberWithCommas(amount.toFixed(2))}`;
    } else if (amount === 0) {
        formattedAmount = '-';
    } else {
        formattedAmount = `(${formatNumberWithCommas(amount.toFixed(2))})`;
    }
    return `${currencySymbol} ${formattedAmount}`;
}

// 格式化数字，添加千位分隔符
function formatNumberWithCommas(number) {
    return number.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

// 价格校验函数
/**
 * 验证价格是否合理
 * @param {Object} auction - 拍卖数据
 * @param {number} newPrice - 新价格
 * @returns {boolean} - 价格是否合理
 */
function validateAuctionPrice(auction, newPrice) {
    if (!auction || typeof newPrice !== 'number') {
        console.error('价格验证失败：无效的参数');
        return false;
    }

    // 检查价格是否为非负数
    if (newPrice < 0) {
        console.error(`价格验证失败: 价格不能为负数 (${newPrice})`);
        return false;
    }

    // 检查价格是否在合理范围内
    const minPrice = auction.minPrice || auction.min_price || 0;
    const maxPrice = auction.initialPrice || auction.initial_price || auction.startPrice || 0;

    if (newPrice < minPrice || newPrice > maxPrice) {
        console.warn(`价格验证警告：价格 ${newPrice} 超出合理范围 [${minPrice}, ${maxPrice}]`);
        return false;
    }

    // 检查价格变化是否合理（价格应该递减）
    const currentPrice = auction.currentPrice || auction.current_price || maxPrice;
    if (newPrice > currentPrice) {
        console.error(`价格验证失败: 新价格 (${newPrice}) 高于当前价格 (${currentPrice})，荷兰钟拍卖价格不应上涨`);
        return false;
    }
    
    // 验证价格变化幅度是否合理（防止异常跳变）
    const priceDifference = Math.abs(newPrice - currentPrice);
    const maxReasonableChange = currentPrice * 0.5; // 允许最大50%的变化
    
    if (priceDifference > maxReasonableChange && currentPrice > 0) {
        console.error(`价格验证失败: 价格变化幅度过大 (${currentPrice} -> ${newPrice})，可能是异常波动`);
        return false;
    }

    return true;
}

/**
 * 验证剩余时间是否合理
 * @param {Object} auction - 拍卖数据
 * @param {number} timeRemaining - 剩余时间（秒）
 * @returns {boolean} - 时间是否合理
 */
function validateAuctionTimeRemaining(auction, timeRemaining) {
    if (!auction || typeof timeRemaining !== 'number') {
        console.error('时间验证失败：无效的参数');
        return false;
    }

    // 检查时间是否为负数
    if (timeRemaining < 0) {
        console.warn(`时间验证警告：剩余时间 ${timeRemaining} 为负数`);
        return false;
    }

    // 检查时间是否过长（超过24小时）
    const maxTime = 24 * 60 * 60; // 24小时，单位：秒
    if (timeRemaining > maxTime) {
        console.warn(`时间验证警告：剩余时间 ${timeRemaining} 过长`);
        return false;
    }

    return true;
}