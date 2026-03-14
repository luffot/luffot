// ── 全局状态 ──
let messagesRefreshTimer = null;

// ── Toast 通知 ──
function showToast(msg, type) {
    const toast = document.getElementById("toast");
    toast.textContent = msg;
    toast.className = "toast " + (type || "");
    toast.classList.add("show");
    setTimeout(() => toast.classList.remove("show"), 3000);
}

// ── 工具函数 ──
function escapeHtml(str) {
    if (!str) return "";
    return str
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
}

// ── 页面导航 ──
const barrageSubPages = ["barrage-filter", "barrage-highlight"];
const envSubPages = ["camera-settings", "camera-log"];

function toggleBarrageMenu() {
    const parent = document.getElementById("nav-barrage-parent");
    const submenu = document.getElementById("nav-barrage-submenu");
    const isOpen = submenu.classList.contains("open");
    if (isOpen) {
        submenu.classList.remove("open");
        parent.classList.remove("open");
    } else {
        submenu.classList.add("open");
        parent.classList.add("open");
    }
}

function toggleEnvMenu() {
    const parent = document.getElementById("nav-env-parent");
    const submenu = document.getElementById("nav-env-submenu");
    const isOpen = submenu.classList.contains("open");
    if (isOpen) {
        submenu.classList.remove("open");
        parent.classList.remove("open");
    } else {
        submenu.classList.add("open");
        parent.classList.add("open");
    }
}

function navigateTo(pageId) {
    document.querySelectorAll(".page").forEach(page => page.classList.remove("active"));
    document.querySelectorAll(".nav-item").forEach(item => item.classList.remove("active"));
    document.querySelectorAll(".nav-sub-item").forEach(item => item.classList.remove("active"));

    const targetPage = document.getElementById("page-" + pageId);
    if (targetPage) targetPage.classList.add("active");

    const targetNav = document.querySelector("[data-page=" + JSON.stringify(pageId) + "]");
    if (targetNav) targetNav.classList.add("active");

    // 弹幕子页：展开子菜单并高亮父项
    const barrageParent = document.getElementById("nav-barrage-parent");
    const barrageSubmenu = document.getElementById("nav-barrage-submenu");
    if (barrageSubPages.includes(pageId)) {
        barrageSubmenu.classList.add("open");
        barrageParent.classList.add("open", "has-active");
    } else {
        barrageParent.classList.remove("has-active");
    }

    // 环境监测子页：展开子菜单并高亮父项
    const envParent = document.getElementById("nav-env-parent");
    const envSubmenu = document.getElementById("nav-env-submenu");
    if (envSubPages.includes(pageId)) {
        envSubmenu.classList.add("open");
        envParent.classList.add("open", "has-active");
    } else {
        envParent.classList.remove("has-active");
    }

    // 离开消息总览时停止定时刷新
    if (pageId !== "messages" && messagesRefreshTimer) {
        clearInterval(messagesRefreshTimer);
        messagesRefreshTimer = null;
    }

    // 按需懒加载
    if (pageId === "prompt") loadPrompts();
    if (pageId === "tasks") loadTasks();
    if (pageId === "messages") loadMessagesPage();
    if (pageId === "guard-log") loadGuardLog();
    if (pageId === "camera-settings") loadCameraSettings();
    if (pageId === "camera-log") loadCameraLog();
    if (pageId === "barrage-filter") loadBarrageFilterConfig();
    if (pageId === "barrage-highlight") loadBarrageHighlightConfig();
    if (pageId === "process-monitor") loadProcessMonitorConfig();
    if (pageId === "skin") loadSkins();
}

// ESC 键关闭灯箱
document.addEventListener("keydown", function(e) {
    if (e.key === "Escape") closeLightbox();
});

// ── 初始化 ──
loadSettings();
loadAlertConfig();
