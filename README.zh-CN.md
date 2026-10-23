# GoPaste

[English](README.md) | 简体中文

macOS 和 Windows 剪贴板管理器，使用 Go 和 [MyGo](https://github.com/egoist/mygo) 开发。

[运行截图](docs/screenshots.md)

## 功能

- 记录复制过的文本、图片、链接、颜色和文件，保留来源应用和复制时间
- 搜索历史记录，用普通列表整理内容，或用智能列表按规则筛选
- 预览、拖拽记录，粘贴时可保留原格式或转为纯文本
- 自选不记录的应用，也可排除空白内容、带机密标记的内容和过大的图片
- 面板可贴在屏幕底部，或在鼠标旁弹出，大小可调
- 经典和 Liquid Glass 两种主题，可切换中英文

## MyGo

本项目使用 [MyGo](https://github.com/egoist/mygo) 的原生界面：

- Go 编写的 GPU 渲染界面，无需 WebView、HTML 或 JavaScript
- 无需 cgo，同一份代码可交叉编译到 macOS 和 Windows
- 提供剪贴板、托盘、全局快捷键、拖拽和签名更新等桌面接口
- 界面可直接测试和渲染为截图，无需打开窗口

## 内存占用

2026-10-23 使用本地开发构建（arm64）实测，环境为 macOS 27.0.1、Apple M1 Pro、32 GB 内存。

测试使用 2000 条记录：800 条文本、500 条图片、400 条文件、200 条链接和 100 条颜色，包含 4K 图片、单条约 1 MiB 的长文本和单条 600 个文件。

| 场景 | 内存占用 |
| --- | --- |
| 历史加载完成 | 46.2 MiB |
| 打开面板 | 68.1 MiB |
| 滚动至第 2000 条 | 183.8 MiB |
| 贴边连续缩放 | 171.5 MiB |
| 测试后隐藏面板 | 104.8 MiB |
| 每轮运行的峰值 | 202.1 MiB |

数据为三轮测试的中位数。每个阶段空闲 3 秒后使用 `vmmap -summary` 的 Physical footprint 指标采样，不强制触发垃圾回收。

测试通过原生窗口运行存储与界面代码，未启动剪贴板监听、托盘和更新器。Windows 尚未实测。

## 安装与使用

在 [Releases](https://github.com/viewhang/go_paste/releases) 下载：

- macOS：选择 Apple Silicon（arm64）或 Intel（x64）版本，解压后将 `GoPaste.app` 放入应用程序目录
- Windows：运行 x64 Setup 安装包

默认按 `Command+Shift+V`（Windows 为 `Ctrl+Shift+V`）呼出或收起面板。面板形态、快捷键和排除规则可在设置中修改。

macOS 自动粘贴需要在「系统设置 → 隐私与安全性 → 辅助功能」中授权 GoPaste。

| 快捷键 | 操作 |
| --- | --- |
| `⌘⇧V` / `Ctrl+Shift+V` | 呼出或收起面板 |
| `↑` `↓` | 选择记录，贴边模式使用 `←` `→` |
| `Enter` / `Shift+Enter` | 粘贴 / 粘贴为纯文本 |
| `⌘1–0` / `Ctrl+1–0` | 粘贴当前可见项目，最多十项 |
| `Space` | 预览 |
| `⌘F` / `Ctrl+F` | 搜索 |
| `⌘C` / `Ctrl+C` | 复制选中记录 |
| `⌘,` / `Ctrl+,` | 打开设置 |
| `Backspace` | 删除选中记录 |
| `Esc` | 关闭预览、清空搜索或收起面板 |

按住 Command（Windows 为 Ctrl）时显示快速粘贴编号。设置 → 关于中可检查更新、开启自动检查或自动下载安装。

## 开发

需要 Go 1.27.1。

```bash
CGO_ENABLED=0 go run .
CGO_ENABLED=0 go test ./...
```

界面文案位于 `internal/locales/`，修改后运行：

```bash
go tool mygo generate
go tool mygo i18n check
```

## 打包与发布

```bash
# macOS Apple Silicon
go run ./tools/package -arch arm64 -o build/darwin-arm64 -release

# macOS Intel x64
go run ./tools/package -arch amd64 -o build/darwin-amd64 -release

# Windows x64
go tool mygo build -platform windows/amd64 -o build
```

各平台的程序、安装包和更新文件输出到对应的 `build/<平台>-<架构>/` 目录。

发布前修改 `mygo.json` 的 `version`，重新打包。创建标签为 `v<版本号>` 的正式 Release，上传三个目录中的 `.tar.gz`、`update-*.json` 和 Windows Setup 安装包。
