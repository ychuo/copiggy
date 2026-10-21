> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy · 玻璃外壳与扁平控件验收

日期：2026-10-20。

final result: passed

## 视觉目标与证据

用户确认恢复玻璃外壳，同时保留 refined-flat 的字号、留白、列表与右侧预览流程。
参考为先前用户认可的 `qa/light-glass/light-final.png` / `dark-final.png`（822 × 898），
及本轮 Clipbara/Paste 调研 `../../docs/competitor-review.md`。这次是现有原型的材质迭代，不是竞品逐像素复刻。

最终截图 `qa/clear-glass/light.png` / `dark.png` 均为 1200 × 900 CSS viewport、1200 × 900 输出，
截图 API 已归一到 CSS 像素，不再乘设备 DPR。两张状态一致：图片选中、无草稿、保护开启、搜索为空。
展开面板 1000 × 550，位置 x=100、y=80；初始面板宽640。历史旧图 viewport 不同，排版沿用用户批准的 refined-flat 规格，比较不将尺寸差异误判为材质漂移。

已实际打开以下合成及证据：
- `qa/clear-glass/reference-comparison.png`：先前玻璃版本与本轮两主题置于同一画布，均保持原始比例。
- `qa/clear-glass/comparison.png`：最终深浅模式，同状态、同尺寸、同倍率。
- `qa/clear-glass/detail.png`：最终列表、来源锁角标、元信息与操作区域按原像素裁切，用于检查细节。
- `qa/clear-glass/default.png`：默认历史列表（玻璃层裁切修正前；结构和文案未改变）。
- `qa/clear-glass/dark-draft.png`：修正后最终草稿、范围继承、防误贴失效提示。
- `qa/clear-glass/medium.png`：681 × 900 分栏状态。
- `qa/clear-glass/narrow.png`：390 × 844 上下状态，完整产品面板和所有底栏控件可见；页面下方原型演示控件可滚动访问。

## 发现及修正历史

- [P2，已修复] 深色选中行的辅助字在理论全白底上对比度不足。
  初始 muted #c3ccd7、白色选中叠层 .13 的理论合成约3.61:1；改为 #d1d8e0 与 .08 后约4.61:1。
  此核算不依赖展示壁纸的暗罩。浅色在理论全黑底上，选中行辅助字约4.64:1。
  最终 `dark.png` 和 `dark-draft.png` 实际检查清晰。此为 sRGB 叠色核算，不等同于全平台材质可访问性认证。
- [P2，已修复] 在窗口本体设置 backdrop-filter 时，草稿截图出现矩形模糊区域越出圆角窗口。
  证据保留为 `dark-draft-initial.png`；仅设置 overflow:hidden 的第一次尝试未消除该问题。
  最终将唯一材质层置于 `.panel::before`，使用明确的圆角 clip-path；窗口外阴影仍由父元素承载。
  `dark-draft.png` 的同状态复查确认模糊区域仅位于窗口内，深浅图片状态亦重新截图。
- 未解决 P0/P1/P2：无。独立视觉复核已检查最终 light、dark、dark-draft 三张截图。

## 五项视觉检查

- **字体与排版：** 保留系统无衬线、18px/26px历史内容、13px辅助字、16px正文、15px等宽代码及20px搜索。
  文字本身不透明，长内容换行/截断正常，图标与文字权重保持稳定。
- **空间与布局：** 保留24px外围留白、32px列间距、约52:48比例、76px历史行和40px圆形操作按钮。
  图片与底部信息左边线一致，没有重复类型标题、内部划线或嵌套操作胶囊。
- **颜色与材质：** 浅色 rgba(248,251,255,.74)，深色 rgba(25,31,39,.78)，单一36px模糊材质层。
  仅外缘细亮线及轻外投影；内部搜索、按钮、键帽均为无阴影的平面填充。
  保护绿色保持克制，失效警告仍是清楚的琥珀色文字与实底。
- **图像与图标：** 使用既有 coast.png 作为演示桌面背景，图片本体及应用图标未修改。
  真实背景色能够透入面板；图片按原比例展示，来源小锁未被裁切。
- **文案与内容：** 保留现有少文字原则、独立键帽、范围继承及必要警告。
  原型主题/演示工具仍在产品面板外，改用实色底座保证照片上的小字可读。

## 交互及构建验证

- 选择图片、切换两主题：选中记录与当前剪贴板样例保持。
- 选择 JSON → 格式化 → 关闭防误贴 → 切换主题：草稿、继承范围与失效提示保持。
- 放弃草稿 → 开启保护 → 搜索“让每一次” → 方向键选中：普通文本仅显示正文与字符数，无处理工具。
- 清空搜索恢复完整历史；图片放大弹层可打开并通过关闭按钮返回。
- 681px：scrollWidth=681，元信息右边界512.72、工具左边界550，无重叠。
- 390px：scrollWidth=390，无横向溢出；滚回页面顶部后，产品搜索、预览和底栏均可见。
- 增加 prefers-reduced-transparency 的实色回退及不支持 backdrop-filter 的回退；未模拟操作系统偏好，回退做源码检查。
- 最终浏览器 warn/error 日志为空。已恢复用户窗口尺寸、显示并保留原型标签页。
- 最终使用内置 Node 顺序运行 Vite build 和 scripts/prepare-sites-build.mjs，两步通过。
  当前 shell 没有 npm；备用 pnpm 触发了自动依赖安装并遇到不可达镜像，已中断，恢复其移动的五个既有依赖后直接执行构建。没有升级依赖。

## 实施清单

- [x] 恢复一层连续玻璃外壳，保留两个主题及选择记忆。
- [x] 保留新排版和现有交互，内部控件无装饰高光。
- [x] 修复深色辅助字对比及玻璃圆角溢出。
- [x] 复核默认、图片、JSON草稿、保护关闭、搜索与窄窗口。
- [x] 同步 AGENTS、README、设计说明、截图和本报告。

边界：这是网页材质/交互原型，背景为演示照片，不是读取用户真实桌面。未重建原生 MyGo 应用，原生系统材质与防误贴能力没有在本轮重新验证。
