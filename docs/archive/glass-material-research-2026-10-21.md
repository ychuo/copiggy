> **Historical record — archived 2026-10-21.** This document describes an earlier prototype or verification session, not the current implementation or current test results. Experiment commands, paths and generated evidence may no longer exist. See the [current documentation](../development.md) for supported behavior and development commands.

# macOS 玻璃材质与 MyGo Native UI 调研

日期：2026-10-21

本轮仅调研。没有修改应用代码、构建或运行应用，也没有调整辅助功能权限。以下“已证实”指官方接口声明或当前锁定依赖的源码证据；未进行新方案的运行时验证。本文是调研记录。

## 结论

实现苹果系统玻璃材质不要求把应用迁移到 SwiftUI。AppKit 提供 `NSVisualEffectView`、`NSPopover`，macOS 26 起还提供 `NSGlassEffectView` 和 `NSGlassEffectContainerView`。MyGo 使用原生 macOS 窗口，也有调用 AppKit 的桥接能力。

但“使用 MyGo Native UI”与“每个控件都是 AppKit 系统控件”并不是同一件事。当前卡片、文字、图标和操作控件是 MyGo 在原生绘图表面中绘制的。系统材质可与它组合，不能假定 MyGo 自绘控件会自动获得系统玻璃的所有视觉和自适应行为。

截图只能说明目标外观，不能证明 Paste 使用了什么框架，也不能仅凭静态截图确定它使用传统半透明材质还是新一代 Liquid Glass。

官方资料：

- [Adopting Liquid Glass](https://developer.apple.com/documentation/technologyoverviews/adopting-liquid-glass)：SwiftUI、UIKit、AppKit 都支持采用系统新设计；自定义背景及辅助功能设置会影响最终外观。
- [NSVisualEffectView](https://developer.apple.com/documentation/appkit/nsvisualeffectview)：传统模糊和 vibrancy；Behind-window 模式用于窗口后方的内容。
- [NSGlassEffectView](https://developer.apple.com/documentation/appkit/nsglasseffectview)：AppKit 的动态玻璃效果容器。
- [Build an AppKit app with the new design](https://developer.apple.com/videos/play/wwdc2025/310/)：AppKit 采用 Liquid Glass 的官方讲解，包括 contentView 的使用要求。

## 已证实的当前架构

以下 MyGo 路径相对于当前依赖：

`.cache/go/pkg/mod/github.com/egoist/mygo@v0.5.6-0.20261020063447-d5bcfdb3d75b`

| 项目 | 证据 | 含义 |
| --- | --- | --- |
| Copiggy 原生内容入口 | `experiments/mygo-clipboard/main.go:133–140`：`Content: ui.View(s.nativeView)`，透明窗口 | 当前产品面板不是 WebView |
| MyGo 内容自行绘制 | MyGo `content.go:17–24` | Native UI 是 MyGo 绘制的 UI，不是为每个元素实例化系统控件 |
| 原生承载视图 | MyGo `internal/darwin/surface.go:152–174,739`：创建 `MyGoSurfaceView`，它继承 `NSView` | 原生窗口、输入和无障碍桥接承载自绘内容 |
| macOS 绘图 | MyGo `docs/architecture.md:4346–4349` | 图层支持的 NSView；Metal 渲染到 CAMetalLayer |
| 当前 Copiggy 材质 | `experiments/mygo-clipboard/internal/macos/glass_darwin.go:281–294` | 创建 `NSVisualEffectView`，BehindWindow、Active、Popover material；作为 MyGoSurfaceView 后方的 sibling |
| 当前预览的箭头 | 同上 `:302–310,346–378`；`native_view.go:96–107,596–601` | 本体和箭头采用连续路径；系统模糊遮罩加 MyGo 自绘填充与描边 |
| 当前预览不是 NSPopover | `native_view.go:576–601`；`native_actions.go:117–127` | 预览仍在同一 MyGo 内容表面内绘制，另同步一块材质区域 |

因此目前的效果是“系统传统背景模糊 + 自定义轮廓、颜色与描边”，不是已经在使用 `NSGlassEffectView`。

## 已有的 MyGo 系统 Popover 路线

当前锁定的 MyGo 版本已经提供：

```go
mygo.NewFlyout(mygo.FlyoutOptions{
    Parent:    parent,
    Anchor:    selectedCardRectangle,
    Placement: mygo.PlacementTop,
    Popover:   true,
    Content:   ui.View(previewView),
})
```

这里只展示现有 API 用法，没有把代码接入应用。

已证实：

- MyGo `flyout.go:108–119` 明确说明：`Popover:true` 在 macOS 使用 `NSPopover`，由系统负责材质、圆角、阴影、指向锚点的箭头以及开关动画。
- `flyout.go:135–137` 接受原生 `Content`；`examples/flyout/main.go:173–194` 是完整的 `Popover:true` 与 `Content:ui.View(...)` 组合示例，内容根背景透明。
- `internal/darwin/popover.go:27–42` 实际创建 `NSViewController` 和 `NSPopover`，并设置 contentViewController。
- `internal/darwin/window.go:298–311` 根据原生 Surface 标记创建 MyGoSurfaceView，不走 WebView。
- `flyout.go:64–73` 的 Anchor 是父窗口内容区坐标中的矩形，可以直接使用所选方卡的 bounds；`:201–210` 提供 `SetAnchor` 更新锚点。
- `internal/e2e/flyout_test.go:297–301` 有原生 UI Popover 与 Escape 关闭的测试代码。本轮只读，未运行此测试。

边界与需要验证的部分：

- 系统负责实际位置。`flyout.go:111–114` 说明指定方向空间不足会翻转；其余 start/end 对齐与 Gap 对 NSPopover 不生效。因此不能承诺保持自定义的准确 6px 距离或箭头尺寸。
- Popover 会获得键盘。`internal/darwin/popover.go:15–20` 描述父窗口与 Popover 的焦点行为；`:128–139` 的 Hide 会尝试把键盘交还可见父窗口。
- `Hide` 后可以重新 `Show`；系统点击关闭会走生命周期清理，应用应正确处理 `OnClosed` 后重新创建或选择合适的 `OnClose` 行为。证据见 `internal/e2e/flyout_test.go:145–150` 和 `internal/darwin/popover.go:179–191`。
- Copiggy 目前 `main.go:150–157` 在面板失焦时关闭面板，并修改防误贴的面板状态。改成独立 Popover 时，必须验证父子窗口焦点转换、Escape、左右选择、回车取用、点击外部和防误贴状态，避免子浮层打开时父面板被误收起。
- 上述 API 与实现已确认存在，但它们在当前 Copiggy 非激活面板中的最终外观与交互尚未实测。

## AppKit Liquid Glass 路线

本机 SDK 头文件：

`/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/System/Library/Frameworks/AppKit.framework/Headers/NSGlassEffectView.h`

已证实：

- `:19,23`：`NSGlassEffectView` 及样式从 macOS 26.0 可用。
- `:24`：它是 AppKit 的 `NSView` 子类，不要求 SwiftUI。
- `:28,31,34,37`：公开属性包含 contentView、cornerRadius、tintColor、style。
- `:27` 明确要求：只有 contentView 保证放置在玻璃效果内，任意子视图与材质及内容之间的层级关系不保证。
- 同一个头文件 `:43–56` 定义 `NSGlassEffectContainerView`，通过 contentView 和 spacing 对符合条件的玻璃子视图分组、批处理与合并。

Apple 在 [Build an AppKit app with the new design](https://developer.apple.com/videos/play/wwdc2025/310/) 中也说明应使用 contentView 包裹内容，而不是仅将效果视图放在内容后面；这样系统才能正确处理内容与效果的适应关系。

对当前工程的推断：

1. 通过现有 purego/Objective-C 调用能力接入 NSGlassEffectView 在架构上可行，但还没有运行验证。
2. 不能把当前 `NSVisualEffectView` 的类名直接替换成 `NSGlassEffectView` 就宣称完成。当前 bridge 是 sibling 背景；新效果应遵守 contentView 包含关系。
3. 当前托盘和预览都画在一个 MyGoSurfaceView 内。如果需要不同区域各自拥有系统玻璃，需要研究内容表面的拆分/承载层次、透明区域、命中测试与生命周期。
4. 固定颜色的自绘文字和图标不应直接假定能获得与系统控件相同的自动颜色适应；应验证不同壁纸、深浅模式、增加对比度和减少透明度设置。
5. 公开接口有圆角参数，但未证实存在任意尖角轮廓接口。对 NSGlassEffectView 再加自定义 mask，不等于已保证沿该 mask 形成完整的系统折射边缘。需要带尖角的系统浮层时，应优先评估 NSPopover。
6. 若要支持 macOS 26 以前，需要运行时能力判断并保留 NSVisualEffectView 等回退方式。

## MyGo 的 glass 插件不等于系统玻璃视图

当前版本还包含 `github.com/egoist/mygo/plugins/glass`。

- `plugins/glass/glass.go:1–6` 说明它由 MyGo 自己的 GPU/CPU shader 绘制，跨平台重现玻璃外观。
- `docs/plugins/glass.md:21–25` 说明它采样的是同一个窗口内已经绘制在其下方的内容。
- `docs/plugins/glass.md:190–191` 说明其表面为圆角矩形，不能像系统 NSGlassEffectContainerView 一样自动合并。

所以它可以用于应用内部内容上的玻璃处理，但不能简单等同于系统材质，也不能据此宣称可直接折射和模糊桌面上其他窗口。当前依赖中插件对系统效果的视觉还原程度属于框架作者描述，本轮没有测量验证。

## 建议的最小后续验证顺序

1. 保留 MyGo、剪贴板模型、防误贴与增强逻辑。
2. 先单独验证 `NewFlyout + Popover:true + ui.View` 的预览外壳：由系统负责尖角和材质，内容仍由 MyGo Native UI 构建。
3. 如果托盘也需要 macOS 26 Liquid Glass 的动态边缘和适应效果，再验证 NSGlassEffectView 正确包裹原生内容表面的方案。
4. 对比传统 NSVisualEffectView、系统 NSPopover、NSGlassEffectView 的实际结果后再确定实现，不因“玻璃”而直接重写为 SwiftUI。

以上仅是待验证路线，没有在本轮执行。
