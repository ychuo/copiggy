> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../../development.md) for supported behavior and development commands.

# Copiggy 烟灰玻璃视觉 · 设计验收

日期：2026-10-20。对象：现有网页原型的视觉调整，交互代码未改。

final result: passed

## 参考与比较

- Source visual truth：`qa/smoked-glass/reference.png`，用户提供的 2048 × 1536 截图。
  采用其右侧烟灰半透明材质、边缘高光、浅色字和圆润控件。
  该截图属于不同产品，只参考视觉风格，不复制聊天布局、内容或分割线。
- 实现：`qa/smoked-glass/final.png`，viewport 与输出均为 822 × 898，密度 1。
- 同状态前后对比：`qa/smoked-glass/before-after.png`；两侧相同图片选择、当前文本记录、
  保护开启和面板尺寸，按相同坐标裁切，未缩放字体。
- 已打开并检查的最终参考/实现合成：`qa/smoked-glass/final-style-comparison.png`。
  参考右面板截取 (1060,145)–(1955,1235)，等比缩至宽 440；
  实现截取 (25,98)–(797,592)，等比缩至宽 760。不同产品的长宽比保留，
  用于材质与层级判断，不作为排版尺寸逐像素比较。
- 局部证据：`qa/smoked-glass/material-detail.png`，比较外缘、控件表面与字色。
  两区域均为 380 × 160 原始像素；参考缩放密度未知，不把字体尺寸差异当作偏差。
- 其他状态：`guard-off.png`、`draft.png`、`narrow.png`。
  窄窗口 viewport 与图片均为 390 × 844，已实际查看。

## 发现与修复记录

1. **P2：搜索占位字在比正文更亮的搜索表面上对比不足。**
   首轮占位字采用统一辅助色，截图采样背景为 (106,109,102)，对比约 3.73:1。
   提亮到 #f0f1e9、固定 opacity=1，最终同位置采样约 4.63:1。
   最终 DOM 确认颜色为 rgb(240,241,233)，已重新捕获并查看最终比较图。
2. 原浅白主题中的蓝灰正文、范围信息、状态色、toast、演示控件及减少透明度回退
   均已同步调整，未留下浅底深字的混搭组件。
3. 无未解决的 P0/P1/P2 差异。搜索聚焦时的轮廓比普通胶囊更明显，作为焦点反馈保留。

## 五项视觉检查

- **字体**：保持系统字体、17px 列表与 14px 等宽代码；保留清晰的主次层级。
  原图字号较大是另一产品布局，本轮不放大历史行或改变密度。
- **布局与间距**：面板圆角改为 34px，搜索为胶囊，操作组用圆形按钮与胶囊承托。
  全宽搜索、列表左文右图、右侧上内容下信息、无内部分隔线均保留。
- **颜色与材质**：暖中性灰背景、烟灰透明表面、克制外缘高光；选中行深一阶。
  正文和辅助文字在最终主体背景 (93,95,90) 上的采样对比分别约 6.01:1 与 4.58:1。
  这是当前固定背景的检查，不代表所有原生桌面背景或系统设置均已验收。
- **图像与图标**：真实来源图标和照片沿用原资产，图像不裁切、不染色。
  锁角标使用不透明深底与浅色锁；关闭保护时采用暖色提示。操作仍用现有图标库。
- **文案**：未增加品牌标题、类型标题或新的操作说明；普通文本无处理控件。
  原文/草稿、范围继承和保护失效的信息保持原义，警告文字有足够辨识度。

## 浏览器与构建检查

- 实测图片/JSON 选择、直接开关、防误贴关闭警告、Space 重新开启、
  Enter 触发格式化、生成草稿、放弃草稿及返回图片状态。
- 当前记录未因浏览或格式化草稿改变。
- 窄屏 document clientWidth=scrollWidth=390，底部控件边界为 11–379，无横向溢出。
  已恢复默认 viewport，并保留最终预览页。
- 浏览器 warn/error 为空；最终 npm run build 通过。
- 独立视觉复核没有阻塞意见。减少透明度样式已在代码中调整为同色系不透明底，
  未更改用户系统设置进行实测。

本轮为参考图风格适配。网页效果仍是玻璃材质近似，未重建原生 MyGo 应用。
