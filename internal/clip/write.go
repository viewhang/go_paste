package clip

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// ownClipboardFormat 标识 GoPaste 主动写入的剪贴板内容
const ownClipboardFormat transfer.Format = "application/x-gopaste-origin"

// Write 为本次复制添加来源标记并写入系统剪贴板
func Write(data transfer.Data) error {
	return mygo.Clipboard.Write(markOwnClipboard(data))
}

// markOwnClipboard 在首个项目中添加标记并保留所有原始格式与项目
func markOwnClipboard(data transfer.Data) transfer.Data {
	items := data.Items()
	if len(items) == 0 {
		return data
	}
	item := items[0]
	reps := []transfer.Representation{transfer.Bytes(ownClipboardFormat, []byte("GoPaste"))}
	for _, format := range item.Formats() {
		reps = append(reps, transfer.Lazy(format, func() ([]byte, error) { return item.Read(format) }))
	}
	items[0] = transfer.NewItem(reps...)
	return transfer.New(items...)
}
