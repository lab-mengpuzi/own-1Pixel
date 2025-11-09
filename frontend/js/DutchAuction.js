document.addEventListener('DOMContentLoaded', () => {
    // 初始化页面
    loadBalance();
    loadAuctions();
    loadSellerAuctions();

    // 初始化荷兰钟可视化器
    initDutchClockVisualizer();

    // 创建拍卖表单提交
    document.getElementById('createAuctionForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await createAuction();
    });

    // 竞价表单提交
    document.getElementById('bidForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await placeBid();
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
function initDutchClockVisualizer() {
    try {
        // 检查canvas元素是否存在
        const canvas = document.getElementById('dutchClockCanvas');
        if (!canvas) {
            console.error('荷兰钟canvas元素未找到');
            return;
        }

        // 创建荷兰钟可视化器实例
        window.dutchClockVisualizer = new DutchClockVisualizer('dutchClockCanvas');

        // 检查拍卖选择器是否存在
        const auctionSelect = document.getElementById('visualAuctionSelect');
        if (auctionSelect) {
            // 添加拍卖选择事件监听
            auctionSelect.addEventListener('change', (e) => {
                const auctionId = e.target.value;
                if (auctionId) {
                    loadAuctionForVisualization(auctionId);
                } else {
                    window.dutchClockVisualizer.reset();
                    updateVisualAuctionInfo(null);
                }
            });
        }

        // 检查快速竞拍按钮是否存在
        const quickBidButton = document.getElementById('quickBidButton');
        if (quickBidButton && auctionSelect) {
            // 添加快速竞拍按钮事件监听
            quickBidButton.addEventListener('click', () => {
                const auctionId = auctionSelect.value;
                if (auctionId && window.dutchClockVisualizer.auction) {
                    placeQuickBid(auctionId, window.dutchClockVisualizer.currentPrice);
                }
            });
        }

        console.log('荷兰钟可视化器初始化成功');
    } catch (error) {
        console.error('初始化荷兰钟可视化器失败:', error);
    }
}

// 加载拍卖到可视化器
async function loadAuctionForVisualization(auctionId) {
    try {
        const response = await fetch('/api/dutch-auction/get', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: parseInt(auctionId)
            })
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || '获取拍卖详情失败');
        }

        const data = await response.json();

        if (data.auction) {
            // 确保拍卖对象有正确的当前价格
            if (!data.auction.currentPrice) {
                data.auction.currentPrice = data.auction.initialPrice || data.auction.startPrice || 0;
            }

            // 设置拍卖数据到可视化器
            window.dutchClockVisualizer.setAuction(data.auction);

            // 更新拍卖信息显示
            updateVisualAuctionInfo(data.auction);

            // 启用快速竞拍按钮
            const quickBidButton = document.getElementById('quickBidButton');
            quickBidButton.disabled = false;

            // 如果拍卖正在进行中，启动递减价格计时器
            if (data.auction.status === 'active') {
                startVisualPriceDecrementTimer(data.auction);
            }
        }
    } catch (error) {
        console.error('加载拍卖到可视化器失败:', error);
        showNotification(`加载拍卖详情失败: ${error.message}`, 'error');
    }
}

// 更新可视化拍卖信息显示
function updateVisualAuctionInfo(auction) {
    const visualAuctionInfo = document.getElementById('visualAuctionInfo');

    // 检查元素是否存在
    if (!visualAuctionInfo) {
        console.error('可视化拍卖信息元素未找到');
        return;
    }

    if (!auction) {
        visualAuctionInfo.innerHTML = `
            <h4 class="font-medium mb-2">拍卖信息</h4>
            <div class="text-sm text-gray-600">
                <p>请选择一个拍卖以查看详情</p>
            </div>
        `;
        return;
    }

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
            auction.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                'bg-gray-100 text-gray-800';

    const statusText = auction.status === 'active' ? '进行中' :
        auction.status === 'pending' ? '待开始' :
            auction.status === 'completed' ? '已完成' : '已取消';

    visualAuctionInfo.innerHTML = `
        <h4 class="font-medium mb-2">拍卖信息</h4>
        <div class="space-y-2 text-sm">
            <div class="flex justify-between">
                <span class="text-gray-600">物品:</span>
                <span class="font-medium">${auction.itemType === 'apple' ? '苹果' : '木材'}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">数量:</span>
                <span class="font-medium">${auction.quantity}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">状态:</span>
                <span class="px-2 py-1 text-xs rounded-full ${statusClass}">${statusText}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">初始价格:</span>
                <span class="font-medium">¥${(auction.initialPrice || auction.startPrice || 0).toFixed(2)}</span>
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
    `;
}

// 启动可视化递减价格计时器
function startVisualPriceDecrementTimer(auction) {
    // 使用DutchTimerManager管理可视化定时器
    if (!auction || !window.dutchClockVisualizer) return;
    
    // 清除可能存在的旧定时器
    window.timerManager.stopVisualTimer('price-decrement');
    
    let currentPrice = auction.currentPrice || auction.initialPrice || auction.startPrice || 0;
    let remainingSeconds = calculateRemainingSeconds(auction);
    
    // 更新剩余时间显示
    window.dutchClockVisualizer.updateRemainingTime(remainingSeconds);
    
    // 设置每秒更新一次，与canvas刷新频率同步
    window.timerManager.startVisualTimer('price-decrement', () => {
        // 只有在递减间隔秒数到达时才更新价格
        const decrementInterval = auction.decrementInterval || 1;
        const elapsedSeconds = (Date.now() - window.dutchClockVisualizer.lastUpdateTime) / 1000;
        
        if (elapsedSeconds >= decrementInterval) {
            currentPrice -= (auction.priceDecrement || 0);
            remainingSeconds -= decrementInterval;
            
            if (currentPrice <= (auction.minPrice || 0)) {
                currentPrice = auction.minPrice || 0;
                remainingSeconds = 0;
                window.timerManager.stopVisualTimer('price-decrement');
                
                // 价格达到最低价，自动结束拍卖
                endAuction(auction.id);
            }
            
            // 更新可视化器中的价格
            window.dutchClockVisualizer.updatePrice(currentPrice);
            
            // 更新剩余时间显示
            window.dutchClockVisualizer.updateRemainingTime(Math.max(0, remainingSeconds));
            
            // 更新最后更新时间
            window.dutchClockVisualizer.lastUpdateTime = Date.now();
        }
    }, 1000); // 每秒检查一次，但只在递减间隔到达时更新价格
}

// 更新价格显示
function updatePriceDisplay(auctionId, region) {
    // 获取拍卖数据
    fetch('/api/dutch-auction/get', {
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
            
            // 确定元素前缀
            const prefix = region === 'seller' ? 'seller-auction-' : 'auction-';
            const elementId = `${prefix}${auctionId}`;
            const auctionElement = document.getElementById(elementId);
            
            if (!auctionElement) return;
            
            // 更新价格显示
            const priceElement = auctionElement.querySelector('.price-countdown');
            if (priceElement) {
                priceElement.textContent = (auction.currentPrice || 0).toFixed(2);
            }
            
            // 如果价格已达到最低价，停止定时器
            if (auction.currentPrice <= auction.minPrice) {
                if (region === 'seller') {
                    window.timerManager.stopSellerPriceDecrementTimer(auctionId);
                } else {
                    window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
                }
                
                // 更新拍卖状态
                updateAuctionStatus(auctionId, 'completed');
            }
        })
        .catch(error => {
            console.error('更新价格显示失败:', error);
        });
}

// 更新拍卖状态
function updateAuctionStatus(auctionId, status) {
    // 获取拍卖数据
    fetch('/api/dutch-auction/get', {
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

// 计算剩余时间
function calculateRemainingSeconds(auction) {
    const startPrice = auction.initialPrice || auction.startPrice || 0;
    const priceRange = startPrice - (auction.minPrice || 0);
    const currentPriceOffset = (auction.currentPrice || 0) - (auction.minPrice || 0);
    const remainingDecrementSteps = Math.ceil(currentPriceOffset / (auction.priceDecrement || 1));
    return remainingDecrementSteps * (auction.decrementInterval || 1);
}

// 快速竞拍
async function placeQuickBid(auctionId, bidAmount) {
    try {
        const response = await fetch('/api/dutch-auction/bid', {
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

            // 使用DutchTimerManager清除定时器
            window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
            window.timerManager.stopVisualTimer('price-decrement');

            // 刷新页面数据
            loadBalance();
            loadAuctions();
            loadSellerAuctions();

            // 重新加载当前拍卖以更新状态
            loadAuctionForVisualization(auctionId);
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
        const response = await fetch('/api/market/balance');
        if (!response.ok) {
            throw new Error('Failed to load balance');
        }

        const balance = await response.json();

        // 更新余额显示
        const balanceAmount = document.getElementById('balanceAmount');
        if (balanceAmount) {
            balanceAmount.textContent = formatCurrency(balance.amount);
        }

        // 更新最后更新时间
        const lastUpdated = document.getElementById('lastUpdated');
        if (lastUpdated) {
            const date = new Date(balance.updated_at);
            lastUpdated.textContent = `最后更新: ${formatDateTime(date)}`;
        }
    } catch (error) {
        console.error('Error loading balance:', error);
        showNotification('加载余额失败', 'error');
    }
}

// 加载卖家拍卖列表
async function loadSellerAuctions() {
    try {
        const response = await fetch('/api/dutch-auction/seller-list');

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
                    startPriceDecrementTimer(auction.id, auction.currentPrice, auction.minPrice, auction.priceDecrement, auction.decrementInterval);
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
                    <span id="current-price-${auction.id}" class="price-countdown font-bold text-lg text-indigo-600">${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">${(auction.priceDecrement || 0).toFixed(2)}</span>
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
        const response = await fetch('/api/dutch-auction/list');

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

        // 更新可视化器的拍卖选择器
        const visualAuctionSelect = document.getElementById('visualAuctionSelect');
        if (visualAuctionSelect) {
            visualAuctionSelect.innerHTML = '<option value="">选择拍卖</option>';
        }

        if (data.auctions && data.auctions.length > 0) {
            data.auctions.forEach(auction => {
                const auctionCard = createAuctionCard(auction);
                auctionsList.appendChild(auctionCard);

                // 添加到可视化器的选择器中
                if (visualAuctionSelect) {
                    const option = document.createElement('option');
                    option.value = auction.id;

                    // 添加状态前缀
                    let statusPrefix = '';
                    if (auction.status === 'active') {
                        statusPrefix = '[进行中] ';
                    } else if (auction.status === 'pending') {
                        statusPrefix = '[待开始] ';
                    } else if (auction.status === 'completed') {
                        statusPrefix = '[已完成] ';
                    } else {
                        statusPrefix = '[已取消] ';
                    }

                    option.textContent = statusPrefix + `${auction.itemType === 'apple' ? '苹果' : '木材'} x${auction.quantity} - ¥${(auction.currentPrice || 0).toFixed(2)}`;
                    visualAuctionSelect.appendChild(option);
                }

                // 如果拍卖已开始且未结束，启动递减价格计时器
                if (auction.status === 'active') {
                    startPriceDecrementTimer(auction.id, auction.currentPrice, auction.minPrice, auction.priceDecrement, auction.decrementInterval);
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

// 创建拍卖卡片
function createAuctionCard(auction) {
    const card = document.createElement('div');
    card.className = 'auction-card bg-white rounded-lg shadow-md overflow-hidden';
    card.id = `auction-${auction.id}`;

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
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
                    <span id="current-price-${auction.id}" class="price-countdown font-bold text-lg text-indigo-600">${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">${(auction.priceDecrement || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减间隔:</span>
                    <span class="font-medium">${auction.decrementInterval}秒</span>
                </div>
            </div>
            <div class="mt-4 flex space-x-2">
                ${auction.status === 'active' ?
            `<button class="bid-button flex-1 bg-indigo-600 text-white py-2 px-4 rounded-md hover:bg-indigo-700 transition" 
                            onclick="openBidModal(${auction.id}, '${auction.itemType}', ${auction.currentPrice}, ${auction.minPrice}, ${auction.quantity})">
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
        const response = await fetch('/api/dutch-auction/create', {
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
        const response = await fetch('/api/dutch-auction/pause', {
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
        const response = await fetch('/api/dutch-auction/start', {
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
                const auctionResponse = await fetch('/api/dutch-auction/get', {
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
                        startPriceDecrementTimer(auctionData.auction);
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
        const response = await fetch('/api/dutch-auction/cancel', {
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
function openBidModal(auctionId, itemType, currentPrice, minPrice, quantity) {
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
                <span class="font-bold text-indigo-600">${(currentPrice || 0).toFixed(2)}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">最低价格:</span>
                <span class="font-medium">${(minPrice || 0).toFixed(2)}</span>
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
async function placeBid() {
    const bidModal = document.getElementById('bidModal');

    // 检查元素是否存在
    if (!bidModal) {
        console.error('竞价模态框元素未找到');
        return;
    }

    const auctionId = parseInt(bidModal.dataset.auctionId);
    const bidAmount = parseInt(document.getElementById('bidAmount').value);

    try {
        const response = await fetch('/api/dutch-auction/bid', {
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
                loadAuctionForVisualization(auctionId);
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

// 启动递减价格计时器
function startPriceDecrementTimer(auction) {
    if (!auction || !auction.id) return;
    
    try {
        // 根据拍卖状态决定使用哪个区域的定时器
        const isSellerView = document.getElementById('sellerAuctionsList').querySelector(`[data-auction-id="${auction.id}"]`);
        
        if (isSellerView) {
            // 卖家区域，启动卖家区域定时器
            const decrementInterval = (auction.decrementInterval || 1) * 1000; // 转换为毫秒
            window.timerManager.startSellerPriceDecrementTimer(
                auction.id,
                () => updatePriceDisplay(auction.id, 'seller'),
                decrementInterval
            );
        } else {
            // 买家区域，启动买家区域定时器
            const decrementInterval = (auction.decrementInterval || 1) * 1000; // 转换为毫秒
            window.timerManager.startBuyerPriceDecrementTimer(
                auction.id,
                () => updatePriceDisplay(auction.id, 'buyer'),
                decrementInterval
            );
            
            // 如果有可视化器，也启动可视化器定时器
            if (window.dutchClockVisualizer && window.dutchClockVisualizer.isVisible) {
                window.timerManager.startVisualTimer('price-decrement', auction, 1000);
            }
        }
    } catch (error) {
        console.error('启动价格递减定时器失败:', error);
    }
}

// 计算剩余时间
function calculateRemainingSeconds(auction) {
    const startPrice = auction.initialPrice || auction.startPrice || 0;
    const priceRange = startPrice - (auction.minPrice || 0);
    const currentPriceOffset = (auction.currentPrice || 0) - (auction.minPrice || 0);
    const remainingDecrementSteps = Math.ceil(currentPriceOffset / (auction.priceDecrement || 1));
    return remainingDecrementSteps * (auction.decrementInterval || 1);
}

// 快速竞拍
async function placeQuickBid(auctionId, bidAmount) {
    try {
        const response = await fetch('/api/dutch-auction/bid', {
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

            // 使用统一定时器管理系统清除所有相关定时器
            if (window.timerManager) {
                // 清除卖家区域的价格递减定时器
                window.timerManager.stopSellerPriceDecrementTimer(auctionId);
                // 清除买家区域的价格递减定时器
                window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
                // 清除可视化器定时器
                window.timerManager.stopVisualTimer('price-decrement');
            }

            // 刷新页面数据
            loadBalance();
            loadAuctions();
            loadSellerAuctions();

            // 重新加载当前拍卖以更新状态
            loadAuctionForVisualization(auctionId);
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
        const response = await fetch('/api/market/balance');
        if (!response.ok) {
            throw new Error('Failed to load balance');
        }

        const balance = await response.json();

        // 更新余额显示
        const balanceAmount = document.getElementById('balanceAmount');
        if (balanceAmount) {
            balanceAmount.textContent = formatCurrency(balance.amount);
        }

        // 更新最后更新时间
        const lastUpdated = document.getElementById('lastUpdated');
        if (lastUpdated) {
            const date = new Date(balance.updated_at);
            lastUpdated.textContent = `最后更新: ${formatDateTime(date)}`;
        }
    } catch (error) {
        console.error('Error loading balance:', error);
        showNotification('加载余额失败', 'error');
    }
}

// 加载卖家拍卖列表
async function loadSellerAuctions() {
    try {
        // 使用统一定时器管理系统清除所有卖家区域定时器
        if (window.timerManager) {
            window.timerManager.stopAllSellerTimers();
        }

        const response = await fetch('/api/dutch-auction/seller-list');

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
                    startPriceDecrementTimer(auction);
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
                    <span class="price-countdown font-bold text-lg text-indigo-600">${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">${(auction.priceDecrement || 0).toFixed(2)}</span>
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
        // 使用统一定时器管理系统清除所有买家区域定时器
        if (window.timerManager) {
            window.timerManager.stopAllBuyerTimers();
        }

        const response = await fetch('/api/dutch-auction/list');

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

        // 更新可视化器的拍卖选择器
        const visualAuctionSelect = document.getElementById('visualAuctionSelect');
        if (visualAuctionSelect) {
            visualAuctionSelect.innerHTML = '<option value="">选择拍卖</option>';
        }

        if (data.auctions && data.auctions.length > 0) {
            data.auctions.forEach(auction => {
                const auctionCard = createAuctionCard(auction);
                auctionsList.appendChild(auctionCard);

                // 添加到可视化器的选择器中
                if (visualAuctionSelect) {
                    const option = document.createElement('option');
                    option.value = auction.id;

                    // 添加状态前缀
                    let statusPrefix = '';
                    if (auction.status === 'active') {
                        statusPrefix = '[进行中] ';
                    } else if (auction.status === 'pending') {
                        statusPrefix = '[待开始] ';
                    } else if (auction.status === 'completed') {
                        statusPrefix = '[已完成] ';
                    } else {
                        statusPrefix = '[已取消] ';
                    }

                    option.textContent = statusPrefix + `${auction.itemType === 'apple' ? '苹果' : '木材'} x${auction.quantity} - ¥${(auction.currentPrice || 0).toFixed(2)}`;
                    visualAuctionSelect.appendChild(option);

                    // 如果拍卖已开始且未结束，启动递减价格计时器
                    if (auction.status === 'active') {
                        startPriceDecrementTimer(auction);
                    }
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

// 创建拍卖卡片
function createAuctionCard(auction) {
    const card = document.createElement('div');
    card.className = 'auction-card bg-white rounded-lg shadow-md overflow-hidden';
    card.id = `auction-${auction.id}`;

    const statusClass = auction.status === 'active' ? 'bg-green-100 text-green-800' :
        auction.status === 'pending' ? 'bg-yellow-100 text-yellow-800' :
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
                    <span class="price-countdown font-bold text-lg text-indigo-600">${(auction.currentPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${(auction.minPrice || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减价格:</span>
                    <span class="font-medium">${(auction.priceDecrement || 0).toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">递减间隔:</span>
                    <span class="font-medium">${auction.decrementInterval}秒</span>
                </div>
            </div>
            <div class="mt-4 flex space-x-2">
                ${auction.status === 'active' ?
            `<button class="bid-button flex-1 bg-indigo-600 text-white py-2 px-4 rounded-md hover:bg-indigo-700 transition" 
                            onclick="openBidModal(${auction.id}, '${auction.itemType}', ${auction.currentPrice}, ${auction.minPrice}, ${auction.quantity})">
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

    return card;
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
        const response = await fetch('/api/dutch-auction/create', {
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
        const response = await fetch('/api/dutch-auction/pause', {
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

        // 使用统一定时器管理系统清除所有相关定时器
        if (window.timerManager) {
            // 清除卖家区域的价格递减定时器
            window.timerManager.stopSellerPriceDecrementTimer(auctionId);
            // 清除买家区域的价格递减定时器
            window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
            // 清除可视化器定时器
            window.timerManager.stopVisualTimer('price-decrement');
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
        const response = await fetch('/api/dutch-auction/start', {
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
        const response = await fetch('/api/dutch-auction/cancel', {
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

            // 使用统一定时器管理系统清除所有相关定时器
            if (window.timerManager) {
                // 清除卖家区域的价格递减定时器
                window.timerManager.stopSellerPriceDecrementTimer(auctionId);
                // 清除买家区域的价格递减定时器
                window.timerManager.stopBuyerPriceDecrementTimer(auctionId);
                // 清除可视化器定时器
                window.timerManager.stopVisualTimer('price-decrement');
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
function openBidModal(auctionId, itemType, currentPrice, minPrice, quantity) {
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
                <span class="font-bold text-indigo-600">${(currentPrice || 0).toFixed(2)}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">最低价格:</span>
                <span class="font-medium">${(minPrice || 0).toFixed(2)}</span>
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
async function placeBid() {
    const bidModal = document.getElementById('bidModal');
    const auctionId = parseInt(bidModal.dataset.auctionId);
    const bidAmount = parseInt(document.getElementById('bidAmount').value);

    try {
        const response = await fetch('/api/dutch-auction/bid', {
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
                // 清除可视化器定时器
                window.timerManager.stopVisualTimer('price-decrement');
            }

            loadBalance();
            loadAuctions();
            loadSellerAuctions();

            // 如果当前在可视化器中显示的是这个拍卖，更新其状态
            const visualAuctionSelect = document.getElementById('visualAuctionSelect');
            if (visualAuctionSelect.value == auctionId) {
                loadAuctionForVisualization(auctionId);
            }
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
        const response = await fetch('/api/dutch-auction/end', {
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
                // 清除可视化器定时器
                window.timerManager.stopVisualTimer();
            }

            // 刷新拍卖列表
            loadAuctions();
            loadSellerAuctions();

            // 如果当前在可视化器中显示的是这个拍卖，更新其状态
            const visualAuctionSelect = document.getElementById('visualAuctionSelect');
            if (visualAuctionSelect && visualAuctionSelect.value == auctionId) {
                loadAuctionForVisualization(auctionId);
            }
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