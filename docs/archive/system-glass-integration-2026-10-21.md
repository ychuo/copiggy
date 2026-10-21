> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../development.md) for supported behavior and development commands.

# 系统玻璃与 NSPopover 接入

界面仍由 MyGo `Content: ui.View(...)` 原生渲染，不使用 WebView、HTML 或 JavaScript。此记录针对系统材质与独立预览浮层的接入，不覆盖或改写此前的研究及验收证据。

## 实现

- 主托盘：macOS 26 使用 `NSGlassEffectView`，MyGo 原生绘制层作为其内容；旧系统回退到 `NSVisualEffectView`。内容根节点透明，系统负责材质。
- 预览：MyGo `NewFlyout` 配置 `Popover: true`、`Focusable: true`，内容仍是原生 `ui.View`。系统负责材质、连续箭头、阴影、间距和靠近屏幕边缘时的翻转；锚点使用选中卡片的可见区域。
- 生命周期：点击外部触发关闭并销毁浮层；程序性 `Hide()` 保留实例。关闭会取消未确认草稿，主动收起后恢复卡片操作；整个面板收起时一并隐藏预览。
- 焦点与守卫：使用真实窗口焦点同步面板状态；明确发往外部 PID 的事件不再被滞后的面板状态豁免。目标 PID 未知时保留非激活面板输入兼容，该路径仍需物理键盘验证。

## 本轮验证

环境为 macOS 26.5。以下状态截至记录时，不将旧版结果推定为本版通过。

| 项目 | 状态 |
| --- | --- |
| 系统玻璃实际启用 | 已确认；启动日志为 `native tray material: liquid-glass` |
| 系统预览容器 | 桌面辅助功能树可见 `NSPopover` 节点，界面显示系统箭头 |
| JSON 草稿与返回 | 格式化草稿显示正常；草稿生成前后剪贴板 SHA-256 一致（`413d5d62…55d742`）；回车确认后在文本编辑用 `⌘V` 得到格式化结果，原记录仍保留；`Esc` 收起并恢复同一卡片焦点 |
| 守卫焦点回归 | 内存 CGEvent 测试通过：外部目标不豁免、待确认复制不误取消、配对 key-up 拦截、PID 0 面板兼容 |
| 全套代码检查 | `go test ./...` 与 `go vet ./...` 通过，最终 bundle 构建和签名成功 |
| 浅色 / 深色 | 实机切换通过，卡片文字、来源图标和位图保留，Popover 同步外观；见下方截图 |
| 预览中的 `⌘F` | 收起 Popover，展开父窗口搜索；不需再次点击，可直接输入；Esc 清空并收起搜索，焦点回到原卡片 |
| 图片预览 | Preview 中复制受控 960×640 PNG → 方卡 → 系统弹层 → 放大 → Esc 返回；预览内左右键可切换到 JSON，箭头随卡片更新 |
| 本版物理键盘同应用 / 跨应用粘贴 | 未重复人工验收 |
| 旧系统材质回退 | 已实现，未实机验证 |

启动材料证据见运行日志（历史产物已从项目移除：`../experiments/mygo-clipboard/build/system-glass-live.log`）。单个主窗口截图可能裁掉其上方的独立 Popover，应结合桌面画面与辅助功能节点判断。

实机截图：浅色托盘（历史产物已从项目移除：`../experiments/mygo-clipboard/build/system-glass-qa/light-tray.png`）、深色托盘（历史产物已从项目移除：`../experiments/mygo-clipboard/build/system-glass-qa/dark-tray.png`）。窗口级截图未完整保留桌面背景透射，也不代表整个桌面的材质表现。测试时短暂暂停了新记录以固定样本；结束前已恢复记录，并将浅色图片 Popover 留在屏幕上。

桌面工具把复制键直接投递给测试应用时，不保证该应用成为系统前台，因此本轮截图的来源图标不可作为来源归属验收。受控 JSON 和 PNG 的内容流程、原生材质接入与焦点行为均已单独确认。

## 既有证据与边界

### 提示、预览与卡片修订（2026-10-21）

- 原生绘制层背景透明，MyGo 默认 Tooltip 的文字回退到该背景色，导致文字透明；现在为深浅主题分别指定不透明提示背景和文字色，并从对应基础主题构造所有控件配色。
- `NSPopover` 的材质外观由其本体拥有。显示时短暂监听公开 will-show 通知，以 content view 所在窗口严格匹配本次浮层，设置并保留其真实 owner；关闭/替换时释放引用。切换主题同步该 owner，未遍历私有视图。
- 主面板去掉眼睛入口；预览工具栏移到上方，底部保留信息和保护范围。标准 Enter/Esc 行为继续保留。
- 鼠标点击工具栏保留焦点；主面板非搜索输入状态允许左右浏览。图片只在外层卡片裁剪一次，去掉有色底边，信息区域有局部暗衬；选中描边使用同一完整矩形与圆角，移除多余内缩。
- 精确重复的完整保存表示且保护范围一致时，刷新旧记录时间并移到最前，保留 ID、来源与增强关联。不同保护范围不合并。

已通过根包及内容模型的纯内存/离屏回归、`go vet ./...`。没有运行系统剪贴板/桌面场景、改权限或生成截图，实际玻璃与主题显示由用户手动确认。

### 主托盘圆角跟进（2026-10-21）

用户截图确认：仅移除窗口的 `titled` 样式未解决四角外溢，不能将标题框认定为完整根因。本轮保留窗口尺寸和玻璃布局，仅在外层内容视图设置与玻璃相同的连续圆角及 `masksToBounds`，并在形状变化提交后重算窗口阴影；卸载玻璃时恢复原层属性。尚待用户确认四角边界是否消失、系统高光是否完整，不以构建通过代替视觉验收。

后续按用户要求只构建、重启应用，由用户手动验证；不再运行桌面交互、复制样本或截图验证。若仍异常，下一步区分系统玻璃外扩被矩形窗口截断与玻璃/裁剪曲线不匹配，不同时叠加多种调整。

系统玻璃改动前一版原生构建已由用户确认：在文本编辑按 `⌃⌘C` 后，同应用粘贴成功，系统设置搜索框中的 `⌘V` 被阻止。原记录保留在前版原生验收结果（历史产物已从项目移除：`../experiments/mygo-clipboard/build/native-ui-verification.json`）；JSON 和 PNG 的真实剪贴板流程见[原生 UI 接入验证](native-ui-integration-2026-10-21.md)。这些不代替本次改动后的人工回归。

应用仍打包到 `experiments/mygo-clipboard/build/Copiggy Spike.app`，bundle ID 为 `dev.copiggy.spike`。本地 ad-hoc 重签后可能需要刷新辅助功能授权，路径与 ID 不变也不代表新签名自动获授权。

守卫覆盖标准粘贴快捷键，不覆盖菜单粘贴或后台读取。历史仍为会话内存数据；AI、持久化和跨设备同步尚未接入。
