document.addEventListener('DOMContentLoaded', () => {
    // 初始化页面
    loadBalance();
    loadAuctions();
    loadSellerAuctions();

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
        showToast('加载余额失败', 'error');
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
        
        sellerAuctionsList.innerHTML = '';
        
        if (data.auctions && data.auctions.length > 0) {
            noSellerAuctions.classList.add('hidden');
            data.auctions.forEach(auction => {
                const auctionCard = createSellerAuctionCard(auction);
                sellerAuctionsList.appendChild(auctionCard);
                
                // 如果拍卖已开始且未结束，启动价格递减计时器
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
                    <span class="price-countdown font-bold text-lg text-indigo-600">${auction.currentPrice.toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${auction.minPrice.toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">价格递减:</span>
                    <span class="font-medium">${auction.priceDecrement.toFixed(2)}</span>
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
        auctionsList.innerHTML = '';
        
        if (data.auctions && data.auctions.length > 0) {
            data.auctions.forEach(auction => {
                const auctionCard = createAuctionCard(auction);
                auctionsList.appendChild(auctionCard);
                
                // 如果拍卖已开始且未结束，启动价格递减计时器
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
                    <span class="price-countdown font-bold text-lg text-indigo-600">${auction.currentPrice.toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">最低价格:</span>
                    <span class="font-medium">${auction.minPrice.toFixed(2)}</span>
                </div>
                <div class="flex justify-between">
                    <span class="text-gray-600">价格递减:</span>
                    <span class="font-medium">${auction.priceDecrement.toFixed(2)}</span>
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
                <span class="font-bold text-indigo-600">${currentPrice.toFixed(2)}</span>
            </div>
            <div class="flex justify-between">
                <span class="text-gray-600">最低价格:</span>
                <span class="font-medium">${minPrice.toFixed(2)}</span>
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

// 启动价格递减计时器
function startPriceDecrementTimer(auctionId, currentPrice, minPrice, decrementAmount, intervalSeconds) {
    let price = currentPrice;
    const priceElement = document.querySelector(`#auction-${auctionId} .price-countdown`);
    
    if (!priceElement) return;
    
    const timer = setInterval(() => {
        price -= decrementAmount;
        
        if (price <= minPrice) {
            price = minPrice;
            clearInterval(timer);
            
            // 价格达到最低价，自动结束拍卖
            endAuction(auctionId);
        }
        
        priceElement.textContent = price.toFixed(2);
    }, intervalSeconds * 1000);
    
    // 将计时器ID存储在元素上，以便在需要时清除
    priceElement.dataset.timerId = timer;
}

// 结束拍卖
async function endAuction(auctionId) {
    try {
        const response = await fetch('/api/dutch-auction/get', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                auction_id: auctionId
            })
        });
        
        const data = await response.json();
        
        if (data.auction && data.auction.status !== 'completed') {
            // 如果拍卖还没有结束标记，说明需要手动结束
            // 这里可以添加结束拍卖的逻辑，或者等待系统自动处理
            loadAuctions();
        }
    } catch (error) {
        console.error('结束拍卖失败:', error);
    }
}

// 显示通知
function showNotification(message, type = 'success') {
    const notification = document.getElementById('notification');
    const notificationMessage = document.getElementById('notificationMessage');
    
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