/**
 * 荷兰钟可视化类
 * 基于二维笛卡尔坐标系实现荷兰钟动画效果
 */
class AuctionClockVisualizer {
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
        
        // 指针动画控制
        this.handAnimationTimer = null;
        
        // 价格刻度
        this.priceTicks = [];
        this.subTicks = []; // 次刻度数组
        
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
        // 确保拍卖对象存在
        if (!auction) {
            console.error('拍卖对象为空，无法设置拍卖数据');
            return;
        }
        
        this.auction = auction;
        this.currentPrice = auction.currentPrice || 0;
        this.minPrice = auction.minPrice || 0;
        this.maxPrice = auction.initialPrice || auction.startPrice || 0;
        this.priceDecrement = auction.priceDecrement || 1; // 默认递减量为1
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
     * 注意：这里根据后端WebSocket发送的价格来更新角度
     */
    updateAngleFromPrice() {
        if (this.maxPrice <= this.minPrice) {
            this.currentAngle = 0;
            this.targetAngle = 0;
            return;
        }
        
        // 计算价格比例
        const priceRange = this.maxPrice - this.minPrice;
        const priceRatio = (this.maxPrice - this.currentPrice) / priceRange;
        
        // 计算角度（逆时针方向，从初始价格到最低价格）
        // 考虑保留的间隔，所以需要乘以0.95
        this.currentAngle = priceRatio * Math.PI * 2 * 0.95;
        this.targetAngle = this.currentAngle;
    }
    
    /**
     * 生成价格刻度
     * 生成主刻度和次刻度
     */
    generatePriceTicks() {
        // 确保数组已初始化
        this.priceTicks = [];
        this.subTicks = []; // 次刻度数组
        
        // 检查价格数据是否有效
        if (this.maxPrice <= this.minPrice || this.priceDecrement <= 0) {
            console.warn('价格数据无效，无法生成刻度');
            return;
        }
        
        // 根据初始价格和递减价格计算主刻度数量
        const totalDecrementSteps = Math.ceil((this.maxPrice - this.minPrice) / this.priceDecrement);
        const mainTickCount = Math.min(totalDecrementSteps, 20); // 限制最多20个主刻度
        
        // 生成主刻度
        for (let i = 0; i <= mainTickCount; i++) {
            // 逆时针方向，从0度开始
            // 保留一个刻度的间隔，所以实际使用范围是 0 到 360-18 度
            const angle = (Math.PI * 2 * 0.95 * i) / mainTickCount;
            
            // 从初始价格开始，逆时针向最低价格递减
            const price = this.maxPrice - ((this.maxPrice - this.minPrice) * (i / mainTickCount));
            
            this.priceTicks.push({
                angle: angle,
                price: price,
                label: this.formatPrice(price),
                isMain: true
            });
            
            // 在每个主刻度之间生成4个次刻度（每个次刻度代表1的价格单位）
            if (i < mainTickCount) {
                const nextPrice = this.maxPrice - ((this.maxPrice - this.minPrice) * ((i + 1) / mainTickCount));
                const priceDiff = nextPrice - price;
                
                // 如果价格差大于等于4，则生成次刻度
                if (Math.abs(priceDiff) >= 4) {
                    for (let j = 1; j <= 4; j++) {
                        const subPrice = price - (j * Math.abs(priceDiff) / 5);
                        const subAngle = angle + (Math.PI * 2 * 0.95 * j) / (mainTickCount * 5);
                        
                        this.subTicks.push({
                            angle: subAngle,
                            price: subPrice,
                            isMain: false
                        });
                    }
                }
            }
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
     * 注意：动画更新完全基于WebSocket信号，不使用独立定时器
     */
    startAnimation() {
        if (this.isAnimating) return;
        
        this.isAnimating = true;
        
        // 不再启动独立的定时器，动画将完全基于WebSocket信号更新
        // 移除startCanvasRefresh()和startHandAnimation()调用
        
        // 绘制初始状态
        this.drawClock();
    }
    
    /**
     * 机械钟表式角度更新
     * 实现1刻度->1刻度的离散跳跃
     */
    updateMechanicalAngle() {
        const now = Date.now();
        const timeDiff = now - this.lastUpdateTime;
        
        // 计算理想的目标角度（基于实际时间）
        const targetAngle = this.calculateTargetAngle();
        
        // 计算当前应该达到的刻度位置
        const currentTick = this.getCurrentTickPosition();
        const targetTick = this.getTargetTickPosition(targetAngle);
        
        // 如果还没有达到下一个刻度位置，不进行跳跃
        if (Math.abs(targetAngle - this.currentAngle) < this.getTickStepAngle()) {
            return;
        }
        
        // 机械钟表式跳跃：每次跳跃一个刻度
        this.performMechanicalJump();
        
        // 更新最后更新时间
        this.lastUpdateTime = now;
    }
    
    /**
     * 计算目标角度
     */
    calculateTargetAngle() {
        if (this.maxPrice <= this.minPrice) {
            return 0;
        }
        
        const priceRange = this.maxPrice - this.minPrice;
        const priceRatio = (this.maxPrice - this.currentPrice) / priceRange;
        return priceRatio * Math.PI * 2 * 0.95;
    }
    
    /**
     * 获取当前刻度位置
     */
    getCurrentTickPosition() {
        const tickStepAngle = this.getTickStepAngle();
        return Math.round(this.currentAngle / tickStepAngle);
    }
    
    /**
     * 获取目标刻度位置
     */
    getTargetTickPosition(targetAngle) {
        const tickStepAngle = this.getTickStepAngle();
        return Math.round(targetAngle / tickStepAngle);
    }
    
    /**
     * 获取刻度步长角度
     * 获取次刻度的步长角度（更精细的刻度）
     */
    getTickStepAngle() {
        // 使用次刻度的数量计算步长，实现更精细的跳跃
        const totalTicks = this.priceTicks.length * 5; // 每个主刻度之间有4个次刻度
        return (Math.PI * 2 * 0.95) / totalTicks;
    }
    
    /**
     * 执行机械钟表式跳跃
     */
    performMechanicalJump() {
        const targetAngle = this.calculateTargetAngle();
        const currentTick = this.getCurrentTickPosition();
        const targetTick = this.getTargetTickPosition(targetAngle);
        
        // 确定跳跃方向（通常是逆时针）
        const direction = targetTick > currentTick ? 1 : -1;
        
        // 跳跃到下一个刻度
        const tickStepAngle = this.getTickStepAngle();
        this.currentAngle = (currentTick + direction) * tickStepAngle;
        
        // 添加机械感：轻微的回弹效果
        // this.addMechanicalBounce(direction);
        
        // 更新价格
        this.updatePriceFromAngle();
    }
    
    /**
     * 添加机械钟表的回弹效果
     */
    // addMechanicalBounce(direction) {
    //     // 记录回弹前的角度
    //     const preBounceAngle = this.currentAngle;
        
    //     // 轻微回弹（模拟机械钟表的齿轮咬合感）
    //     setTimeout(() => {
    //         this.currentAngle = preBounceAngle + (direction * this.getTickStepAngle() * 0.1);
            
    //         setTimeout(() => {
    //             this.currentAngle = preBounceAngle;
    //         }, 50);
    //     }, 30);
    // }
    
    /**
     * 停止动画
     * 注意：动画更新完全基于WebSocket信号，不使用独立定时器
     */
    stopAnimation() {
        this.isAnimating = false;
        
        // 清除动画帧
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
            this.animationFrame = null;
        }
        
        // 清除canvas刷新计时器（回退方法）
        if (this.canvasRefreshInterval) {
            clearInterval(this.canvasRefreshInterval);
            this.canvasRefreshInterval = null;
        }
        
        // 清除指针动画计时器
        if (this.handAnimationTimer) {
            clearInterval(this.handAnimationTimer);
            this.handAnimationTimer = null;
        }
    }
    
    /**
     * 暂停动画
     * 保留当前状态，但停止动画更新
     * 注意：动画更新完全基于WebSocket信号，不使用独立定时器
     */
    pauseAnimation() {
        // 标记动画为暂停状态
        this.isAnimating = false;
        
        // 清除动画帧
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
            this.animationFrame = null;
        }
        
        // 清除指针动画计时器，但保留当前角度和价格
        if (this.handAnimationTimer) {
            clearInterval(this.handAnimationTimer);
            this.handAnimationTimer = null;
        }
        
        // 清除canvas刷新计时器（回退方法）
        if (this.canvasRefreshInterval) {
            clearInterval(this.canvasRefreshInterval);
            this.canvasRefreshInterval = null;
        }
    }
    
    /**
     * 恢复动画
     * 从暂停状态继续播放动画
     * 注意：动画更新完全基于WebSocket信号，不使用独立定时器
     */
    resumeAnimation() {
        if (this.isAnimating || !this.auction) return;
        
        // 恢复动画状态
        this.isAnimating = true;
        
        // 不再启动独立的定时器，动画将完全基于WebSocket信号更新
        // 移除startCanvasRefresh()和startHandAnimation()调用
        
        // 绘制当前状态
        this.drawClock();
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
     * 绘制主刻度和次刻度
     */
    drawPriceTicks() {
        // 确保priceTicks和subTicks数组已初始化
        if (!this.priceTicks) {
            this.priceTicks = [];
        }
        if (!this.subTicks) {
            this.subTicks = [];
        }
        
        // 绘制主刻度
        this.priceTicks.forEach((tick, index) => {
            // 计算主刻度位置（逆时针方向）
            const x1 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 15);
            const y1 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 15);
            const x2 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            const y2 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            
            // 绘制主刻度线（更粗）
            this.ctx.beginPath();
            this.ctx.moveTo(x1, y1);
            this.ctx.lineTo(x2, y2);
            this.ctx.strokeStyle = '#374151';
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
            
            // 绘制价格标签（仅主刻度有标签）
            this.ctx.save();
            this.ctx.fillStyle = '#374151';
            this.ctx.font = '9px Arial';
            this.ctx.textAlign = 'center';
            this.ctx.textBaseline = 'middle';
            
            // 调整标签位置
            const labelX = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 25);
            const labelY = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 25);
            
            this.ctx.fillText(tick.label, labelX, labelY);
            this.ctx.restore();
        });
        
        // 绘制次刻度
        this.subTicks.forEach((tick, index) => {
            // 计算次刻度位置（逆时针方向）
            const x1 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 10);
            const y1 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 10);
            const x2 = this.centerX + Math.cos(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            const y2 = this.centerY + Math.sin(Math.PI * 1.5 + tick.angle) * (this.radius - 5);
            
            // 绘制次刻度线（更细，无标签）
            this.ctx.beginPath();
            this.ctx.moveTo(x1, y1);
            this.ctx.lineTo(x2, y2);
            this.ctx.strokeStyle = '#9ca3af';
            this.ctx.lineWidth = 1;
            this.ctx.stroke();
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
        
        // 绘制当前价格位置的实心圆点
        const priceX = this.centerX + Math.cos(Math.PI * 1.5 + this.currentAngle) * (this.radius - 10);
        const priceY = this.centerY + Math.sin(Math.PI * 1.5 + this.currentAngle) * (this.radius - 10);
        
        this.ctx.beginPath();
        this.ctx.arc(priceX, priceY, 2, 0, 2 * Math.PI, false);
        this.ctx.fillStyle = '#ef4444';
        this.ctx.fill();
    }
    
    /**
     * 绘制角度标记
     */
    drawDegreeMarkers() {
        // 0度标记（顶部）
        this.ctx.save();
        this.ctx.fillStyle = '#10b981';
        this.ctx.font = 'bold 9px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('0°', this.centerX, this.centerY - this.radius + 40);
        this.ctx.restore();
        
        // 90度标记（右侧）
        this.ctx.save();
        this.ctx.fillStyle = '#3b82f6';
        this.ctx.font = 'bold 9px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('90°', this.centerX + this.radius - 40, this.centerY);
        this.ctx.restore();
        
        // 180度标记（底部）
        this.ctx.save();
        this.ctx.fillStyle = '#f59e0b';
        this.ctx.font = 'bold 9px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('180°', this.centerX, this.centerY + this.radius - 40);
        this.ctx.restore();
        
        // 270度标记（左侧）
        this.ctx.save();
        this.ctx.fillStyle = '#8b5cf6';
        this.ctx.font = 'bold 9px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText('270°', this.centerX - this.radius + 40, this.centerY);
        this.ctx.restore();
    }
    
    /**
     * 根据角度更新价格
     * 根据当前角度计算对应的价格
     */
    updatePriceFromAngle() {
        if (this.maxPrice <= this.minPrice) {
            this.currentPrice = this.maxPrice;
            return;
        }
        
        // 计算角度比例（考虑保留的间隔0.95）
        const maxAngle = Math.PI * 2 * 0.95;
        const angleRatio = this.currentAngle / maxAngle;
        
        // 根据角度比例计算价格
        const priceRange = this.maxPrice - this.minPrice;
        this.currentPrice = this.maxPrice - (priceRange * angleRatio);
        
        // 确保价格不低于最低价格
        if (this.currentPrice < this.minPrice) {
            this.currentPrice = this.minPrice;
        }
        
        // 更新UI中的价格显示
        this.updatePriceDisplay();
    }
    
    /**
     * 更新价格显示
     * 更新UI中的价格显示元素
     */
    updatePriceDisplay() {
        // 查找价格显示元素
        const priceElement = document.getElementById('current-price');
        if (priceElement) {
            priceElement.textContent = this.formatPrice(this.currentPrice);
        }
        
        // 触发自定义事件，通知价格更新
        const priceUpdateEvent = new CustomEvent('priceUpdate', {
            detail: {
                price: this.currentPrice,
                auctionId: this.auction ? this.auction.id : null
            }
        });
        document.dispatchEvent(priceUpdateEvent);
    }
    
    /**
     * 更新价格
     * 注意：这里直接使用后端WebSocket发送的价格，而不是本地计算
     * 动画更新完全基于WebSocket信号，不使用独立定时器
     * @param {number} newPrice - 新价格
     */
    updatePrice(newPrice) {
        // 直接使用后端WebSocket发送的价格
        this.currentPrice = newPrice;
        
        // 根据新价格更新角度
        this.updateAngleFromPrice();
        
        // 立即更新动画显示，不依赖独立定时器
        this.drawClock();
    }
    
    /**
     * 更新剩余时间
     * @param {number} timeRemaining - 剩余时间（秒）
     */
    updateRemainingTime(timeRemaining) {
        // 验证剩余时间是否有效
        if (typeof timeRemaining !== 'number' || isNaN(timeRemaining) || timeRemaining < 0) {
            console.warn(`无效的剩余时间数据: ${timeRemaining}`);
            return;
        }
        
        // 更新拍卖对象的剩余时间
        if (this.auction) {
            this.auction.timeRemaining = timeRemaining;
        }
        
        // 可以在这里添加剩余时间显示的逻辑
        // 例如在钟表上显示剩余时间
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
        this.subTicks = []; // 重置次刻度数组
        this.lastUpdateTime = 0;
        
        // 重新绘制钟表
        this.drawClock();
    }
}