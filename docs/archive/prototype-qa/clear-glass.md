> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy 扁平深浅模式 · 排版与布局验收

日期：2026-10-20。范围：按用户参考整体调整配色、布局、间距和字体，去除毛玻璃，保留深浅模式。

final result: passed

## 参考与比较证据

- 视觉参考：`qa/flat/reference-1.png`、`reference-2.png`，均为 2048 × 1536。
  提取实色黑白面、圆角几何、清晰字级、充足留白和功能色；按照用户要求不复制局部光影。
  保留剪贴板连续历史列表，不采用参考的小组件网格。
- 本轮证据均在 `qa/refined-flat/`。`light.png`、`dark.png` 和 `before.png`
  的 viewport 及截图均为 1200 × 900；截图输出归一到 CSS 像素，不能按设备 DPR 再放大。
  最终展开面板约 1000 × 550，位置 x=100、y=80。
- 前后及深浅截图状态一致：图片已选中、当前剪贴板为普通文本、保护开启、搜索为空、没有草稿。
- 已实际打开 `reference-comparison.png`：两张完整参考与两种实现置于同一画布，均等比缩放。
  已打开 `before-after.png`：相同坐标区域按原像素并列，用于检查字号、间距、对齐变化。
- `comparison.png` 为两种模式完整截图的等比并列图，保留原型模式切换入口。
  参考图片像素密度未知，采用设计语言对照而非跨产品逐像素复刻。
- 响应式证据：`medium-dark-draft.png` 724 × 932、`breakpoint.png` 681 × 900、
  `narrow.png` 390 × 844；均覆盖 JSON 草稿与保护关闭的组合状态。
- 上轮报告及 CSS 已归档为 `previous-design-qa.md`、`previous-styles.css`。

## 五项视觉检查与本轮调整

- **字体：** 系统无衬线字体；历史内容 18px/26px、weight 500，辅助信息 13px/18px；
  预览正文 16px/1.75，代码 15px/1.75，搜索 20px。中窄窗口按断点适度收紧。
  主次信息明确，避免把参考的大数字样式照搬到长文本。
- **布局与间距：** 外边距统一 24px、主列间距 32px，历史与预览约 52:48。
  历史行 76px、搜索高 52px、底栏至少 72px；展开宽 1000px，默认宽 640px。
  图片改为左对齐，与底部信息共用起点；预览操作统一为 40px 独立圆形按钮。
  无内部划线；保持内容在左、来源图标在右、范围锁在应用图标右上角。
- **颜色：** 中性灰取代偏暖灰，浅色 #ffffff 面板/#ebebeb 画布，深色 #202020/#141414。
  选中、搜索、按钮用平整色面区分；绿色仅用于保护与草稿状态，琥珀用于失效警告。
  无透明面、毛玻璃、渐变、装饰阴影或光晕。
- **图片与图标：** 保留原图片和真实来源应用图标，图片等比展示，不裁切内容。
  来源图标 26px、锁徽标 16px；动作图标不变，触达区域增大。键帽维持一键一块。
- **内容与文案：** 没有新增名称栏、类型标题或取用按钮。纯文本仍只有正文及字符数。
  草稿、范围继承和保护失效提示保持可见。示例 JSON 的 glass 为用户内容示例，未修改。

## 验证结果

- 两主题切换保留选中内容；JSON 格式化草稿及关闭保护的状态跨主题保持。
- Esc 取消草稿，开启保护，搜索“让每一次”，方向键选中普通文本：
  正文与字符数正常，且没有处理按钮。清空搜索后全部历史恢复，图片选择正常。
- 724px 保持分栏，元信息与操作之间有约 58px 间距。
- 681px 分栏临界点：元信息右边界 512.72、按钮左边界 550，无重叠；
  document.scrollWidth = viewportWidth = 681。
- 390px 上下布局：元信息右边界 185.31、按钮左边界 265；
  底栏位于 x=13–377、y=677–749，控件完整；clientWidth = scrollWidth = 390。
- 独立视觉复核覆盖 light、dark、medium-dark-draft、narrow，与两张参考对照，
  未发现 P0/P1/P2 问题。最终 681px 截图亦已实际打开检查。
- 本轮 `npm run build` 通过；浏览器 warn/error 为空。已恢复用户窗口尺寸并保留可见预览。
- 本轮没有修改应用交互逻辑；持久化主题选择沿用既有实现。没有重建原生 MyGo 验证应用。

## 实施清单

- [x] 中性黑白配色、实色面板，无玻璃光影。
- [x] 统一间距和字级，扩大预览，内容与信息共用左边线。
- [x] 调整控件比例，保留图标、锁徽标与独立键帽。
- [x] 两主题、草稿警告、普通文本、搜索、响应式及构建验证。
- [x] 同步 README、设计说明和 AGENTS.md，更新可见原型。

网页仍使用示例内容；此报告仅验收本轮视觉与交互回归，不作为真实系统剪贴板拦截验证。
