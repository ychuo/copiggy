> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy 统一预览与防误贴控件 · 设计验收

日期：2026-10-20。验收对象：网页交互原型。

final result: passed

## 当前设计依据

用户提供的右侧预览截图与本轮补充要求共同构成设计目标：无类型标题；
右侧统一为上方内容、下方信息和操作；普通文本没有操作按钮；
左下角直接切换防误贴，来源应用右上角显示限制锁徽标；继续不使用内部分割线。

- 用户来源图：`/var/folders/hv/xmdf2pn1469fklvql7h21fl00000gn/T/codex-clipboard-4da84716-7381-4a9e-8818-2ea7f69f9393.png`。
- 官方来源和方案取舍：`../../docs/preview-research-2026-10-20.md`。
- 原界面实测捕获：`qa/preview/01-before-json.png`，1200 × 900 viewport/pixels。
- 首轮去标题后的对比：`qa/preview/comparison.png`。源和实现同为 1200 × 900，
  面板等尺寸裁切，未拉伸。这个版本的内容旁动作位置已被用户后续要求替代。
- 当前统一布局：`qa/preview/unified-json.png`、`unified-image.png`、`unified-text.png`、
  `unified-draft.png`、`unified-off.png`。默认 viewport 和截图均为 822 × 898，输出密度为 1。
- 当前四状态放在同一画布实际查看：`qa/preview/unified-comparison.png`。
  所有面板按相同坐标裁切、1:1 合成，没有缩放字体；状态切换可能改变历史滚动位置。
- 锁徽标及开关局部：`qa/preview/badge-toggle-detail.png`，锁区域等比放大两倍以检查重叠。
- 窄窗口：`qa/preview/unified-narrow.png`，390 × 844。

本轮是受约束的设计修订，不要求复刻被用户否定的旧标题和动作位置。
最终比较以用户的最新结构要求为准，旧版本作为问题证据。

## 发现、修复与复查

1. **P2：右侧重复类型标题、关闭叉和工具栏形成独立页面。**
   `01-before-json.png` 可见内容被多个控制层推后。改为全宽搜索，预览直接显示内容，
   底部统一信息与操作。`unified-comparison.png` 确认文本、图片、JSON 使用同一框架。
2. **P2：不同内容的动作位置不一致。**
   首轮将格式化紧随文本、放大放在图片角落；用户后续明确要求统一上下结构。
   现内容在可滚动上区；信息与图标按钮固定下区。实测三类底栏 y=475、bottom=519，
   图片和 JSON 按钮位置一致。纯文本只有字符数，不产生操作按钮。
3. **P2：历史超过五条后，系统浅色滚动轨道像一条竖分隔线。**
   `08-scroll-track-before.png` 是修复前；轨道改透明、滑块用弱色。
   实际查看 `scroll-track-comparison.png`，当前四状态图确认不再出现整条白色轨道。
4. **用户指定：防误贴直接开关与图标角标。**
   移除状态浮层；switch 用盾牌勾/斜线盾牌、位置和颜色表达开启/关闭。
   来源图标 24 × 24，小锁 14 × 14，在右上方各溢出 4px，仍在行内边距中。
   `unified-off.png` 确认关闭时锁为提示色，范围未生效警告可见。

无遗留的 P0/P1/P2 项。

## 五项视觉检查

- **字体**：系统字体与原列表层级一致；代码等宽，正文 15px，信息 11px；
  无 JSON/文本等重复标题。长内容留在上方滚动区，辅助文案可换行。
- **布局**：右侧固定 350px，与历史区域同高；下方 caption 至少 44px。
  上区伸缩，底栏不随短文本或图片位置变化。无横竖分割线，保留留白。
  窄屏为上下布局，预览 310px；document clientWidth=scrollWidth=390，
  工具组 right=351，小开关 right=70，未溢出视口。
- **颜色与材质**：内容使用统一的浅色底；小型操作胶囊轻量承托，不为右侧增加第二层卡片。
  开关开为柔和蓝色、关为灰色，范围未生效为暖色且有文字，不只依赖颜色。
  外缘高光、键帽、焦点轮廓保留。
- **图像与图标**：图片维持 1448:1086 原比例，不裁切；放大入口统一放底部，图片也可点击。
  来源图标沿用真实资源；锁、盾牌和操作均使用图标库，未手绘替代。
- **文案**：纯文本只显示全文和字符数。JSON 原文、格式化草稿、原文对照清楚区分；
  底部明确提示取用或使用结果。范围信息、范围继承与失效警告继续显示。

## 流程与交互检查

1. **选择/浏览 — 通过。** 三类内容均无标题；切换预览不改变当前记录。
   纯文本详情 button 数量为 0，底部只有信息。
2. **JSON 增强 — 通过。** 格式化真实生成草稿，当前记录保持不变；
   对照/取消正常，点击底部 Esc 可取消草稿；确认后新增一条结果并继承应用范围。
3. **图片查看 — 通过。** 底部放大按钮可按 Enter 触发；Esc 退出对话框。
4. **防误贴状态 — 通过。** 点击 switch 立即关闭，aria-checked=false，警告和锁提示同步；
   按 Space 重新开启，aria-checked=true。无需额外状态浮层。
5. **窄屏和键盘 — 通过。** 390px 下信息和按钮均在容器内；开关、操作按钮保留焦点显示。
   已恢复原浏览器 viewport。未声称完成全套辅助技术或原生权限验收。

最终构建通过，浏览器控制台检查无 warn/error；只读代码复核无新增实质问题。
网页只操作示例状态；原生系统剪贴板、真实全局按键和材质没有在本轮重建或重新验证。
