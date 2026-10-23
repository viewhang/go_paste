package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"gopaste/internal/types"
)

// ErrHistoryLimit 表示单条记录的资源已超过自动历史容量上限
var ErrHistoryLimit = errors.New("store: record exceeds history storage limit")

// blobName 按原始字节生成资源文件名以复用相同内容
func blobName(data []byte, ext string) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16]) + "." + strings.TrimPrefix(ext, ".")
}

// cloneRecord 复制记录和文件列表以隔离未提交的修改
func cloneRecord(r *types.Record) *types.Record {
	cp := *r
	cp.Files = slices.Clone(r.Files)
	return &cp
}

// recordHash 根据持久化内容恢复指纹并兼容旧版文字与文件指纹
func recordHash(r *types.Record) string {
	var content string
	switch r.Kind {
	case types.KindImage:
		if r.Image != "" {
			return "image:" + r.Image
		}
	case types.KindText:
		if r.Text != "" || r.HTML != "" {
			content = r.Text + "\x00" + r.HTML
		}
	case types.KindLink:
		content = r.URL
	case types.KindColor:
		content = strings.ToLower(r.Text)
	case types.KindFile:
		paths := make([]string, len(r.Files))
		for i, f := range r.Files {
			paths[i] = f.Path
		}
		content = strings.Join(paths, "\x00")
	}
	if content == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(string(r.Kind) + "\x00" + content + "\x00"))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// references 汇总记录引用的独立原图、预览图与来源图标
func references(records []*types.Record) map[string]int64 {
	refs := make(map[string]int64)
	for _, r := range records {
		for _, name := range []string{r.Image, r.PreviewImage, r.Source.Icon} {
			if name != "" {
				refs[name] = 0
			}
		}
	}
	return refs
}

// blobSizeLocked 读取资源实际占用，待写资源使用原始字节长度
func (s *Store) blobSizeLocked(name string, pending map[string]int64) (int64, error) {
	if n, ok := pending[name]; ok {
		return n, nil
	}
	path := s.BlobPath(name)
	if path == "" {
		return 0, errors.New("store: bad blob name")
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// protectedLocked 汇总普通列表永久保留的资源
func protectedLocked(records []*types.Record) map[string]int64 {
	return references(slices.DeleteFunc(slices.Clone(records), func(r *types.Record) bool { return r.ListID == "" }))
}

// admitLocked 在落盘与清理前拒绝自身已超出历史容量的记录
func (s *Store) admitLocked(rec *types.Record, pending map[string]int64) error {
	if rec.ListID != "" {
		return nil
	}
	protected := protectedLocked(s.records)
	var total int64
	for name := range references([]*types.Record{rec}) {
		if _, ok := protected[name]; ok {
			continue
		}
		n, err := s.blobSizeLocked(name, pending)
		if err != nil {
			return err
		}
		total += n
	}
	if total > int64(s.settings.HistoryLimitMiB)<<20 {
		return ErrHistoryLimit
	}
	return nil
}

// pruneLocked 按条数、期限与独立文件容量保留最新历史并豁免普通列表
func (s *Store) pruneLocked(records []*types.Record) ([]*types.Record, error) {
	used := protectedLocked(records)
	sizes := make(map[string]int64)
	kept := make([]*types.Record, 0, len(records))
	var total int64
	count := 0
	cutoff := time.Now().AddDate(0, 0, -s.settings.RetentionDays)
	for _, r := range records {
		if r.ListID != "" {
			kept = append(kept, r)
			continue
		}
		if count >= s.settings.MaxRecords || s.settings.RetentionDays > 0 && r.CreatedAt.Before(cutoff) {
			continue
		}
		refs := references([]*types.Record{r})
		var extra int64
		for name := range refs {
			if _, ok := used[name]; ok {
				continue
			}
			n, ok := sizes[name]
			if !ok {
				var err error
				n, err = s.blobSizeLocked(name, nil)
				if err != nil {
					return nil, err
				}
				sizes[name] = n
			}
			extra += n
		}
		if total+extra > int64(s.settings.HistoryLimitMiB)<<20 {
			continue
		}
		total += extra
		count++
		kept = append(kept, r)
		for name := range refs {
			used[name] = 0
		}
	}
	return kept, nil
}

// removeUnusedLocked 仅回收候选文件中已经没有记录引用的资源
func (s *Store) removeUnusedLocked(candidates map[string]int64) {
	used := references(s.records)
	for name := range candidates {
		if _, ok := used[name]; ok {
			continue
		}
		if path := s.BlobPath(name); path != "" {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				log.Print("removing unused blob: ", err)
			}
		}
	}
}

// saveBlobLocked 以临时文件原子保存原始资源并复用已有内容
func (s *Store) saveBlobLocked(data []byte, name string) error {
	path := s.BlobPath(name)
	if path == "" {
		return errors.New("store: bad blob name")
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return writeAtomic(path, data)
}

// writeAtomic 将完整文件同步到临时路径后原子替换目标
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}
