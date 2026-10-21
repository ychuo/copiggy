> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy 扁平深浅模式 · 设计验收

日期：2026-10-20。范围：用户批准的实色扁平方向与中等宽度分栏优化。

final result: passed

## 参考与比较证据

- Source visual truth：`qa/flat/reference-1.png`、`reference-2.png`，均为 2048 × 1536 的用户参考图。
  采用黑白实色面、圆角几何、信息层级和少量功能色；用户明确要求去除光影，
  因而未复制参考中局部光晕、渐变、阴影和分割线，也未照搬小组件卡片网格。
- 最终实现：`qa/flat/light-final.png`、`dark-final.png`。CSS viewport 与截图输出均为
  1200 × 850，面板 1040 × 482，位置 x=80、y=104；截图 API 输出已归一到 CSS 像素尺寸。
- 两主题同状态：选中图片、当前内容为普通文本、保护开启、搜索为空、无草稿。
  `qa/flat/comparison.png` 按相同坐标裁切并等比缩至 80%，保留模式入口。
- 全局参考/实现合成：`qa/flat/reference-comparison.png`，参考分别缩至宽 720，
  实现同样等比缩至宽 720。已实际打开检查。不同产品的布局不作逐像素复制判断。
- 局部合成：`qa/flat/material-detail.png`，参考白/炭黑实色区域与实现列表区域均按原像素裁切。
  已打开核对色面、字色、圆角、边缘与控件。源图字体密度未知，不将字号差异误判为漂移。
- 响应式证据：`medium.png` 为 724 × 932；`narrow.png` 为 390 × 844。
- 状态证据：`dark-draft-warning.png`、`light-draft-warning.png`。
- `before.png` 与 `previous-styles.css` 仅用于历史归档；与最终截图状态不同，不作同状态比较。

## 发现、调整与结果

- 旧的 800px 断点使中等桌面窗口过早变为上下排版。用户授权必要布局优化后，
  将详情堆叠阈值改为 680px，保留原组件尺寸与交互。
  最终 724px 截图及 DOM 确认左右排列、无横向溢出；390px 保持纵向排列，底部控件完整。
- 视觉比较未发现未解决的 P0/P1/P2。两种模式均为不透明实色表面，没有装饰光影残留。
- 参考中的立体应用图标和照片作为实际内容继续保留，不通过滤镜破坏来源辨识。
  “扁平”适用于界面外壳和控件，并非将用户内容去色。

## 五项视觉检查

- **字体与排版：** 保持系统字体、17px 历史标题、12px 辅助字、14px 等宽代码；
  内容、时间、范围层级清晰。长列表标题截断、右侧正文换行行为不变。
- **空间与布局：** 保留 34px 面板圆角、全宽搜索、连续列表、来源图标右置及锁角标。
  右侧上方为主体、底部为信息与相关操作；无内部分割线。中等宽度保留分栏。
- **颜色与状态：** 浅色白面板/中性浅灰底，深色 #202020 面板/#151515 底。
  搜索、选中、键帽及操作组通过平整色块区分；防误贴用克制绿色，失效警告用琥珀色。
  CSS 颜色核算：浅色辅助字/选中底约 5.00:1，浅色辅助字/控件底约 5.14:1；
  深色辅助字/选中底约 6.18:1；两主题警告均超过 6:1。
- **图像与图标：** 保留已有真实图片和来源应用图标，Phosphor 动作图标不变。
  图片等比显示。锁徽标用实色面及细边界，开关滑块、圆形按钮均无阴影。
- **文案与内容：** 不增加产品标题、预览类型标题或取用按钮。普通文本只有正文与字符数；
  JSON 草稿、范围继承、保护未生效的必要提示仍在。示例 JSON 中的 glass 是演示内容，未改动。

## 验证

- 浏览器选择图片、切换深浅模式，选中记录和当前内容保留。
- 选择 JSON → 格式化 → 关闭防误贴 → 切换浅色，草稿、范围与警告均保留。
- Esc 放弃草稿，方向键选择普通文本，恢复保护；普通文本没有内容处理按钮。
- 当前渲染 DOM 中面板及内部控件的 box-shadow / backdrop-filter 非 none 元素数为 0。
  源样式无渐变、模糊、阴影规则；页面装饰照片背景元素已移除。
- 724px：flex-direction=row，scrollWidth=724。390px：flex-direction=column，
  clientWidth=scrollWidth=390；footer 左右边界 11–379，控件未溢出。已恢复用户窗口尺寸。
- 控制台 warn/error 为空；最终 npm run build 通过。独立视觉复核无阻塞意见。
- 模式持久化逻辑未修改，沿用上一轮刷新验证；本轮实际验证两种模式与草稿状态切换。

## 实施清单

- [x] 用统一主题变量替代玻璃样式与重复浅色覆盖规则。
- [x] 移除透底、模糊、高光、阴影，保留平整键帽与必要焦点指示。
- [x] 保留深浅模式及选择记忆、原有键盘和草稿流程。
- [x] 检查两主题、警告/草稿及响应式布局，更新文档和预览。

网页仍为示例数据交互原型；本轮未重建或改动原生 MyGo 验证应用。
