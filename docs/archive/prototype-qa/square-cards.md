> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# 卡片历史与浮层预览 · Design QA

final result: passed

## 目标与证据

本轮是经用户授权的重新布局，借鉴卡片和浮层结构，不逐像素复制竞品内容或分享功能。

- 参考卡片：`/var/folders/hv/xmdf2pn1469fklvql7h21fl00000gn/T/codex-clipboard-c6d9e392-7cab-4818-9ff5-0a5bdb5d56f9.png`，1680 × 800。
- 参考浮层：`/var/folders/hv/xmdf2pn1469fklvql7h21fl00000gn/T/codex-clipboard-19b19902-0647-4af7-b778-0a78fd5d3d6e.png`，1680 × 960。
- 实现：`http://127.0.0.1:4173/`，由 Codex IAB 实际操作和截图。
- 主要证据：`qa/cards-popover/light-default.png`、`light-image.png`、`dark-image.png`、`light-json.png`、`dark-json-final.png`、`light-text.png`、`dark-restricted.png`。
- 搜索与响应式：`dark-empty.png`、`dark-search-link.png`、`medium-dark-image.png`、`narrow-dark-image.png`。
- 完整同屏对照：`qa/cards-popover/reference-comparison.png`，1800 × 1240，已打开逐项比较。
- 局部同屏对照：`qa/cards-popover/detail-comparison.png`，1200 × 1160，已打开检查卡片类型、字号层级、来源图标及浮层提示。

桌面 CSS viewport 为 1200 × 900，截图同为 1200 × 900；另验证 681 × 900 与 390 × 844，截图均按 CSS 像素输出。浏览器 devicePixelRatio=2，截图接口已归一到 CSS 尺寸；最终用户窗口恢复为 902 × 932，记录在 `qa/cards-popover/final-preview.png`。参考为营销场景图，无法确认原始 CSS 密度；组合对照使用等比缩放和明确裁切，不将设备边框、橙色桌面、英文分享内容与中文剪贴板示例的差别作为偏差。卡片参考裁切为 (390,104)–(1680,450)，实现裁切为 (30,418)–(1170,806)。两个产品的状态、内容和尺度不同，本轮比较结构与视觉原则，非像素一致性声明。

## 五项视觉检查

- **字体与排版**：系统无衬线用于界面和中文正文，SFMono/Menlo 用于 JSON。卡片正文 16px，短文本 23px；次要来源/时间 11–12px。短文突出、长文截断、代码保留缩进，浮层完整显示原始内容。中文没有照搬英文衬线字体。
- **布局与间距**：1140px 上限的单层托盘，204 × 226px 卡片，16px 卡间距；搜索上置，底部开关与单键提示。独立预览随所选卡片定位，宽 440px，按内容和可用高度收缩。取消固定右栏，无内部隔线。窄屏保持可横向滚动的卡片，不压成不可读的小格子。
- **颜色与材质**：深浅两种玻璃外壳，卡片根据内容呈现代码深底、便签暖底、图片和中性链接底。深浅都保留背景透色，控件扁平；选中轮廓与当前剪贴板勾标区分。浅色预览的代码颜色进一步压深，状态警告有文字而非只靠颜色。
- **图片与图标**：复用真实海岸图和原有系统来源图标；缩略图裁切用于辨认，浮层保持原图比例且居中，放大层展示完整图片。Phosphor 图标用于常规控件和 GitHub 标志，无 CSS 插画或伪造网页摘要。来源小锁保持右上角，不抢占内容区域。
- **文案与内容**：没有产品标题栏、粘贴按钮或“设为当前”主按钮；预览没有重复 JSON 大标题。纯文本和链接无处理工具。图片只出现放大，JSON 才出现格式化；草稿显示原文/结果及继承范围，Enter 提示对应当前所见内容。

## 发现、修复与复核记录

1. **[P2 已修复] 草稿的两组键提示冲突**：初版 `dark-json.png` 中浮层“取用结果/放弃”与托盘“取用/收起”同时出现。现在浮层打开时托盘只留导航提示；`dark-json-final.png` 及局部对照确认只保留一组取用/放弃提示。
2. **[P2 已修复] 窄窗口浮层压住搜索上缘**：390 × 844 下页面滚动后，初始浮层高度 347px 超过可用上方区域。改为按托盘到视口顶部的可用距离限制高度，图片可缩小。最终 `narrow-dark-image.png` 中浮层 y=16..324，托盘 y=340..718，保留 16px 间隔，页面无横向溢出。
3. **[P2 已修复] 边界导航丢弃草稿**：首张卡按 ← 旧逻辑重选自身并取消草稿。现在同一 id 不重选；浏览器验证首张 JSON 草稿按 ← 后仍为草稿。
4. **[P2 已修复] 处理焦点与原文语义**：格式化开始时将焦点放在预览，处理中 Esc 可取消；结束不抢回用户已转移的焦点。原文对照 Enter 取用原文，结果视图 Enter 保存副本，警告文案统一为“取用将替换它”。
5. **[P2 已修复] 短文本预览留白过大**：固定高度改成随内容自适应；`light-text.png` 与 `light-image.png` 显示同一上下结构、不同合理高度。

复核后无剩余 P0/P1/P2。

## 实际交互验证

- 默认只有卡片；单击选中而不取用、不打开预览；搜索 ↓ 进入所选卡片。
- 卡片 Space、搜索 ⌘Y 唤起；←/→ 在浮层内切换内容；Esc 收起且回焦原卡片。
- 搜索框输入两个空格仍为输入，不触发预览；按来源 Safari 筛出图片和链接；空结果预览按钮禁用且 ⌘Y 无空浮层。
- JSON 格式化显示草稿，原文/结果对照可切换；Esc 放弃后再次打开为原文。处理中取消后迟到回调未写入草稿。
- 结果视图 Enter 生成第 6 张副本且保留原始项、来源与应用锁；原文对照 Enter 不新增副本（保持 5 张），当前标记转到原始项。
- 模拟粘贴验证增强副本在 Safari 被阻止，在文本编辑成功显示。这里验证网页状态模型，不声称重测原生键盘拦截。
- 图片可从浮层放大；放大层 Esc 只关最上层，焦点回到“放大查看”。
- 防误贴 toggle 开关、锁角标失效状态和预览警告同步；深浅切换保留所选内容与草稿。
- 1200、681、390px 三种宽度检查；所选卡片横向滚入可见区域；浮层按可用区域调整位置与高度。
- 控制台 warn/error：空。Vite production build 及 Sites 本地产物准备均成功；未发布。

## 边界与后续

- 仅网页交互原型和示例数据；没有读写系统剪贴板，没有重建原生应用或改变系统授权。
- 暂无文件型卡片、远端网页元信息、AI 服务接入；这轮的增强入口为本地 JSON 格式化。
- 这是键盘为主的桌面工具，窄屏用于检查布局边界；不表示新增移动端产品。
- P3 后续可在更多真实长文本/大 JSON/不同图片比例下微调卡片密度。

## 实施检查

- [x] 类型卡片和浮层参考同屏对照
- [x] 深浅默认、预览、草稿、限制和搜索状态
- [x] 焦点、取消、范围继承与所见即所得
- [x] 窄屏边界、图片比例、构建与控制台
- [x] 同步 AGENTS.md、README.md 和产品设计说明
