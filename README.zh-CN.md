# GoPaste

[English](README.md) | 简体中文

macOS 和 Windows 剪贴板管理器，使用 Go 和 [MyGo](https://github.com/egoist/mygo) 开发。

## 功能

- 记录文本、图片、链接、颜色和文件，显示来源应用与复制时间
- 搜索历史记录，支持普通列表和智能列表
- 预览、拖拽、保留格式粘贴和纯文本粘贴
- 按应用排除记录，支持机密内容、空白内容和图片大小过滤
- 贴边长条与跟随鼠标面板，支持调整大小
- 经典与 Liquid Glass 主题，支持简体中文和英文

## MyGo

本项目使用 [MyGo](https://github.com/egoist/mygo) 的原生界面：

- Go 编写的 GPU 渲染界面，无需 WebView、HTML 或 JavaScript
- 无需 cgo，同一份代码可交叉编译到 macOS 和 Windows
- 提供剪贴板、托盘、全局快捷键、拖拽和签名更新等桌面接口
- 界面可直接测试和渲染为截图，无需打开窗口

## 内存占用

### 220 条记录

2026-10-23 本机实测：GoPaste 0.1.0 arm64，macOS 27.0.1，Apple M1 Pro，32 GB 内存。采样时保存了 220 条记录，其中包含 42 条图片记录。

| 指标 | 物理内存占用 |
| --- | --- |
| 当前采样 | 约 63 MB |
| 本次运行期间的峰值 | 约 122 MB |

使用 `vmmap -summary` 的 Physical footprint 指标。占用会随图片、预览和使用情况变化。

### 2000 条记录

2026-10-23 使用本地开发构建（arm64）实测，环境为 macOS 27.0.1、Apple M1 Pro、32 GB 内存。

测试生成了 2000 条记录：800 条文本、500 条图片、400 条文件、200 条链接和 100 条颜色，包含 4K 图片、单条约 1 MiB 的长文本和单条 600 个文件。

| 场景 | 物理内存占用（三轮中位数） |
| --- | --- |
| 加载全部记录 | 123.4 MiB |
| 打开面板 | 133.2 MiB |
| 滚动至第 2000 条 | 108.0 MiB |
| 贴边连续缩放 | 131.5 MiB |
| 测试后隐藏面板 | 133.0 MiB |
| 每轮运行的峰值 | 199.3 MiB |

每个阶段空闲 3 秒后使用 `vmmap -summary` 采样，不强制触发垃圾回收。三轮峰值范围为 199.0–207.4 MiB。

测试通过独立原生窗口运行存储与界面代码，未启动剪贴板监听、托盘和更新器，因此这些数据不代表完整应用的内存占用。Windows 尚未实测。

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
