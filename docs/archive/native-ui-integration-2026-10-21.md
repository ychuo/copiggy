> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../development.md) for supported behavior and development commands.

# MyGo Native UI 接入验证

界面使用 `Content: ui.View(...)`，由 MyGo 原生控件和绘制 API 构成。没有 WebView、HTML 页面或 JavaScript 桥接。Copiggy 保留已经确认的玻璃托盘、方卡和一体尖角浮层。

## 已完成

- 默认从启动后的新复制开始记录，不载入示例。文本、JSON、链接、PNG 使用不同卡片；来源应用从原生前台应用 API 识别并读取图标。
- 搜索按需展开；方向键浏览；空格或 `⌘Y` 打开预览；回车取用。浏览、搜索和预览不写剪贴板。
- JSON 格式化先生成本地草稿，可切换原文或放弃。确认后新增结果、保留原文及应用限制。没有 AI 请求或内容上传。
- 图片直接来自系统剪贴板，可预览、放大及重新取用，保留原始 PNG 数据。
- `⌃⌘C` 接入原生保护复制回调，保护规则绑定剪贴板版本。状态开关读取实际事件监听器状态；本轮新构建仍需物理键盘验收。
- 图片和文本取用均带独立的随机写入标记，防止把其他应用的新复制误认为本应用的回写。
- 历史最多 100 条，原始表示累计最多 64 MiB；已解码图片最多缓存 8 张。全部为当前会话内存数据。

## 本轮实际操作

桌面工具不能稳定唤起非激活面板，因此提供 `--inspect`：使用相同的 Native UI 与剪贴板逻辑，在可访问的普通窗口中检查。运行方式为在 `experiments/mygo-clipboard` 执行 `sh run.sh build`，再运行 `build/copiggy-spike --inspect`。默认启动仍是无标题栏、失焦收起的面板。

1. 在文本编辑中创建合成 JSON 并复制，原生历史出现 JSON 卡片。
2. 按空格打开预览，点击格式化。外部 `pbpaste` 的哈希与生成草稿前相同，确认草稿未写入剪贴板。
3. 按回车确认，历史新增格式化结果并保留原文；外部读取确认 JSON 内容正确且含换行。
4. 文本编辑中 `⌘V` 收到格式化结果。
5. 在预览应用复制一张 960 × 640 的合成 PNG，历史出现图片卡，浮层显示原图及尺寸，放大后方向键未切换背后记录。
6. 复制普通文本覆盖系统剪贴板，再回历史取用旧图片。预览应用执行“从剪贴板新建”后得到同一合成图片，保存后使用 PIL 比较，尺寸和逐像素内容一致（960 × 640）。

图片证据：复制源图（历史产物已从项目移除：`../experiments/mygo-clipboard/build/native-ui-qa/clipboard-test.png`）、取用后恢复图（历史产物已从项目移除：`../experiments/mygo-clipboard/build/native-ui-qa/clipboard-restored.png`）。

## 验证范围

- 最终显示修复后，模型、图片捕获、保护复制、版本规则、历史预算与 Native UI 键盘/布局自动化测试全部通过，`go vet ./...` 通过。
- 实机确认深浅切换、预览展开/返回后 JSON 和图片不再消失。当前 MyGo 版本需要在外观/预览状态改变时重建卡片内部展示树；外层卡片 ID 保持稳定，保留焦点与滚动身份。图片元信息另放到图片之后的绝对布局层，像素回归覆盖浅色、深色和预览状态。
- 最新原生受控 `--self-test` 结果（历史产物已从项目移除：`../experiments/mygo-clipboard/build/verification.json`）中可自动执行的检查全部通过：文本/HTML/PNG 真实剪贴板往返、增强确认前不写入、范围继承、回写去重、普通复制不继承旧限制、事件监听器启动/停止、全局快捷键注册、菜单栏、非激活面板前台应用保持和剪贴板恢复。报告明确保留 `real_target_paste_blocking: not_tested` 与 `ai_provider: not_configured`，没有把这两项计为通过。
- 本轮实机初次启动确认是原生面板；后续可检查窗口中完成了上述真实剪贴板流程。
- 桌面工具的按键会直接发送给目标应用，未必改变系统前台应用，也不可靠地经过全局事件监听器。因此自动操作时的来源可能显示工具前台应用，不能把它算作来源识别或物理 `⌃⌘C`/`⌘V` 守卫验证。
- 旧版本用户实际键盘验证：来源应用内粘贴成功，其他应用被阻止。保留旧版本来源限定复制记录（历史产物已从项目移除：`../experiments/mygo-clipboard/build/protected-copy-verification.json`）与旧交互记录（历史产物已从项目移除：`../experiments/mygo-clipboard/build/interactive-verification.json`）。这不是本轮新构建的物理按键验收。
- 用户完成触控 ID 认证后，已通过系统设置刷新同一路径的 Copiggy 辅助功能条目；再次从桌面独立启动当前构建，界面显示实际监听器已开启。最终构建实机确认图片卡的类型、时间、来源图标和尺寸正常显示。
- **本轮最终原生构建的物理 `⌃⌘C` / `⌘V` 守卫已通过用户实测。** 在文本编辑中保护复制后，用户确认“同应用成功，跨应用被阻止”；目标为文本编辑与系统设置搜索框。结果保存在 本轮验收记录（历史产物已从项目移除：`../experiments/mygo-clipboard/build/native-ui-verification.json`）。
- 范围继承与普通复制解除旧限制已由模型/真实剪贴板受控测试覆盖；最终原生构建的受限增强结果尚未单独做物理键盘复核。权限撤销、无选区和快速切换应用等异常场景仍需完整发布验收。

保护范围仍是标准粘贴快捷键，不能约束菜单粘贴、后台读取或同一应用里的错误会话。AI、持久化和跨设备同步尚未接入。

本轮最终应用包的 ad-hoc CDHash：`40e68f8fe7c3d0c8e541da799248c3f62595a401`。刷新授权及后续物理按键验收应针对这一构建。
