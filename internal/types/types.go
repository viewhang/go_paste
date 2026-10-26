// Package types holds the data model shared by the clipboard store, the
// watcher and the frontend.
package types

import (
	"runtime"
	"time"
)

// Kind is the type of a clipboard record.
type Kind string

const (
	KindText  Kind = "text"
	KindImage Kind = "image"
	KindLink  Kind = "link"
	KindColor Kind = "color"
	KindFile  Kind = "file"
)

// Source describes the application a record was copied from.
type Source struct {
	AppName string `json:"appName,omitempty"`
	AppID   string `json:"appId,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

// FileRef is one file of a file record. It references the original file on
// disk; the app never copies the file itself.
type FileRef struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Dir     bool   `json:"dir,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Missing bool   `json:"missing,omitempty"`
}

// Record 保存剪贴板内容、来源与整理信息
type Record struct {
	ID        string    `json:"id"`
	Kind      Kind      `json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
	Source    Source    `json:"source"`

	// Text content, and its rich text form when the source offered HTML.
	Text string `json:"text,omitempty"`
	HTML string `json:"html,omitempty"`

	// Image payload: a file name in the blob directory, its pixel size and
	// format from the encoded bytes.
	Image       string `json:"image,omitempty"`
	ImageW      int    `json:"imageW,omitempty"`
	ImageH      int    `json:"imageH,omitempty"`
	ImageFormat string `json:"imageFormat,omitempty"`
	Animated    bool   `json:"animated,omitempty"`

	// Link fields.
	URL          string `json:"url,omitempty"`
	Title        string `json:"title,omitempty"`
	PreviewImage string `json:"previewImage,omitempty"`
	PreviewState string `json:"previewState,omitempty"` // "", "loading", "done", "failed"

	// Color value in canonical #rrggbb or #rrggbbaa form.
	Color string `json:"color,omitempty"`

	Files []FileRef `json:"files,omitempty"`
	// FileCount 保存摘要对应的完整文件数量
	FileCount int `json:"fileCount,omitempty"`
	// Summary 标识按需加载的卡片摘要
	Summary bool `json:"-"`

	ListID string `json:"listId,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`

	// Hash 保存内容指纹以便重启后继续合并重复记录
	Hash string `json:"hash,omitempty"`
}

// RuleField 标识智能列表的匹配字段
type RuleField string

const (
	// RuleKind 按剪贴板内容类型匹配
	RuleKind RuleField = "kind"
	// RuleExtension 按文件扩展名或图片格式匹配
	RuleExtension RuleField = "extension"
	// RuleKeyword 按记录包含的关键词匹配
	RuleKeyword RuleField = "keyword"
)

// ListRule 保存智能列表的一条匹配规则
type ListRule struct {
	Field RuleField `json:"field"`
	Value string    `json:"value"`
}

// Tab 保存普通列表名称或智能列表的匹配方式与规则
type Tab struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Smart bool       `json:"smart,omitempty"`
	Rules []ListRule `json:"rules,omitempty"`
	// MatchAny 为真时匹配任意规则，否则匹配全部规则
	MatchAny bool `json:"matchAny,omitempty"`
}

// ExcludedApp 保存用户排除的应用标识与显示名称
type ExcludedApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Path 保存用户选择的应用位置以判断应用是否仍安装在本机
	Path string `json:"path,omitempty"`
}

// Settings 保存快捷键、排除规则、应用偏好与两种面板形态的尺寸
type Settings struct {
	Hotkey     string `json:"hotkey"`
	PastePlain bool   `json:"pastePlain"`
	// MoveToFrontOnPaste 控制粘贴后是否将记录移至最前
	MoveToFrontOnPaste bool `json:"moveToFrontOnPaste"`
	// ResetPanelAfterPaste 控制粘贴后是否清空搜索，默认关闭以保留搜索
	ResetPanelAfterPaste bool `json:"resetPanelAfterPaste"`
	LinkPreview          bool `json:"linkPreview"`
	MaxRecords           int  `json:"maxRecords"`
	// HistoryLimitMiB 限制未分组历史引用的独立文件总容量
	HistoryLimitMiB int  `json:"historyLimitMiB"`
	RetentionDays   int  `json:"retentionDays"`
	LaunchAtLogin   bool `json:"launchAtLogin"`
	Paused          bool `json:"paused"`
	// CopySound 控制成功记录复制内容后是否播放系统提示音
	CopySound bool `json:"copySound"`
	// UpdateMirror 保存更新检查与下载使用的 GitHub 镜像前缀，留空时直连
	UpdateMirror string `json:"updateMirror,omitempty"`
	// ExcludedApps 保存用户可增删的默认及自定义排除应用
	ExcludedApps []ExcludedApp `json:"excludedApps"`
	// ExcludedAppsInitialized 标记默认列表已初始化以保留用户的移除结果
	ExcludedAppsInitialized bool `json:"excludedAppsInitialized"`
	// ExcludeConfidential 控制是否忽略应用标记为机密的剪贴板内容
	ExcludeConfidential bool `json:"excludeConfidential"`
	// ExcludeWhitespace 控制是否排除仅包含空白字符与空白行的文本
	ExcludeWhitespace bool `json:"excludeWhitespace"`
	// ImageLimitMiB 限制单张剪贴板图片的编码字节大小，0 表示不限制
	ImageLimitMiB int `json:"imageLimitMiB"`
	// Theme 保存外观主题，0 为经典，1 为 Liquid Glass
	Theme int `json:"theme,omitempty"`
	// Form is the panel's shape: 0 along the bottom of the screen, 1 beside
	// the pointer.
	Form       int   `json:"form"`
	CustomTabs []Tab `json:"customTabs"`
	// BarHeight 保存贴底长条不含预览区的高度
	BarHeight int `json:"barHeight,omitempty"`
	// PanelWidth 保存跟随鼠标面板的宽度
	PanelWidth int `json:"panelWidth,omitempty"`
	// PanelHeight 保存跟随鼠标面板的高度
	PanelHeight int `json:"panelHeight,omitempty"`
	// Shortcuts 保存面板操作的自定义快捷键，未设置的操作使用默认按键
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
}

// DefaultExcludedApps 提供当前平台的常见密码应用默认排除列表
func DefaultExcludedApps() []ExcludedApp {
	switch runtime.GOOS {
	case "darwin":
		return []ExcludedApp{
			{ID: "com.apple.Passwords", Name: "Apple Passwords"},
			{ID: "com.apple.keychainaccess", Name: "Keychain Access"},
			{ID: "com.1password.1password", Name: "1Password"},
			{ID: "com.agilebits.onepassword7", Name: "1Password 7"},
			{ID: "com.bitwarden.desktop", Name: "Bitwarden"},
			{ID: "org.keepassxc.keepassxc", Name: "KeePassXC"},
			{ID: "org.keepassx.keepassxc", Name: "KeePassXC (Legacy)"},
			{ID: "in.sinew.Enpass-Desktop", Name: "Enpass"},
			{ID: "me.proton.pass.electron", Name: "Proton Pass"},
			{ID: "com.nordsec.nordpass", Name: "NordPass"},
			{ID: "com.keepersecurity.passwordmanager", Name: "Keeper"},
		}
	case "windows":
		return []ExcludedApp{
			{ID: "1password.exe", Name: "1Password"},
			{ID: "bitwarden.exe", Name: "Bitwarden"},
			{ID: "keepassxc.exe", Name: "KeePassXC"},
			{ID: "keepass.exe", Name: "KeePass"},
			{ID: "enpass.exe", Name: "Enpass"},
			{ID: "protonpass.exe", Name: "Proton Pass"},
			{ID: "nordpass.exe", Name: "NordPass"},
			{ID: "keeper password manager.exe", Name: "Keeper"},
		}
	}
	return nil
}

// DefaultHotkey 定义 macOS 的 Command+Shift+V 与 Windows 的 Ctrl+Shift+V 默认呼出键
const DefaultHotkey = "CmdOrCtrl+Shift+V"

// DefaultSettings 提供默认应用偏好、排除规则与容量限制
func DefaultSettings() Settings {
	return Settings{
		Hotkey:                  DefaultHotkey,
		CopySound:               true,
		LinkPreview:             true,
		MaxRecords:              600,
		HistoryLimitMiB:         256,
		ExcludedApps:            DefaultExcludedApps(),
		ExcludedAppsInitialized: true,
		ExcludeWhitespace:       true,
		ImageLimitMiB:           5,
	}
}
