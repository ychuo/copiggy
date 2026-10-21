> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# 一体玻璃预览 · Design QA

日期：2026-10-21
Final result: passed

## 范围与参考

用户要求底部尖角与预览浮层融为一体，无明显接缝，玻璃材质保持一致。本轮只调整浮层表面轮廓，沿用既有卡片布局、内容、键盘操作与深浅主题。此前报告和源码备份在 `qa/seamless-preview/previous-*`。

同一 1280 × 720 视口、同一 JSON 预览状态：
- 改前 `qa/seamless-preview/before-dark.png`。
- 改后 `qa/seamless-preview/after-dark.png`、`after-light.png`。
- 同屏对照 `qa/seamless-preview/comparison.png`，包括连接区域的 4 倍像素放大，已打开检查。
- 图片与右边缘状态：`light-image.png`、`light-right.png`。

## 视觉检查

- 字体：继承现有系统字体、等宽 JSON 和原字号，未发生文本重排回归。
- 布局：维持主体宽 440px、正方形卡片 204px、尾尖距所选卡片 6px。边缘卡片定位实测正确。尖角根部安全偏移增至 42px，防止进入主体圆角。
- 颜色与材质：主体与尖角由同一个背景层绘制，统一 `var(--glass)` 和 36px backdrop blur；边缘仅为一条连续外轮廓。暗色原有横穿尖角的底线和叠色已消除；浅色填充连续。未引入高光、额外光斑或渐变。
- 图片与图标：图片、来源图标、类型图标未改动；独立的装饰 CaretDown 被合并到容器几何轮廓中。
- 文案：内容和提示保持不变；没有增加说明、操作或 CTA。

## 实现与验证

- 背景层一次性裁剪圆角主体和尖角；相同外路径与内缩路径组成 evenodd 细边缘，根部没有矩形底边或独立箭头描边。
- 裁剪仅作用于装饰伪元素；没有裁剪交互内容或给父元素加 filter，不改变背景采样层级。
- 继续继承已有减少透明度与不支持 backdrop-filter 的表面回退规则。提高背景选择器优先级，防止窄屏通用规则覆盖连续轮廓。
- 浏览器检查 JSON、文本、图片切换后的高度与轮廓重算；最右卡片浮层避让视口，尖角仍指向选中卡片。
- 实测右边缘状态：卡片 top=343，浮层 bottom=321，尖角高16，间隔6px。背景计算样式为 blur(36px) saturate(0.85)。
- 独立 QA 页没有 warn/error。
- Vite production build 与 prepare-sites-build 通过；未改原生应用或系统权限。

本轮未发现未解决的 P0/P1/P2 问题。未重复全量原生及小屏幕测试；本轮证据范围为上述局部浏览器验证。
