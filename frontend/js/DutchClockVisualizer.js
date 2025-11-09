/**
 * 荷兰钟可视化类
 * 基于二维笛卡尔坐标系实现荷兰钟动画效果
 */
class DutchClockVisualizer {
    constructor(canvasId) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            throw new Error(`Canvas element with id '${canvasId}' not found`);
        }
        
        this.ctx = this.canvas.getContext('2d');
        this.centerX = this.canvas.width / 2;
        this.centerY = this.canvas.height / 2;
        this.radius = Math.min(this.canvas.width, this.canvas.height) / 2 - 20;
        
        // 钟表状态
        this.currentAngle = 0; // 当前角度（0度为顶部）
        this.targetAngle = 0; // 目标角度
        this.animationSpeed = 0.02; // 动画速度
        
        // 拍卖数据
        this.auction = null;
        this.currentPrice = 0;
        this.minPrice = 0;
        this.maxPrice = 0;
        this.priceDecrement = 0;
        this.decrementInterval = 1; // 递减间隔（秒）
        
        // 动画控制
        this.animationFrame = null;
        this.isAnimating = false;
        
        // Canvas刷新控制
        this.canvasRefreshInterval = null;
        this.lastUpdateTime = 0;
        
        // 价格刻度
        this.priceTicks = [];
        
        // 初始化
        this.init();
    }
    
    /**
     * 初始化荷兰钟
     */
    init() {
        this.drawClock();
    }
    
    /**
     * 设置拍卖数据
     * @param {Object} auction - 拍卖数据
     */
    setAuction(auction) {
        this.auction = auction;
        this.currentPrice = auction.currentPrice || 0;
        this.minPrice = auction.minPrice || 0;
        this.maxPrice = auction.initialPrice || auction.startPrice || 0;
        this.priceDecrement = auction.priceDecrement || 0;
        this.decrementInterval = auction.decrementInterval || 1; // 默认1秒
        
        // 初始化最后更新时间
        this.lastUpdateTime = Date.now();
        
        // 计算当前价格对应的角度
        this.updateAngleFromPrice();
        
        // 生成价格刻度
        this.generatePriceTicks();
        
        // 开始动画
        this.startAnimation();
    }
    
    /**
     * 根据价格更新角度
     */
    updateAngleFromPrice() {
        if (this.maxPrice <= this.minPrice) {
            this.targetAngle = 0;
            return;
        }
        
        // 计算价格范围
        const priceRange = this.maxPrice - this.minPrice;
        const currentPriceOffset = this.currentPrice - this.minPrice;
        
        // 计算角度比例（逆时针方向，从初始价格到最低价格）
        // 保留一个刻度的间隔，避免最低价格与初始价格重叠
        const angleRatio = currentPriceOffset / priceRange;
        
        // 转换为弧度（逆时针方向，从0度开始）
        // 从初始价格开始，逆时针向最低价格递减
        // 保留一个刻度的间隔，所以实际使用范围是 0 到 360-18 度（18度约为一个刻度的间隔）
        this.targetAngle = Math.PI * 2 * (1 - angleRatio) * (0.95); // 0.95 确保保留5%的间隔
    }
    
    /**
     * 根据角度更新价格
     */
    updatePriceFromAngle() {
        if (this.maxPrice <= this.minPrice) {
            this.currentPrice = this.minPrice;
            return;
        }
        
        // 计算角度比例（逆时针方向，从初始价格到最低价格）
        // 考虑保留的间隔，所以需要除以0.95
        const angleRatio = (this.currentAngle / (Math.PI * 2)) / 0.95;
        
        // 计算价格
        const priceRange = this.maxPrice - this.minPrice;
        // 从初始价格开始，逆时针向最低价格递减
        this.currentPrice = this.maxPrice - (priceRange * angleRatio);
        
        // 确保价格不低于最低价格
        this.currentPrice = Math.max(this.minPrice, this.currentPrice);
        
        // 更新显示
        this.updatePriceDisplay();
    }
    
    /**
     * 生成价格刻度
     */
    generatePriceTicks() {
        this.priceTicks = [];
        
        // 根据初始价格和递减价格计算刻度数量
        const totalDecrementSteps = Math.ceil((this.maxPrice - this.minPrice) / this.priceDecrement);
        const tickCount = Math.min(totalDecrementSteps, 20); // 限制最多20个刻度
        
        for (let i = 0; i <= tickCount; i++) {
            // 逆时针方向，从0度开始
            // 保留一个刻度的间隔，所以实际使用范围是 0 到 360-18 度
            const angle = (Math.PI * 2 * 0.95 * i) / tickCount;
            
            // 从初始价格开始，逆时针向最低价格递减
            const price = this.maxPrice - ((this.maxPrice - this.minPrice) * (i / tickCount));
            
            this.priceTicks.push({
                angle: angle,
                price: price,
                label: this.formatPrice(price)
            });
        }
    }
    
    /**
     * 格式化价格
     */
    formatPrice(price) {
        return `¥${(price || 0).toFixed(0)}`;
    }
    
    /**
     * 开始动画
     */
    startAnimation() {
        if (this.isAnimating) return;
        
        this.isAnimating = true;
        
        // 启动每秒刷新canvas的计时器
        this.startCanvasRefresh();
        
        // 启动指针动画计时器
        this.startHandAnimation();
        
        // 启动平滑动画
        this.animate();
    }
    
    /**
     * 启动每秒刷新canvas的计时器
     */
    startCanvasRefresh() {
        // 使用统一定时器管理系统
        if (window.timerManager) {
            // 清除可能存在的旧可视化器定时器
            window.timerManager.stopVisualTimer('canvas-refresh');
            
            // 设置每秒刷新一次canvas
            window.timerManager.startVisualTimer('canvas-refresh', () => {
                this.lastUpdateTime = Date.now();
                this.drawClock();
            }, 1000);
        } else {
            // 回退到原始方法
            if (this.canvasRefreshInterval) {
                clearInterval(this.canvasRefreshInterval);
            }
            
            this.canvasRefreshInterval = setInterval(() => {
                this.lastUpdateTime = Date.now();
                this.drawClock();
            }, 1000);
        }
    }
    
    /**
     * 启动指针动画计时器
     * 根据递减间隔更新指针位置
     */
    startHandAnimation() {
        // 清除可能存在的旧计时器
        if (this.handAnimationInterval) {
            clearInterval(this.handAnimationInterval);
        }
        
        // 根据递减间隔设置指针更新频率
        const interval = (this.decrementInterval || 1) * 1000; // 转换为毫秒
        
        // 确保指针从当前价格对应的角度开始
        this.updateAngleFromPrice();
        
        this.handAnimationInterval = setInterval(() => {
            // 计算新的目标角度（根据递减价格）
            // 逆时针方向，角度增加
            const priceDecrementAngle = (this.priceDecrement / (this.maxPrice - this.minPrice)) * Math.PI * 2 * 0.95;
            this.targetAngle += priceDecrementAngle;
            
            // 确保不超过最大角度（考虑保留的间隔）
            const maxAngle = Math.PI * 2 * 0.95;
            if (this.targetAngle >= maxAngle) {
                this.targetAngle = maxAngle;
                clearInterval(this.handAnimationInterval);
            }
            
            // 更新当前价格
            this.updatePriceFromAngle();
        }, interval);
    }
    
    /**
     * 停止动画
     */
    stopAnimation() {
        this.isAnimating = false;
        
        // 清除动画帧
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
            this.animationFrame = null;
        }
        
        // 使用统一定时器管理系统清除可视化器定时器
        if (window.timerManager) {
            window.timerManager.stopVisualTimer('canvas-refresh');
        }
        
        // 清除canvas刷新计时器（回退方法）
        if (this.canvasRefreshInterval) {
            clearInterval(this.canvasRefreshInterval);
            this.canvasRefreshInterval = null;
        }
        
        // 清除指针动画计时器
        if (this.handAnimationInterval) {
            clearInterval(this.handAnimationInterval);
            this.handAnimationInterval = null;
        }
    }
    
    /**
     * 动画循环
     */
    animate() {
        if (!this.isAnimating) return;
        
        // 平滑过渡到目标角度
        const angleDiff = this.targetAngle - this.currentAngle;
        this.currentAngle += angleDiff * this.animationSpeed;
        
        // 更新价格显示
        this.updatePriceFromAngle();
        
        // 绘制钟表
        this.drawClock();
        
        // 继续动画
        this.animationFrame = requestAnimationFrame(() => this.animate());
    }
    
    /**
     * 绘制钟表
     */
    drawClock() {
        // 清空画布
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        
        // 绘制外圆
        this.drawCircle(this.centerX, this.centerY, this.radius, '#e5e7eb', 2);
        
        // 绘制内圆（全圆）
        this.ctx.beginPath();
        this.ctx.arc(this.centerX, this.centerY, this.radius - 10, 0, Math.PI * 2, false);
        this.ctx.strokeStyle = '#9ca3af';
        this.ctx.lineWidth = 2;
        this.ctx.stroke();
        
        // 绘制价格刻度
        this.drawPriceTicks();
        
        // 绘制指针
        this.drawHand();
        
        // 绘制中心点
        this.drawCircle(this.centerX, this.centerY, 8, '#3b82f6', 0);
        
        // 绘制角度标记
        this.drawDegreeMarkers();
    }
    
    /**
     * 绘制圆
     */
    drawCircle(x, y, radius, color, lineWidth) {
        this.ctx.beginPath();
        this.ctx.arc(x, y, radius, 0, 2 * Math.PI, false);
        this.ctx.strokeStyle = color;
        this.ctx.lineWidth = lineWidth;
        this.ctx.stroke();
    }
    
    /**
     * 绘制价格刻度
     */
    drawPriceTicks() {
        this.priceTicks.forEach((tick, index) => {
            // 计算刻度位置（逆时针方向）
            const x1 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 15);
            const y1 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 15);
            const x2 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            const y2 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            
            // 绘制刻度线
            this.ctx.beginPath();
            this.ctx.moveTo(x1, y1);
            this.ctx.lineTo(x2, y2);
            this.ctx.strokeStyle = '#6b7280';
            this.ctx.lineWidth = 1;
            this.ctx.stroke();
            
            // 绘制价格标签
            this.ctx.save();
            this.ctx.fillStyle = '#374151';
            this.ctx.font = '12px Arial';
            this.ctx.textAlign = 'center';
            this.ctx.textBaseline = 'middle';
            
            // 调整标签位置
            const labelX = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 25);
            const labelY = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 25);
            
            this.ctx.fillText(tick.label, labelX, labelY);
            this.ctx.restore();
        });
    }
    
    /**
     * 绘制指针
     */
    drawHand() {
        // 计算指针位置（逆时针方向）
        const x = this.centerX + Math.cos(Math.PI * 1.5 + this.currentAngle) * (this.radius - 30);
        const y = this.centerY + Math.sin(Math.PI * 1.5 + this.currentAngle) * (this.radius - 30);
        
        // 绘制指针线
        this.ctx.beginPath();
        this.ctx.moveTo(this.centerX, this.centerY);
        this.ctx.lineTo(x, y);
        this.ctx.strokeStyle = '#ef4444';
        this.ctx.lineWidth = 3;
        this.ctx.stroke();
        
        // 绘制指针圆点
        this.drawCircle(x, y, 5, '#ef4444', 0);
    }
    
    /**
     * 绘制角度标记
     */
    drawDegreeMarkers() {
        // 0度标记（顶部）
        this.ctx.save();
        this.ctx.fillStyle = '#10b981';
        this.ctx.font = 'bold 14px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('0°', this.centerX, this.centerY - this.radius + 40);
        this.ctx.restore();
        
        // 90度标记（右侧）
        this.ctx.save();
        this.ctx.fillStyle = '#3b82f6';
        this.ctx.font = 'bold 14px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('90°', this.centerX + this.radius - 40, this.centerY);
        this.ctx.restore();
        
        // 180度标记（底部）
        this.ctx.save();
        this.ctx.fillStyle = '#f59e0b';
        this.ctx.font = 'bold 14px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('180°', this.centerX, this.centerY + this.radius - 40);
        this.ctx.restore();
        
        // 270度标记（左侧）
        this.ctx.save();
        this.ctx.fillStyle = '#8b5cf6';
        this.ctx.font = 'bold 14px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('270°', this.centerX - this.radius + 40, this.centerY);
        this.ctx.restore();
    }
    
    /**
     * 更新价格显示
     */
    updatePriceDisplay() {
        const priceElement = document.getElementById('currentPrice');
        if (priceElement) {
            priceElement.textContent = this.formatPrice(this.currentPrice);
        }
    }
    
    /**
     * 更新价格
     * @param {number} newPrice - 新价格
     */
    updatePrice(newPrice) {
        this.currentPrice = Math.max(this.minPrice, Math.min(this.maxPrice, newPrice));
        this.updateAngleFromPrice();
        this.updatePriceDisplay();
    }
    
    /**
     * 重置钟表
     */
    reset() {
        // 停止所有动画和计时器
        this.stopAnimation();
        
        // 重置状态
        this.currentAngle = 0;
        this.targetAngle = 0;
        this.auction = null;
        this.currentPrice = 0;
        this.minPrice = 0;
        this.maxPrice = 0;
        this.priceDecrement = 0;
        this.priceTicks = [];
        this.lastUpdateTime = 0;
        
        // 重新绘制钟表
        this.drawClock();
    }
    
    /**
     * 更新剩余时间显示
     * @param {number} remainingSeconds - 剩余秒数
     */
    updateRemainingTime(remainingSeconds) {
        const timeElement = document.getElementById('remainingTime');
        if (timeElement) {
            const minutes = Math.floor(remainingSeconds / 60);
            const seconds = remainingSeconds % 60;
            timeElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        }
    }
}