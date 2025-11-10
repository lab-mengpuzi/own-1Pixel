/**
 * 统一定时器管理系统
 * 用于管理荷兰式拍卖系统中的所有定时器，避免定时器泄漏和重复创建
 */
class AuctionTimerManager {
    constructor() {
        // 存储所有定时器ID的映射
        this.timers = {
            // 卖家区域定时器
            sellerPriceDecrementTimers: {}, // 卖家区域价格递减定时器
            
            // 买家区域定时器
            buyerPriceDecrementTimers: {} // 买家区域价格递减定时器
        };
        
        // 定时器类型常量
        this.TIMER_TYPES = {
            SELLER_PRICE_DECREMENT: 'sellerPriceDecrementTimers',
            BUYER_PRICE_DECREMENT: 'buyerPriceDecrementTimers'
        };
    }
    
    /**
     * 启动定时器
     * @param {string} type - 定时器类型
     * @param {string} id - 定时器ID
     * @param {Function} callback - 回调函数
     * @param {number} interval - 间隔时间（毫秒）
     */
    startTimer(type, id, callback, interval) {
        // 清除已存在的定时器
        this.stopTimer(type, id);
        
        // 创建新定时器
        const timerId = setInterval(callback, interval);
        this.timers[type][id] = timerId;
        
        return timerId;
    }
    
    /**
     * 停止单个定时器
     * @param {string} type - 定时器类型
     * @param {string} id - 定时器ID
     */
    stopTimer(type, id) {
        if (this.timers[type] && this.timers[type][id]) {
            clearInterval(this.timers[type][id]);
            delete this.timers[type][id];
        }
    }
    
    /**
     * 停止所有卖家区域定时器
     */
    stopAllSellerTimers() {
        this.stopAllTimersByType(this.TIMER_TYPES.SELLER_PRICE_DECREMENT);
    }
    
    /**
     * 停止所有买家区域定时器
     */
    stopAllBuyerTimers() {
        this.stopAllTimersByType(this.TIMER_TYPES.BUYER_PRICE_DECREMENT);
    }
    
    /**
     * 停止所有定时器
     */
    stopAllTimers() {
        Object.keys(this.timers).forEach(type => {
            this.stopAllTimersByType(type);
        });
    }
    
    /**
     * 停止指定类型的所有定时器
     * @param {string} type - 定时器类型
     */
    stopAllTimersByType(type) {
        if (this.timers[type]) {
            Object.keys(this.timers[type]).forEach(id => {
                clearInterval(this.timers[type][id]);
            });
            this.timers[type] = {};
        }
    }
    
    /**
     * 启动卖家区域价格递减定时器
     * @param {string} id - 拍卖ID
     * @param {Function} callback - 回调函数
     * @param {number} interval - 间隔时间（毫秒）
     */
    startSellerPriceDecrementTimer(id, callback, interval) {
        return this.startTimer(this.TIMER_TYPES.SELLER_PRICE_DECREMENT, id, callback, interval);
    }
    
    /**
     * 停止卖家区域价格递减定时器
     * @param {string} id - 拍卖ID
     */
    stopSellerPriceDecrementTimer(id) {
        this.stopTimer(this.TIMER_TYPES.SELLER_PRICE_DECREMENT, id);
    }
    
    /**
     * 启动买家区域价格递减定时器
     * @param {string} id - 拍卖ID
     * @param {Function} callback - 回调函数
     * @param {number} interval - 间隔时间（毫秒）
     */
    startBuyerPriceDecrementTimer(id, callback, interval) {
        return this.startTimer(this.TIMER_TYPES.BUYER_PRICE_DECREMENT, id, callback, interval);
    }
    
    /**
     * 停止买家区域价格递减定时器
     * @param {string} id - 拍卖ID
     */
    stopBuyerPriceDecrementTimer(id) {
        this.stopTimer(this.TIMER_TYPES.BUYER_PRICE_DECREMENT, id);
    }
    
    /**
     * 获取指定类型的定时器数量
     * @param {string} type - 定时器类型
     * @returns {number} 定时器数量
     */
    getTimerCount(type) {
        if (this.timers[type]) {
            return Object.keys(this.timers[type]).length;
        }
        return 0;
    }
    
    /**
     * 获取所有定时器的数量
     * @returns {number} 定时器总数量
     */
    getAllTimerCount() {
        let count = 0;
        Object.keys(this.timers).forEach(type => {
            count += this.getTimerCount(type);
        });
        return count;
    }
    
    /**
     * 检查指定定时器是否存在
     * @param {string} type - 定时器类型
     * @param {string} id - 定时器ID
     * @returns {boolean} 是否存在
     */
    hasTimer(type, id) {
        return !!(this.timers[type] && this.timers[type][id]);
    }
    
    /**
     * 获取定时器信息
     * @returns {Object} 定时器信息
     */
    getTimerInfo() {
        const info = {};
        Object.keys(this.timers).forEach(type => {
            info[type] = {
                count: this.getTimerCount(type),
                ids: Object.keys(this.timers[type])
            };
        });
        return info;
    }
}

// 创建全局定时器管理器实例
window.timerManager = new AuctionTimerManager();

// 导出AuctionTimerManager类（如果使用模块系统）
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AuctionTimerManager;
}