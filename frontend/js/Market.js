// 市场页 - 动态供需平衡定价系统前端脚本

// 全局变量
let marketParams = {
    balanceRange: 1,      // 平衡区间系数
    priceFluctuation: 1,  // 价格波动系数
    maxPriceChange: 1     // 最大价格变动系数
};

let backpack = {
    apple: 0,
    wood: 0
};

let marketItems = {
    apple: {
        price: 1.0,
        stock: 0,
        basePrice: 1.0
    },
    wood: {
        price: 5.0,
        stock: 0,
        basePrice: 5.0
    }
};

// 当页面加载完成时执行
document.addEventListener('DOMContentLoaded', function() {
    // 加载市场参数
    loadMarketParams();
    
    // 加载背包状态
    loadBackpack();
    
    // 加载市场货架
    loadMarketItems();
    
    // 加载现金余额
    loadBalance();
    
    // 绑定事件监听器
    bindEventListeners();
});

// 绑定事件监听器
function bindEventListeners() {
    // 保存市场参数按钮
    const saveMarketParamsBtn = document.getElementById('saveMarketParams');
    if (saveMarketParamsBtn) {
        saveMarketParamsBtn.addEventListener('click', saveMarketParams);
    }
    
    // 制作苹果按钮
    const makeAppleBtn = document.getElementById('makeApple');
    if (makeAppleBtn) {
        makeAppleBtn.addEventListener('click', makeApple);
    }
    
    // 制作木材按钮
    const makeWoodBtn = document.getElementById('makeWood');
    if (makeWoodBtn) {
        makeWoodBtn.addEventListener('click', makeWood);
    }
    
    // 卖出苹果按钮
    const sellAppleBtn = document.getElementById('sellApple');
    if (sellAppleBtn) {
        sellAppleBtn.addEventListener('click', () => sellItem('apple'));
    }
    
    // 卖出木材按钮
    const sellWoodBtn = document.getElementById('sellWood');
    if (sellWoodBtn) {
        sellWoodBtn.addEventListener('click', () => sellItem('wood'));
    }
    
    // 买入苹果按钮
    const buyAppleBtn = document.getElementById('buyApple');
    if (buyAppleBtn) {
        buyAppleBtn.addEventListener('click', () => buyItem('apple'));
    }
    
    // 买入木材按钮
    const buyWoodBtn = document.getElementById('buyWood');
    if (buyWoodBtn) {
        buyWoodBtn.addEventListener('click', () => buyItem('wood'));
    }
    
    // 刷新市场按钮
    const refreshMarketBtn = document.getElementById('refreshMarket');
    if (refreshMarketBtn) {
        refreshMarketBtn.addEventListener('click', refreshMarket);
    }
}

// 加载市场参数
async function loadMarketParams() {
    try {
        const response = await fetch('/api/market/params');
        if (response.ok) {
            const params = await response.json();
            marketParams = params;
            
            // 更新UI
            const balanceRangeInput = document.getElementById('balanceRange');
            const priceFluctuationInput = document.getElementById('priceFluctuation');
            const maxPriceChangeInput = document.getElementById('maxPriceChange');
            
            if (balanceRangeInput) balanceRangeInput.value = marketParams.balanceRange;
            if (priceFluctuationInput) priceFluctuationInput.value = marketParams.priceFluctuation;
            if (maxPriceChangeInput) maxPriceChangeInput.value = marketParams.maxPriceChange;
        }
    } catch (error) {
        console.error('Error loading market params:', error);
    }
}

// 保存市场参数
async function saveMarketParams() {
    const balanceRangeInput = document.getElementById('balanceRange');
    const priceFluctuationInput = document.getElementById('priceFluctuation');
    const maxPriceChangeInput = document.getElementById('maxPriceChange');
    
    if (!balanceRangeInput || !priceFluctuationInput || !maxPriceChangeInput) {
        showToast('无法获取参数输入框', 'error');
        return;
    }
    
    const params = {
        balanceRange: parseFloat(balanceRangeInput.value),
        priceFluctuation: parseFloat(priceFluctuationInput.value),
        maxPriceChange: parseFloat(maxPriceChangeInput.value)
    };
    
    try {
        const response = await fetch('/api/market/params', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(params)
        });
        
        if (response.ok) {
            marketParams = params;
            showToast('市场参数保存成功', 'success');
        } else {
            showToast('保存市场参数失败', 'error');
        }
    } catch (error) {
        console.error('Error saving market params:', error);
        showToast('保存市场参数失败', 'error');
    }
}

// 加载背包状态
async function loadBackpack() {
    try {
        const response = await fetch('/api/market/backpack');
        if (response.ok) {
            const data = await response.json();
            backpack = data;
            
            // 更新UI
            const backpackAppleCount = document.getElementById('backpackAppleCount');
            const backpackWoodCount = document.getElementById('backpackWoodCount');
            
            if (backpackAppleCount) backpackAppleCount.textContent = backpack.apple;
            if (backpackWoodCount) backpackWoodCount.textContent = backpack.wood;
        }
    } catch (error) {
        console.error('Error loading backpack:', error);
    }
}

// 加载市场货架
async function loadMarketItems() {
    try {
        const response = await fetch('/api/market/items');
        if (response.ok) {
            const items = await response.json();
            marketItems = items;
            
            // 更新UI
            updateMarketUI();
        }
    } catch (error) {
        console.error('Error loading market items:', error);
    }
}

// 更新市场UI
function updateMarketUI() {
    // 更新苹果信息
    const applePrice = document.getElementById('applePrice');
    const appleStock = document.getElementById('appleStock');
    const appleStockBar = document.getElementById('appleStockBar');
    const appleMarketStatus = document.getElementById('appleMarketStatus');
    
    if (applePrice) applePrice.textContent = marketItems.apple.price.toFixed(2);
    if (appleStock) appleStock.textContent = marketItems.apple.stock;
    
    // 更新苹果库存条
    if (appleStockBar) {
        const stockPercentage = Math.min(100, marketItems.apple.stock * 10); // 假设10个库存为100%
        appleStockBar.style.width = `${stockPercentage}%`;
        
        // 根据库存量设置颜色
        if (marketItems.apple.stock > 10) {
            appleStockBar.className = 'bg-danger h-2.5 rounded-full'; // 供过于求，红色
        } else if (marketItems.apple.stock >= 5) {
            appleStockBar.className = 'bg-success h-2.5 rounded-full'; // 供需平衡，绿色
        } else {
            appleStockBar.className = 'bg-warning h-2.5 rounded-full'; // 供不应求，黄色
        }
    }
    
    // 更新苹果市场状态
    if (appleMarketStatus) {
        let status = '稳定';
        let statusClass = 'text-success';
        
        if (marketItems.apple.stock > 10) {
            status = '供过于求';
            statusClass = 'text-danger';
        } else if (marketItems.apple.stock < 5) {
            status = '供不应求';
            statusClass = 'text-warning';
        }
        
        appleMarketStatus.textContent = status;
        appleMarketStatus.className = `font-medium ${statusClass}`;
    }
    
    // 更新木材信息
    const woodPrice = document.getElementById('woodPrice');
    const woodStock = document.getElementById('woodStock');
    const woodStockBar = document.getElementById('woodStockBar');
    const woodMarketStatus = document.getElementById('woodMarketStatus');
    
    if (woodPrice) woodPrice.textContent = marketItems.wood.price.toFixed(2);
    if (woodStock) woodStock.textContent = marketItems.wood.stock;
    
    // 更新木材库存条
    if (woodStockBar) {
        const stockPercentage = Math.min(100, marketItems.wood.stock * 10); // 假设10个库存为100%
        woodStockBar.style.width = `${stockPercentage}%`;
        
        // 根据库存量设置颜色
        if (marketItems.wood.stock > 10) {
            woodStockBar.className = 'bg-danger h-2.5 rounded-full'; // 供过于求，红色
        } else if (marketItems.wood.stock >= 5) {
            woodStockBar.className = 'bg-success h-2.5 rounded-full'; // 供需平衡，绿色
        } else {
            woodStockBar.className = 'bg-warning h-2.5 rounded-full'; // 供不应求，黄色
        }
    }
    
    // 更新木材市场状态
    if (woodMarketStatus) {
        let status = '稳定';
        let statusClass = 'text-success';
        
        if (marketItems.wood.stock > 10) {
            status = '供过于求';
            statusClass = 'text-danger';
        } else if (marketItems.wood.stock < 5) {
            status = '供不应求';
            statusClass = 'text-warning';
        }
        
        woodMarketStatus.textContent = status;
        woodMarketStatus.className = `font-medium ${statusClass}`;
    }
}

// 加载现金余额
async function loadBalance() {
    try {
        const response = await fetch('/api/cash/balance');
        if (response.ok) {
            const balance = await response.json();
            
            // 更新余额显示
            const balanceAmount = document.getElementById('marketBalanceAmount');
            if (balanceAmount) {
                balanceAmount.textContent = formatCurrency(balance.amount);
            }
            
            // 更新最后更新时间
            const lastUpdated = document.getElementById('marketLastUpdated');
            if (lastUpdated) {
                const date = new Date(balance.updated_at);
                lastUpdated.textContent = `最后更新: ${formatDateTime(date)}`;
            }
        }
    } catch (error) {
        console.error('Error loading balance:', error);
    }
}

// 制作苹果
async function makeApple() {
    try {
        const response = await fetch('/api/market/make-apple', {
            method: 'POST'
        });
        
        if (response.ok) {
            const result = await response.json();
            
            // 更新背包
            backpack.apple = result.backpack.apple;
            
            // 更新UI
            const backpackAppleCount = document.getElementById('backpackAppleCount');
            if (backpackAppleCount) backpackAppleCount.textContent = backpack.apple;
            
            showToast('制作苹果成功', 'success');
        } else {
            showToast('制作苹果失败', 'error');
        }
    } catch (error) {
        console.error('Error making apple:', error);
        showToast('制作苹果失败', 'error');
    }
}

// 制作木材
async function makeWood() {
    try {
        const response = await fetch('/api/market/make-wood', {
            method: 'POST'
        });
        
        if (response.ok) {
            const result = await response.json();
            
            // 更新背包
            backpack.wood = result.backpack.wood;
            
            // 更新UI
            const backpackWoodCount = document.getElementById('backpackWoodCount');
            if (backpackWoodCount) backpackWoodCount.textContent = backpack.wood;
            
            showToast('制作木材成功', 'success');
        } else {
            showToast('制作木材失败', 'error');
        }
    } catch (error) {
        console.error('Error making wood:', error);
        showToast('制作木材失败', 'error');
    }
}

// 卖出物品
async function sellItem(itemType) {
    if (backpack[itemType] <= 0) {
        showToast(`背包中没有${itemType === 'apple' ? '苹果' : '木材'}`, 'warning');
        return;
    }
    
    try {
        const response = await fetch(`/api/market/sell-${itemType}`, {
            method: 'POST'
        });
        
        if (response.ok) {
            const result = await response.json();
            
            // 更新背包
            backpack = result.backpack;
            
            // 更新市场物品
            marketItems = result.marketItems;
            
            // 更新UI
            updateBackpackUI();
            updateMarketUI();
            loadBalance(); // 更新余额
            
            showToast(`卖出${itemType === 'apple' ? '苹果' : '木材'}成功`, 'success');
        } else {
            showToast(`卖出${itemType === 'apple' ? '苹果' : '木材'}失败`, 'error');
        }
    } catch (error) {
        console.error(`Error selling ${itemType}:`, error);
        showToast(`卖出${itemType === 'apple' ? '苹果' : '木材'}失败`, 'error');
    }
}

// 买入物品
async function buyItem(itemType) {
    try {
        const response = await fetch(`/api/market/buy-${itemType}`, {
            method: 'POST'
        });
        
        if (response.ok) {
            const result = await response.json();
            
            // 更新背包
            backpack = result.backpack;
            
            // 更新市场物品
            marketItems = result.marketItems;
            
            // 更新UI
            updateBackpackUI();
            updateMarketUI();
            loadBalance(); // 更新余额
            
            showToast(`买入${itemType === 'apple' ? '苹果' : '木材'}成功`, 'success');
        } else {
            const error = await response.json();
            showToast(error.message || `买入${itemType === 'apple' ? '苹果' : '木材'}失败`, 'error');
        }
    } catch (error) {
        console.error(`Error buying ${itemType}:`, error);
        showToast(`买入${itemType === 'apple' ? '苹果' : '木材'}失败`, 'error');
    }
}

// 更新背包UI
function updateBackpackUI() {
    const backpackAppleCount = document.getElementById('backpackAppleCount');
    const backpackWoodCount = document.getElementById('backpackWoodCount');
    
    if (backpackAppleCount) backpackAppleCount.textContent = backpack.apple;
    if (backpackWoodCount) backpackWoodCount.textContent = backpack.wood;
}

// 刷新市场
function refreshMarket() {
    loadMarketParams();
    loadBackpack();
    loadMarketItems();
    loadBalance();
    showToast('市场数据已刷新', 'success');
}

// 格式化日期时间 (YYYY-MM-DD HH:MM)
function formatDateTime(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}`;
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

// 显示提示消息
function showToast(message, type = 'info') {
    // 创建提示元素
    const toast = document.createElement('div');
    
    // 根据类型设置样式
    let bgColor = 'bg-blue-500';
    if (type === 'success') {
        bgColor = 'bg-green-500';
    } else if (type === 'error') {
        bgColor = 'bg-red-500';
    } else if (type === 'warning') {
        bgColor = 'bg-yellow-500';
    }
    
    // 设置提示样式和内容
    toast.className = `fixed bottom-4 right-4 ${bgColor} text-white px-4 py-2 rounded-md shadow-lg z-50 transition-opacity duration-300`;
    toast.textContent = message;
    
    // 添加到页面
    document.body.appendChild(toast);
    
    // 3秒后自动消失
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => {
            document.body.removeChild(toast);
        }, 300);
    }, 3000);
}