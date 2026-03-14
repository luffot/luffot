-- ============================================================
-- 鸡坤（蔡徐坤风格）- Lua 皮肤
-- 使用 Go Ebiten 渲染桥接 API (graphics.*)
-- ============================================================

local Jikun = {
    name = "鸡坤（篮球版）",
    version = "1.0.0",
    
    -- 颜色配置
    colors = {
        -- 皮肤
        skin = {255/255, 220/255, 190/255, 1},
        skinShadow = {240/255, 200/255, 170/255, 1},
        
        -- 头发（中分黑色）
        hair = {30/255, 30/255, 35/255, 1},
        hairHighlight = {50/255, 50/255, 55/255, 1},
        
        -- 眼睛
        eyeWhite = {1, 1, 1, 1},
        eyePupil = {80/255, 60/255, 40/255, 1},
        eyeShine = {1, 1, 1, 0.8},
        
        -- 嘴巴
        mouth = {200/255, 100/255, 100/255, 1},
        
        -- 背带裤（深色）
        pants = {40/255, 40/255, 60/255, 1},
        pantsLight = {60/255, 60/255, 85/255, 1},
        strap = {200/255, 180/255, 50/255, 1}, -- 金色背带
        
        -- 衬衫（白色）
        shirt = {245/255, 245/255, 250/255, 1},
        shirtShadow = {220/255, 220/255, 230/255, 1},
        
        -- 篮球（橙色）
        basketball = {230/255, 100/255, 30/255, 1},
        basketballDark = {180/255, 70/255, 20/255, 1},
        basketballLine = {40/255, 40/255, 40/255, 1},
        
        -- 鞋子
        shoes = {50/255, 50/255, 50/255, 1},
        
        -- 舞台效果
        stageLight = {255/255, 200/255, 100/255, 0.3},
        sparkle = {1, 1, 1, 0.8},
    },
    
    -- 状态
    state = "idle", -- idle, typing, thinking, talking, alert
    tick = 0,
    
    -- 位置
    x = 80,
    y = 80,
    
    -- 动画参数
    anim = {
        bodyBob = 0,
        headTilt = 0,
        armAngle = 0,
        legAngle = 0,
        ballBounce = 0,
        ballRotation = 0,
        blinkTimer = 0,
        isBlinking = false,
        dancePhase = 0,
    },
    
    -- 篮球粒子效果
    particles = {},
}

-- 常量
local SPRITE_WIDTH = 160
local SPRITE_HEIGHT = 160

-- ============================================================
-- 工具函数
-- ============================================================

-- 绘制填充圆形
function drawFilledCircle(cx, cy, r, color)
    graphics.setColor(color[1], color[2], color[3], color[4])
    graphics.circle("fill", cx, cy, r)
end

-- 绘制线段
function drawLine(x1, y1, x2, y2, color, width)
    width = width or 1
    graphics.setColor(color[1], color[2], color[3], color[4])
    graphics.line(x1, y1, x2, y2, width)
end

-- 绘制填充三角形
function drawFilledTriangle(x1, y1, x2, y2, x3, y3, color)
    local minX = min(x1, min(x2, x3))
    local minY = min(y1, min(y2, y3))
    local maxX = max(x1, max(x2, x3))
    local maxY = max(y1, max(y2, y3))
    graphics.setColor(color[1], color[2], color[3], color[4])
    graphics.rectangle("fill", minX, minY, maxX - minX, maxY - minY)
end

-- 绘制填充矩形
function drawFilledRect(x, y, w, h, color)
    graphics.setColor(color[1], color[2], color[3], color[4])
    graphics.rectangle("fill", x, y, w, h)
end

-- ============================================================
-- 绘制组件
-- ============================================================

-- 绘制中分发型
function Jikun:drawHair(cx, cy, tilt)
    local c = self.colors
    local baseY = cy - 35
    
    -- 头顶头发（中分的基础）
    drawFilledCircle(cx, baseY, 22, c.hair)
    
    -- 左侧中分刘海
    for i = 0, 3 do
        local offsetX = -15 - i * 3
        local offsetY = -5 + i * 2
        local hairLen = 18 - i * 2
        drawLine(cx + offsetX, baseY + offsetY, cx + offsetX - 5, baseY + offsetY + hairLen, c.hair, 4)
        drawFilledCircle(cx + offsetX - 5, baseY + offsetY + hairLen, 3, c.hair)
    end
    
    -- 右侧中分刘海
    for i = 0, 3 do
        local offsetX = 15 + i * 3
        local offsetY = -5 + i * 2
        local hairLen = 18 - i * 2
        drawLine(cx + offsetX, baseY + offsetY, cx + offsetX + 5, baseY + offsetY + hairLen, c.hair, 4)
        drawFilledCircle(cx + offsetX + 5, baseY + offsetY + hairLen, 3, c.hair)
    end
    
    -- 头顶高光
    drawFilledCircle(cx - 8, baseY - 8, 4, c.hairHighlight)
    drawFilledCircle(cx + 6, baseY - 10, 3, c.hairHighlight)
end

-- 绘制脸部
function Jikun:drawFace(cx, cy)
    local c = self.colors
    
    -- 脸部轮廓
    drawFilledCircle(cx, cy - 15, 18, c.skin)
    
    -- 脸部阴影（下巴）
    drawFilledCircle(cx, cy - 5, 12, c.skinShadow)
end

-- 绘制眼睛（带眨眼动画）
function Jikun:drawEyes(cx, cy)
    local c = self.colors
    local eyeY = cy - 18
    
    if self.anim.isBlinking then
        -- 闭眼状态（两条线）
        drawLine(cx - 10, eyeY, cx - 4, eyeY, c.hair, 2)
        drawLine(cx + 4, eyeY, cx + 10, eyeY, c.hair, 2)
    else
        -- 睁眼状态
        -- 左眼
        drawFilledCircle(cx - 7, eyeY, 5, c.eyeWhite)
        drawFilledCircle(cx - 6, eyeY, 2.5, c.eyePupil)
        drawFilledCircle(cx - 5, eyeY - 1, 1, c.eyeShine)
        
        -- 右眼
        drawFilledCircle(cx + 7, eyeY, 5, c.eyeWhite)
        drawFilledCircle(cx + 6, eyeY, 2.5, c.eyePupil)
        drawFilledCircle(cx + 7, eyeY - 1, 1, c.eyeShine)
    end
end

-- 绘制嘴巴
function Jikun:drawMouth(cx, cy)
    local c = self.colors
    local mouthY = cy - 8
    
    -- 微笑嘴巴
    drawFilledCircle(cx, mouthY, 3, c.mouth)
    drawLine(cx - 3, mouthY - 1, cx + 3, mouthY - 1, c.mouth, 1)
end

-- 绘制头部（包含头发、脸、五官）
function Jikun:drawHead(cx, cy, tilt)
    self:drawHair(cx, cy, tilt)
    self:drawFace(cx, cy)
    self:drawEyes(cx, cy)
    self:drawMouth(cx, cy)
end

-- 绘制身体（背带裤+衬衫）
function Jikun:drawBody(cx, cy)
    local c = self.colors
    
    -- 衬衫（上身）
    drawFilledRect(cx - 15, cy, 30, 25, c.shirt)
    
    -- 衬衫阴影
    drawFilledRect(cx - 15, cy + 20, 30, 5, c.shirtShadow)
    
    -- 背带裤（下身）
    drawFilledRect(cx - 16, cy + 22, 32, 28, c.pants)
    
    -- 背带裤高光
    drawFilledRect(cx - 12, cy + 25, 8, 20, c.pantsLight)
    drawFilledRect(cx + 4, cy + 25, 8, 20, c.pantsLight)
    
    -- 背带（金色）
    drawLine(cx - 10, cy + 2, cx - 10, cy + 22, c.strap, 3)
    drawLine(cx + 10, cy + 2, cx + 10, cy + 22, c.strap, 3)
    
    -- 背带扣子
    drawFilledCircle(cx - 10, cy + 22, 2, c.strap)
    drawFilledCircle(cx + 10, cy + 22, 2, c.strap)
end

-- 绘制手臂
function Jikun:drawArm(cx, cy, angle, isLeft)
    local c = self.colors
    local side = isLeft and -1 or 1
    local rad = angle * math.pi / 180
    
    -- 肩膀位置
    local shoulderX = cx + side * 18
    local shoulderY = cy + 5
    
    -- 手臂长度和角度
    local armLen = 20
    local elbowX = shoulderX + side * armLen * 0.5 * cos(rad)
    local elbowY = shoulderY + armLen * 0.5 * sin(rad)
    local handX = shoulderX + side * armLen * cos(rad)
    local handY = shoulderY + armLen * sin(rad)
    
    -- 上臂（衬衫袖子）
    drawLine(shoulderX, shoulderY, elbowX, elbowY, c.shirt, 6)
    
    -- 前臂（皮肤）
    drawLine(elbowX, elbowY, handX, handY, c.skin, 5)
    
    -- 手
    drawFilledCircle(handX, handY, 4, c.skin)
    
    return handX, handY
end

-- 绘制腿部
function Jikun:drawLegs(cx, cy)
    local c = self.colors
    
    -- 左腿
    drawLine(cx - 8, cy + 48, cx - 10, cy + 70, c.pants, 8)
    -- 右腿
    drawLine(cx + 8, cy + 48, cx + 10, cy + 70, c.pants, 8)
    
    -- 鞋子
    drawFilledRect(cx - 16, cy + 68, 12, 6, c.shoes)
    drawFilledRect(cx + 4, cy + 68, 12, 6, c.shoes)
end

-- 绘制篮球
function Jikun:drawBasketball(cx, cy, rotation, scale)
    local c = self.colors
    scale = scale or 1
    local r = 12 * scale
    
    -- 篮球主体
    drawFilledCircle(cx, cy, r, c.basketball)
    
    -- 篮球纹理线
    graphics.setColor(c.basketballLine[1], c.basketballLine[2], c.basketballLine[3], c.basketballLine[4])
    
    -- 中间横线
    graphics.rectangle("fill", cx - r, cy - 1, r * 2, 2)
    
    -- 中间竖线（带旋转效果）
    local rad = rotation * math.pi / 180
    local lineW = r * 0.3
    local lineH = r * 2
    
    -- 简化的旋转效果
    local offsetX = sin(rad) * r * 0.5
    graphics.rectangle("fill", cx + offsetX - 1, cy - r, 2, r * 2)
    
    -- 篮球高光
    drawFilledCircle(cx - r * 0.4, cy - r * 0.4, r * 0.25, {1, 1, 1, 0.4})
end

-- 绘制舞台光效
function Jikun:drawStageEffect(cx, cy)
    local c = self.colors
    
    -- 底部光晕
    drawFilledCircle(cx, cy + 75, 40, c.stageLight)
    drawFilledCircle(cx, cy + 75, 30, {c.stageLight[1], c.stageLight[2], c.stageLight[3], 0.2})
end

-- ============================================================
-- 状态绘制函数
-- ============================================================

-- Idle 状态：运球/玩篮球
function Jikun:drawIdle(cx, cy)
    local a = self.anim
    
    self:drawStageEffect(cx, cy)
    
    -- 身体上下浮动
    local bodyY = cy + a.bodyBob
    
    -- 腿部
    self:drawLegs(cx, bodyY)
    
    -- 身体
    self:drawBody(cx, bodyY)
    
    -- 手臂（运球动作）
    local leftHandX, leftHandY = self:drawArm(cx, bodyY, -30 + a.armAngle, true)
    local rightHandX, rightHandY = self:drawArm(cx, bodyY, 30 - a.armAngle, false)
    
    -- 篮球（在两手之间弹跳）
    local ballY = bodyY + 45 + a.ballBounce
    local ballX = cx + sin(a.ballRotation * 0.1) * 5
    self:drawBasketball(ballX, ballY, a.ballRotation, 1)
    
    -- 头部
    self:drawHead(cx, bodyY - 5, a.headTilt)
end

-- Typing 状态：敲键盘（双手放在前面）
function Jikun:drawTyping(cx, cy)
    local a = self.anim
    
    self:drawStageEffect(cx, cy)
    
    local bodyY = cy + a.bodyBob
    
    -- 腿部
    self:drawLegs(cx, bodyY)
    
    -- 身体
    self:drawBody(cx, bodyY)
    
    -- 手臂（打字动作 - 双手在身前快速移动）
    local typeOffset = sin(self.tick * 0.5) * 3
    self:drawArm(cx, bodyY, 60 + typeOffset, true)
    self:drawArm(cx, bodyY, 60 - typeOffset, false)
    
    -- 键盘（简化的矩形）
    drawFilledRect(cx - 25, bodyY + 35, 50, 8, {100/255, 100/255, 100/255, 1})
    
    -- 头部（专注表情）
    self:drawHead(cx, bodyY - 5, 5)
end

-- Thinking 状态：托腮思考
function Jikun:drawThinking(cx, cy)
    local a = self.anim
    local c = self.colors
    
    self:drawStageEffect(cx, cy)
    
    local bodyY = cy + a.bodyBob
    
    -- 腿部
    self:drawLegs(cx, bodyY)
    
    -- 身体
    self:drawBody(cx, bodyY)
    
    -- 思考气泡
    local bubbleY = bodyY - 70 - abs(sin(self.tick * 0.08)) * 5
    drawFilledCircle(cx + 20, bubbleY, 8, {1, 1, 1, 0.9})
    drawFilledCircle(cx + 35, bubbleY - 12, 12, {1, 1, 1, 0.9})
    drawFilledCircle(cx + 50, bubbleY - 8, 6, {1, 1, 1, 0.9})
    
    -- 问号
    drawFilledCircle(cx + 35, bubbleY - 15, 3, c.hair)
    drawFilledCircle(cx + 35, bubbleY - 8, 1.5, c.hair)
    
    -- 右臂（托腮）
    self:drawArm(cx, bodyY, 110, false)
    
    -- 左臂（自然下垂）
    self:drawArm(cx, bodyY, -20, true)
    
    -- 头部（歪头思考）
    self:drawHead(cx, bodyY - 5, 15)
end

-- Talking 状态：说话（拿着麦克风/手势）
function Jikun:drawTalking(cx, cy)
    local a = self.anim
    local c = self.colors
    
    self:drawStageEffect(cx, cy)
    
    local bodyY = cy + a.bodyBob
    
    -- 腿部（可能有舞蹈动作）
    self:drawLegs(cx, bodyY)
    
    -- 身体
    self:drawBody(cx, bodyY)
    
    -- 左臂（拿着麦克风）
    local micX, micY = self:drawArm(cx, bodyY, -45 + a.armAngle * 0.5, true)
    -- 麦克风
    drawFilledRect(micX - 3, micY - 8, 6, 12, {80/255, 80/255, 80/255, 1})
    drawFilledCircle(micX, micY - 10, 4, {150/255, 150/255, 150/255, 1})
    
    -- 右臂（手势）
    self:drawArm(cx, bodyY, 80 + a.armAngle, false)
    
    -- 头部
    self:drawHead(cx, bodyY - 5, -a.headTilt)
    
    -- 音符效果
    if self.tick % 30 < 15 then
        drawFilledCircle(cx + 25, bodyY - 50, 3, c.sparkle)
    end
end

-- Alert 状态：惊讶（篮球掉了/震惊）
function Jikun:drawAlert(cx, cy)
    local c = self.colors
    
    self:drawStageEffect(cx, cy)
    
    local bodyY = cy - 5 -- 身体后仰
    
    -- 惊讶效果线
    for i = 1, 3 do
        local offset = (self.tick * 3 + i * 25) % 35
        drawLine(cx - 45 - offset, bodyY - 50, cx - 40 - offset, bodyY - 45, {1, 1, 1, 0.7}, 2)
        drawLine(cx + 45 + offset, bodyY - 50, cx + 40 + offset, bodyY - 45, {1, 1, 1, 0.7}, 2)
    end
    
    -- 腿部
    self:drawLegs(cx, bodyY)
    
    -- 身体
    self:drawBody(cx, bodyY)
    
    -- 双臂举起
    self:drawArm(cx, bodyY, -120, true)
    self:drawArm(cx, bodyY, 120, false)
    
    -- 篮球（掉落效果）
    local dropY = bodyY + 60 + (self.tick % 20)
    self:drawBasketball(cx + 30, dropY, self.tick * 5, 1)
    
    -- 头部（后仰惊讶）
    self:drawHead(cx, bodyY - 8, -10)
    
    -- 惊讶的大眼睛
    drawFilledCircle(cx - 8, bodyY - 23, 8, c.eyeWhite)
    drawFilledCircle(cx - 7, bodyY - 23, 4, c.eyePupil)
    drawFilledCircle(cx + 8, bodyY - 23, 8, c.eyeWhite)
    drawFilledCircle(cx + 7, bodyY - 23, 4, c.eyePupil)
end

-- ============================================================
-- 主函数
-- ============================================================

-- 更新动画参数
function Jikun:update(dt)
    self.tick = self.tick + 1
    
    local a = self.anim
    local t = self.tick
    
    if self.state == "idle" then
        -- 运球动画
        a.bodyBob = sin(t * 0.15) * 3
        a.armAngle = sin(t * 0.3) * 15
        a.ballBounce = abs(sin(t * 0.3)) * 15
        a.ballRotation = t * 3
        a.headTilt = sin(t * 0.1) * 5
        
    elseif self.state == "typing" then
        -- 打字动画
        a.bodyBob = sin(t * 0.4) * 1.5
        a.armAngle = sin(t * 0.8) * 10
        
    elseif self.state == "thinking" then
        -- 思考动画
        a.bodyBob = sin(t * 0.08) * 2
        a.headTilt = 15 + sin(t * 0.05) * 3
        
    elseif self.state == "talking" then
        -- 说话/唱歌动画
        a.bodyBob = sin(t * 0.2) * 2
        a.armAngle = sin(t * 0.25) * 20
        a.headTilt = sin(t * 0.15) * 8
        
    elseif self.state == "alert" then
        -- 惊讶保持
        a.bodyBob = -3
    end
    
    -- 眨眼动画
    a.blinkTimer = a.blinkTimer + 1
    if a.blinkTimer > 150 then
        a.isBlinking = true
        if a.blinkTimer > 158 then
            a.isBlinking = false
            a.blinkTimer = 0
        end
    end
end

-- 绘制函数
function Jikun:draw()
    local cx = self.x
    local cy = self.y
    
    -- 根据状态调用对应的绘制函数
    if self.state == "idle" then
        self:drawIdle(cx, cy)
    elseif self.state == "typing" then
        self:drawTyping(cx, cy)
    elseif self.state == "thinking" then
        self:drawThinking(cx, cy)
    elseif self.state == "talking" then
        self:drawTalking(cx, cy)
    elseif self.state == "alert" then
        self:drawAlert(cx, cy)
    else
        self:drawIdle(cx, cy)
    end
end

-- 设置状态（由 Go 程序调用）
function Jikun:setState(newState)
    if newState == "idle" or newState == "typing" or 
       newState == "thinking" or newState == "talking" or newState == "alert" then
        self.state = newState
        self.tick = 0
        -- 重置动画参数
        self.anim.bodyBob = 0
        self.anim.headTilt = 0
        self.anim.armAngle = 0
        self.anim.ballBounce = 0
        self.anim.ballRotation = 0
    end
end

-- 设置位置（由 Go 程序调用）
function Jikun:setPosition(x, y)
    self.x = x
    self.y = y
end

-- 获取当前状态
function Jikun:getState()
    return self.state
end

-- 获取皮肤信息
function Jikun:getInfo()
    return {
        name = self.name,
        version = self.version,
        state = self.state,
        position = {x = self.x, y = self.y}
    }
end

-- 处理事件（由 Go 程序通过 stdin/IPC 发送）
function Jikun:handleEvent(eventType, data)
    if eventType == "state_change" then
        self:setState(data.state)
    elseif eventType == "position_change" then
        self:setPosition(data.x, data.y)
    elseif eventType == "click" then
        -- 点击交互 - 切换到 alert 状态
        self:setState("alert")
    end
end

-- 返回皮肤对象
return Jikun
