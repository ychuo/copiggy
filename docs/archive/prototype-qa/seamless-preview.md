> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# 正方形卡片与贴边预览 · Design QA

日期：2026-10-21
最终结果：passed
范围：网页交互原型；本轮四项简化、底部尖角和贴近卡片的定位。原生应用未改动。

## 参考与证据

用户指定改动：204 × 204px 正方形历史卡片；圆形搜索按需展开；类型用图标；防误贴提示简化；预览下方尖角靠近选中卡片上边缘。

- 原始提示截图：`/var/folders/hv/xmdf2pn1469fklvql7h21fl00000gn/T/codex-clipboard-b46cf42e-d10e-4cd8-aa07-5642462e5923.png`，962 × 738。
- `qa/square-cards/light-default.png`、`light-search.png`：1200 × 900。
- `qa/square-cards/light-restricted.png`、`light-right-anchor.png`、`dark-restricted.png`、`dark-image.png`：902 × 932。
- `qa/square-cards/comparison.png`：1440 × 1120，已打开逐项检查。参考是预览局部，当前截图是整个应用；比较图按区域缩放，并标注实际 CSS 尺寸，未把缩放差异当作字号变化。
- 保留此前版本：`qa/square-cards/previous-App.jsx`、`previous-styles.css`、`previous-design-qa.md`。

## 五项视觉检查

- 字体：界面沿用系统无衬线，JSON 用等宽字体；普通正文 15px、短文本 22px，类型和时间为次要信息。类型图标有 tooltip 和可访问名称。
- 布局：卡片实际测量均为 204 × 204px；圆形搜索和预览按钮相同尺寸。浮层定位锚点改为选中卡片上缘，底边预留 22px，16px 尖角后约 6px 空隙。首卡和末卡均已验证：窗口边缘限制浮层，尖角独立指向卡片中心，边框导致中心差约 1px。浮层可覆盖搜索栏，⌘F 收起浮层进入搜索。
- 颜色：保留深浅玻璃面板、各类型卡片表面、选中轮廓；未生效范围采用暖色单行提示，仍可识别状态。
- 图片与图标：沿用海岸图片、系统来源应用图标与 Phosphor 图标；图片预览保持比例；底部尖角使用库内 CaretDown。来源锁徽标位置不变。
- 文案：删除重复大警告条，范围行合并为“仅限文本编辑 · 未生效”；关闭开关旁只显示“已关闭”，详细解释保留在 tooltip。没有新增标题、粘贴 CTA 或分割线。

## 交互验证

- 点击圆形搜索或 ⌘F 展开并聚焦；从内容浮层按 ⌘F 会关闭浮层并进入搜索。
- 搜索 Safari 返回图片和链接两项；第一次 Esc 清空，第二次 Esc 收起搜索并恢复所选卡片焦点。
- 空格打开 JSON、纯文本和图片预览；纯文本无处理区；类型图标可访问名称正确。
- 首卡、末卡的尖角指向和间距已按 DOM 几何检查；所选卡片横向滚入可见区域。
- 深浅模式切换保留选择和预览；防误贴关闭时显示紧凑范围行。
- 新独立 QA 页控制台无 warn/error。原用户页面未重载，避免打断操作。
- Vite production build 通过；prepare-sites-build 通过。无新增自动化测试；本轮为可逆局部 UI 变更。

未发现本轮范围内未解决的 P0/P1/P2 问题。窄手机尺寸未在本轮重复验证；上轮相关报告留在 previous-design-qa.md，不作为本轮证据。
