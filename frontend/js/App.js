// 现金余额跟踪系统前端脚本

// 当交易时间改变时，自动更新季度
document.addEventListener('DOMContentLoaded', function() {
    const transactionTimeInput = document.getElementById('transaction_time');
    
    // 设置交易时间初始值
    if (transactionTimeInput) {
        const now = new Date();
        const year = now.getFullYear();
        const month = String(now.getMonth() + 1).padStart(2, '0');
        const day = String(now.getDate()).padStart(2, '0');
        const hours = String(now.getHours()).padStart(2, '0');
        const minutes = String(now.getMinutes()).padStart(2, '0');
        const seconds = String(now.getSeconds()).padStart(2, '0');
        transactionTimeInput.value = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    }
    
    // 添加输入格式化功能
    if (transactionTimeInput) {
        transactionTimeInput.addEventListener('input', function() {
            let value = this.value;
            
            // 自动添加日期分隔符
            if (value.length === 4 && !value.includes('-')) {
                value = value + '-';
            } else if (value.length === 7 && value.charAt(4) === '-' && value.charAt(7) !== '-') {
                value = value + '-';
            } else if (value.length === 10 && value.charAt(7) === '-' && value.charAt(10) !== ' ') {
                value = value + ' ';
            } else if (value.length === 13 && value.charAt(10) === ' ' && value.charAt(13) !== ':') {
                value = value + ':';
            } else if (value.length === 16 && value.charAt(13) === ':' && value.charAt(16) !== ':') {
                value = value + ':';
            }
            
            // 限制输入长度
            if (value.length > 19) {
                value = value.substring(0, 19);
            }
            
            this.value = value;
        });
    }

    // 加载余额和交易记录
    loadBalance();
    loadTransactions();

    // 表单提交事件
    const transactionForm = document.getElementById('transactionForm');
    if (transactionForm) {
        transactionForm.addEventListener('submit', handleFormSubmit);
    }

    // 刷新按钮事件
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', function() {
            loadBalance();
            loadTransactions();
        });
    }
});

// 加载余额信息
async function loadBalance() {
    try {
        const response = await fetch('/api/cash/balance');
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

// 加载交易记录
async function loadTransactions() {
    try {
        const response = await fetch('/api/cash/transactions');
        if (!response.ok) {
            throw new Error('Failed to load transactions');
        }
        
        const transactions = await response.json();
        
        // 更新交易记录列表
        const transactionsContainer = document.getElementById('transactionsContainer');
        const noTransactions = document.getElementById('noTransactions');
        
        if (transactionsContainer) {
            // 清空现有列表
            transactionsContainer.innerHTML = '';
            
            // 检查transactions是否为null或undefined
            if (!transactions || transactions.length === 0) {
                // 显示无记录提示
                if (noTransactions) {
                    noTransactions.classList.remove('hidden');
                }
            } else {
                // 隐藏无记录提示
                if (noTransactions) {
                    noTransactions.classList.add('hidden');
                }
                
                // 添加交易记录到容器
                transactions.forEach(transaction => {
                    const card = document.createElement('div');
                    card.className = 'bg-gray-50 rounded-lg p-4 border border-gray-200 hover:shadow-md transition-shadow duration-200';
                    
                    // 根据余额确定卡片边框颜色
                    let borderColor = 'border-green-200';
                    if (transaction.balance < 0) {
                        borderColor = 'border-red-200';
                    }
                    card.classList.add(borderColor);
                    
                    card.innerHTML = `
                        <div class="flex justify-between items-start mb-2">
                            <div class="text-sm font-medium text-gray-900">${formatFullDateTime(new Date(transaction.transaction_time))}</div>
                            <div class="text-sm font-medium ${transaction.balance >= 0 ? 'text-green-600' : 'text-red-600'}">
                                余额: ${formatCurrency(transaction.balance)}
                            </div>
                        </div>
                        <div class="grid grid-cols-2 gap-2 text-sm">
                            <div>
                                <span class="text-gray-500">己方户名:</span>
                                <div class="font-medium">${transaction.our_bank_account_name || '-'}</div>
                            </div>
                            <div>
                                <span class="text-gray-500">对手方:</span>
                                <div class="font-medium">${transaction.counterparty_alias || '-'}</div>
                            </div>
                            <div>
                                <span class="text-gray-500">己方银行:</span>
                                <div class="font-medium">${transaction.our_bank_name || '-'}</div>
                            </div>
                            <div>
                                <span class="text-gray-500">对手银行:</span>
                                <div class="font-medium">${transaction.counterparty_bank || '-'}</div>
                            </div>
                        </div>
                        <div class="flex justify-between mt-3 pt-2 border-t border-gray-200">
                            <div class="text-sm">
                                <span class="text-gray-500">支出:</span>
                                <span class="font-medium text-red-600 ml-1">${formatCurrency(transaction.expense_amount)}</span>
                            </div>
                            <div class="text-sm">
                                <span class="text-gray-500">收入:</span>
                                <span class="font-medium text-green-600 ml-1">${formatCurrency(transaction.income_amount)}</span>
                            </div>
                        </div>
                        ${transaction.note ? `
                        <div class="mt-2 pt-2 border-t border-gray-200">
                            <span class="text-gray-500 text-sm">用途:</span>
                            <div class="text-sm text-gray-700 mt-1">${transaction.note}</div>
                        </div>
                        ` : ''}
                    `;
                    
                    transactionsContainer.appendChild(card);
                });
            }
        }
    } catch (error) {
        console.error('Error loading transactions:', error);
        showToast('加载交易记录失败', 'error');
    }
}

// 处理表单提交
async function handleFormSubmit(event) {
    event.preventDefault();
    
    const form = event.target;
    const formData = new FormData(form);
    
    // 构建交易对象（不包含交易时间，由后端自动生成）
    const transaction = {
        our_bank_account_name: formData.get('our_bank_account_name'),
        counterparty_alias: formData.get('counterparty_alias'),
        our_bank_name: formData.get('our_bank_name'),
        counterparty_bank: formData.get('counterparty_bank'),
        expense_amount: parseFloat(formData.get('expense_amount')) || 0,
        income_amount: parseFloat(formData.get('income_amount')) || 0,
        note: formData.get('note') || ''
    };
    
    try {
        const response = await fetch('/api/cash/transactions', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(transaction)
        });
        
        if (!response.ok) {
            throw new Error('Failed to add transaction');
        }
        
        // 重置表单
        form.reset();
        
        // 刷新余额和交易记录
        loadBalance();
        loadTransactions();
        
        showToast('交易记录添加成功', 'success');
    } catch (error) {
        console.error('Error adding transaction:', error);
        showToast('添加交易记录失败', 'error');
    }
}

// 格式化日期 (YYYY-MM-DD)
function formatDate(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

// 格式化完整日期时间 (YYYY-MM-DD HH:MM:SS)
function formatFullDateTime(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
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