// Package store 使用 SQLite 保存历史与设置，使用独立文件保存图片
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopaste/internal/types"

	"github.com/egoist/mygo/plugins/sqlite"
)

const (
	// indexFile 标识待迁移的旧版历史文件
	indexFile = "records.json"
	// databaseFile 标识历史与设置共用的数据库
	databaseFile = "history.db"
	// legacySettingsFile 标识待迁移的旧版设置文件
	legacySettingsFile = "settings.json"
)

// Store 串行管理历史索引、资源文件与偏好设置
type Store struct {
	mu      sync.Mutex
	dir     string
	blobDir string
	// records 仅缓存清理和去重需要的元数据，不保留正文与文件列表
	records   []*types.Record
	db        *sqlite.DB
	positions map[string]int64
	byID      map[string]*types.Record
	settings  types.Settings

	closed bool
}

// index 保存历史文件版本与有序记录
type index struct {
	Version int             `json:"version"`
	Records []*types.Record `json:"records"`
}

// Open 加载并迁移历史与设置，应用清理规则后回收孤立资源
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, blobDir: filepath.Join(dir, "blobs"), byID: map[string]*types.Record{}}
	if err := os.MkdirAll(s.blobDir, 0o755); err != nil {
		return nil, err
	}
	s.settings = types.DefaultSettings()
	if err := s.openDatabase(); err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			s.db.Close()
		}
	}()
	kept, err := s.pruneLocked(s.records)
	if err != nil {
		return nil, err
	}
	if err := s.commitLocked(kept, references(s.records)); err != nil {
		return nil, err
	}
	// 清理启动前遗留的无引用文件，此时还没有采集或预览任务
	s.CollectGarbage()

	success = true
	return s, nil
}

// Dir 返回存储目录
func (s *Store) Dir() string { return s.dir }

// BlobDir 返回资源文件目录
func (s *Store) BlobDir() string { return s.blobDir }

// Settings 返回当前设置、排除应用及列表规则的副本
func (s *Store) Settings() types.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.settings
	cp.ExcludedApps = slices.Clone(s.settings.ExcludedApps)
	cp.CustomTabs = slices.Clone(s.settings.CustomTabs)
	for i := range cp.CustomTabs {
		cp.CustomTabs[i].Rules = slices.Clone(cp.CustomTabs[i].Rules)
	}
	return cp
}

// normalizeSettings 补齐基本偏好并为旧设置一次性加入默认排除应用
func normalizeSettings(st *types.Settings) {
	defaults := types.DefaultSettings()
	if st.MaxRecords <= 0 {
		st.MaxRecords = defaults.MaxRecords
	}
	if st.HistoryLimitMiB <= 0 {
		st.HistoryLimitMiB = defaults.HistoryLimitMiB
	}
	if strings.TrimSpace(st.Hotkey) == "" {
		st.Hotkey = defaults.Hotkey
	}
	if !st.ExcludedAppsInitialized {
		for _, app := range defaults.ExcludedApps {
			if !slices.ContainsFunc(st.ExcludedApps, func(existing types.ExcludedApp) bool { return strings.EqualFold(existing.ID, app.ID) }) {
				st.ExcludedApps = append(st.ExcludedApps, app)
			}
		}
		st.ExcludedAppsInitialized = true
	}
}

// SetSettings 在同一事务中保存设置与历史清理结果，失败时保留原设置
func (s *Store) SetSettings(st types.Settings) error {
	normalizeSettings(&st)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fs.ErrClosed
	}
	statement, err := settingsStatement(st)
	if err != nil {
		return err
	}
	prev := s.settings
	if st.MaxRecords == prev.MaxRecords && st.RetentionDays == prev.RetentionDays && st.HistoryLimitMiB == prev.HistoryLimitMiB {
		if _, err := s.db.Execute(context.Background(), statement.SQL, statement.Args...); err != nil {
			return err
		}
		s.settings = st
		return nil
	}
	s.settings = st
	kept, err := s.pruneLocked(s.records)
	if err == nil {
		err = s.commitLocked(kept, references(s.records), statement)
	}
	if err != nil {
		s.settings = prev
	}
	return err
}

// List 按顺序读取完整历史快照
func (s *Store) List() []*types.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readRecordsLocked("payload", "ORDER BY position DESC")
	if err != nil {
		log.Print("listing records: ", err)
	}
	return rows
}

// Get 按标识从数据库读取完整记录
func (s *Store) Get(id string) (*types.Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getLocked(id)
	if err != nil {
		log.Print("reading record: ", err)
	}
	return r, r != nil && err == nil
}

// getLocked 在存储锁内读取完整内容
func (s *Store) getLocked(id string) (*types.Record, error) {
	if s.closed {
		return nil, fs.ErrClosed
	}
	rows, err := s.readRecordsLocked("payload", "WHERE id = ?", id)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// Add 保存已准备好资源的记录并报告是否合并重复内容
func (s *Store) Add(r *types.Record) (*types.Record, bool) {
	rec, merged, err := s.Capture(r, nil, nil)
	if err != nil {
		log.Print("saving record: ", err)
	}
	return rec, merged
}

// Capture 在同一次提交中写入原图、来源图标与记录并应用历史容量限制
func (s *Store) Capture(r *types.Record, imageData, iconData []byte) (*types.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, fs.ErrClosed
	}
	rec := cloneRecord(r)
	if len(imageData) > 0 {
		rec.Image = blobName(imageData, "png")
	}
	if len(iconData) > 0 {
		rec.Source.Icon = blobName(iconData, "png")
	}
	if rec.Hash == "" {
		rec.Hash = recordHash(rec)
	}
	if rec.ID == "" {
		rec.ID = NewID()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	merged := false
	next := make([]*types.Record, 0, len(s.records)+1)
	for _, old := range s.records {
		if !merged && (rec.Hash != "" && rec.Hash == old.Hash || rec.Kind == types.KindImage && rec.Image != "" && rec.Image == old.Image) {
			merged = true
			old, err := s.getLocked(old.ID)
			if err != nil {
				return nil, false, err
			}
			rec.ID, rec.ListID = old.ID, old.ListID
			if rec.Title == "" {
				rec.Title = old.Title
			}
			if rec.PreviewImage == "" {
				rec.PreviewImage = old.PreviewImage
			}
			if rec.PreviewState == "" {
				rec.PreviewState = old.PreviewState
			}
			continue
		}
		next = append(next, old)
	}
	pending := map[string]int64{}
	if len(imageData) > 0 {
		pending[rec.Image] = int64(len(imageData))
	}
	if len(iconData) > 0 {
		pending[rec.Source.Icon] = int64(len(iconData))
	}
	if err := s.admitLocked(rec, pending); err != nil {
		return nil, false, err
	}
	for _, blob := range []struct {
		data []byte
		name string
	}{{imageData, rec.Image}, {iconData, rec.Source.Icon}} {
		if len(blob.data) > 0 {
			if err := s.saveBlobLocked(blob.data, blob.name); err != nil {
				s.removeUnusedLocked(pending)
				return nil, false, err
			}
		}
	}
	next = append([]*types.Record{rec}, next...)
	kept, err := s.pruneLocked(next)
	if err == nil {
		err = s.commitLocked(kept, references(next))
	}
	if err != nil {
		s.removeUnusedLocked(pending)
		return nil, false, err
	}
	return rec, merged, nil
}

// MoveToFront 将已有记录移至最前并持久化记录顺序
func (s *Store) MoveToFront(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fs.ErrClosed
	}
	i := slices.IndexFunc(s.records, func(r *types.Record) bool { return r.ID == id })
	if i <= 0 {
		return nil
	}
	next := slices.Clone(s.records)
	copy(next[1:i+1], next[:i])
	next[0] = s.records[i]
	return s.commitLocked(next, nil)
}

// Update 复制并修改指定记录，保存成功后再回收失去引用的文件
func (s *Store) Update(id string, fn func(*types.Record)) (*types.Record, bool) {
	r, ok, err := s.UpdatePreview(id, nil, "", fn)
	if err != nil {
		log.Print("updating record: ", err)
	}
	return r, ok
}

// UpdatePreview 原子提交记录修改与预览图片，拒绝已删除或关闭后的提交
func (s *Store) UpdatePreview(id string, data []byte, ext string, fn func(*types.Record)) (*types.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, fs.ErrClosed
	}
	old, err := s.getLocked(id)
	if err != nil {
		return nil, false, err
	}
	if old == nil {
		return nil, false, nil
	}
	rec := cloneRecord(old)
	fn(rec)
	pending := map[string]int64{}
	if len(data) > 0 {
		rec.PreviewImage = blobName(data, ext)
		pending[rec.PreviewImage] = int64(len(data))
		if err := s.admitLocked(rec, pending); err != nil {
			return nil, false, err
		}
		if err := s.saveBlobLocked(data, rec.PreviewImage); err != nil {
			return nil, false, err
		}
	}
	next := slices.Clone(s.records)
	for i, r := range next {
		if r.ID == id {
			next[i] = rec
			break
		}
	}
	kept, err := s.pruneLocked(next)
	if err == nil {
		err = s.commitLocked(kept, references(next))
	}
	if err != nil {
		s.removeUnusedLocked(pending)
		return nil, false, err
	}
	return rec, true, nil
}

// Delete 保存记录删除结果后回收不再使用的文件
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.byID[id] == nil {
		return false
	}
	next := slices.DeleteFunc(slices.Clone(s.records), func(r *types.Record) bool { return r.ID == id })
	if err := s.commitLocked(next, nil); err != nil {
		log.Print("deleting record: ", err)
		return false
	}
	return true
}

// Clear 保存清空结果后回收全部已失去引用的文件
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fs.ErrClosed
	}
	return s.commitLocked(nil, nil)
}

// SaveBlob 原子写入独立资源并返回内容寻址的文件名
func (s *Store) SaveBlob(data []byte, ext string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", fs.ErrClosed
	}
	name := blobName(data, ext)
	return name, s.saveBlobLocked(data, name)
}

// BlobPath 返回合法资源文件的绝对路径
func (s *Store) BlobPath(name string) string {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return filepath.Join(s.blobDir, name)
}

// ReadBlob 读取资源原始字节
func (s *Store) ReadBlob(name string) ([]byte, error) {
	p := s.BlobPath(name)
	if p == "" {
		return nil, errors.New("store: bad blob name")
	}
	return os.ReadFile(p)
}

// CollectGarbage 在存储锁内清理无记录引用的资源与写入临时文件
func (s *Store) CollectGarbage() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0
	}
	used := references(s.records)

	entries, err := os.ReadDir(s.blobDir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		_, referenced := used[e.Name()]
		if e.IsDir() || referenced {
			continue
		}
		if err := os.Remove(s.BlobPath(e.Name())); err == nil {
			n++
		}
	}
	return n
}

// Close 关闭数据库并拒绝后续资源提交
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// NewID 生成记录的随机标识
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}
