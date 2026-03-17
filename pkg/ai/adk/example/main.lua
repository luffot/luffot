-- 示例 Lua 技能
-- 该脚本会被 ADK 引擎加载并执行

-- execute 函数是入口点，必须实现
-- @param input table 输入参数表
-- @return table 返回结果表
function execute(input)
    -- 获取输入参数
    local message = input.message or "Hello"
    local count = input.count or 1
    
    -- 处理逻辑
    local result = ""
    for i = 1, count do
        result = result .. message .. " "
    end
    
    -- 返回结果
    return {
        result = result:gsub("%s+$", ""),  -- 去除末尾空格
        status = "success",
        processed_at = os.date("%Y-%m-%d %H:%M:%S")
    }
end

-- 可选：初始化函数，在技能加载时调用
function init()
    print("Example skill initialized")
    return true
end

-- 可选：健康检查函数
function health_check()
    return {
        status = "healthy",
        timestamp = os.time()
    }
end
