> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy 浅色透明玻璃与深色对照 · 设计验收

日期：2026-10-20。范围：网页原型的浅色材质与外观对照，不涉及原生应用重建。

final result: passed

## 参考与证据

- Source visual truth：`qa/light-glass/reference.png`，用户提供的 1280 × 934 截图。
  只参考透色、背景模糊、白色外缘高光和深色文字，不复制启动器布局、标题和分割线。
- 既有深色基准：`qa/smoked-glass/final.png`。保留其材质与已确认产品结构。
- 最终实现：`qa/light-glass/light-final.png`、`dark-final.png`，均为 822 × 898 输出。
  CSS viewport 为 822 × 898；浏览器报告 devicePixelRatio=2，截图 API 已归一到 CSS 像素尺寸。
- 同状态对照：`qa/light-glass/comparison.png`，选中图片、当前内容为普通文本、保护开启，
  搜索为空，无草稿。两图按相同 (8,80)–(814,725) 坐标裁切，保留场景和外观切换入口，未缩放。
- 已打开并检查 `qa/light-glass/reference-comparison.png`：参考等比缩到 640 × 467，
  实现裁切 772 × 494。它们属于不同产品，比较材质而非逐像素布局。
- 已打开并检查 `qa/light-glass/material-detail.png`：两侧均为 380 × 350 原始像素局部；
  参考密度未知，字体大小差异不作为偏差，用于观察外缘、选中底色和文字层级。
- 其他状态：`text.png`、`draft-warning.png`、`narrow.png`、`zoom.png`。
  窄屏请求 viewport 390 × 844，实际内容宽 375（滚动条占位），截图 API 输出 375 × 812；
  只用它检查控件可见和溢出，不与桌面进行像素尺寸对比。检查后已恢复默认 viewport。

## 发现与修复

1. **P2，已修复：浅色辅助文字对比不足。**
   初版 `light.png` 中选中行背景采样 (177,191,202)，辅助字 #555b64 对比约 3.64:1。
   调整 `--muted` 为 #434951，保留玻璃透明度；最终同处约 4.84:1，普通列表和图片元信息
   同类背景采样约 5.84:1 / 5.80:1。演示区另用 #25282e，避免照片背景吞没小字。
   最终截图及合成对照已重新打开检查，未再发现阻塞差异。
2. **预防主题混搭：** 图片放大遮罩维持深底并固定浅字；toast、警告、应用小锁、键帽、
   开关、按钮、下拉框及滚动条均有浅色处理。减少透明度分别设置浅灰/深灰不透明回退。
3. 无未解决 P0/P1/P2。未对任意桌面背景和真实系统减少透明度偏好作通过承诺。

## 五项视觉检查

- **字体与层级：** 沿用系统字体、17px 历史标题、12px 辅助字、14px 等宽 JSON。
  小字通过加深颜色而非改变密度保持清晰；长内容省略和右侧正文换行正常。
- **布局与间距：** 两种主题共用完全相同布局。全宽搜索、内容优先列表、右侧上正文下信息、
  底部按键提示与防误贴 toggle 均保留，无内部线。新增对照切换位于面板外。
- **颜色与材质：** 浅色采用 46% 白色表面、32px 背景模糊、柔和白色边缘；真实海岸背景透出
  蓝与暖灰色彩。参考中的紫色来自其背景，不硬编码到应用。深色保持原烟灰效果。
- **图像与图标：** 沿用真实海岸照片、来源应用图标及 Phosphor 操作图标。图片预览不拉伸、
  不染色。小锁位于来源图标右上角；关闭保护时琥珀色小锁与文字警告共同表达状态。
- **文案与内容：** 未增加产品标题、类型标题或主操作按钮。纯文本只有正文和字符数；
  JSON 草稿/原文对照/继承范围与保护失效提示保持原义。

## 实际验证

- 图片预览状态切换深浅主题：仅外观变化，选中、当前内容和面板展开状态保留。
- 搜索 JSON → 选择 → 格式化 → 关闭保护 → 切浅色：搜索词、草稿、限制范围、关闭状态保留。
- 查看原文 → 切深色：原文对照与草稿继续保留。放弃草稿、清空搜索与重新开启保护正常。
- 普通文本不显示处理按钮；图片放大/关闭正常，深色遮罩文字为 rgb(247,247,242)。
- 窄屏无水平溢出，document.scrollWidth=375，footer 范围 x=11–364，控件完整；已恢复桌面。
- 浏览器 warn/error 为空，最终 `npm run build` 通过。未为纯视觉变化新增镜像测试。
- 独立只读检查覆盖主题硬编码与状态隔离，README 与产品文档同步更新。

## 实施清单与后续

- [x] 浅色玻璃默认展示，保留深色主题。
- [x] 外观切换不重置产品状态，不触发点击外部收起。
- [x] 改善浅色小字对比，重新截图并检查参考/实现合成图。
- [x] 保留本地预览，提供同状态对照图。

网页材质仍是视觉近似；原生 MyGo 材质、任意桌面背景适配与真实系统偏好响应需后续验证。

## 追加验证：确定保留双模式与主题记忆

同日用户确认浅色/深色均作为产品外观保留。新增 `copiggy.appearance` 本地偏好，
无有效记录默认浅色，存储不可用时仍可在当前会话切换；未改变 CSS 或布局。
实际点击深色后刷新，深色按钮 aria-pressed 仍为 true；切回浅色后刷新，同样保留。
证据：`qa/light-glass/dark-remembered.png`、`light-remembered.png`，
入口局部 `theme-control.png`。构建通过，控制台无 warn/error。
原视觉验收继续适用，final result: passed。
