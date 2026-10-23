package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gopaste/internal/types"
)

// storageTest 创建独立临时存储并设置测试用历史容量
func storageTest(t *testing.T, limit int) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	settings := s.Settings()
	settings.HistoryLimitMiB = limit
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	return s
}

// captureTest 提交测试记录及原始资源并检查保存结果
func captureTest(t *testing.T, s *Store, r *types.Record, image, icon []byte) *types.Record {
	t.Helper()
	saved, _, err := s.Capture(r, image, icon)
	if err != nil || saved == nil {
		t.Fatalf("保存失败：%v", err)
	}
	return saved
}

// TestStorageDefaultMigration 验证旧设置保留已有偏好并补齐容量与内容排除规则
func TestStorageDefaultMigration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(`{"hotkey":"Alt+V","maxRecords":700,"retentionDays":30,"showSource":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	settings := s.Settings()
	if settings.HistoryLimitMiB != 256 || settings.MaxRecords != 700 || settings.RetentionDays != 30 {
		t.Fatalf("旧设置迁移不正确：%+v", settings)
	}
	if !settings.ExcludeWhitespace || settings.ImageLimitMiB != 5 {
		t.Fatalf("旧设置未补齐默认内容排除规则：%+v", settings)
	}
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.Settings(); got.HistoryLimitMiB != 256 || got.MaxRecords != 700 || got.RetentionDays != 30 {
		t.Fatalf("旧设置重新保存后不兼容：%+v", got)
	}
	if got := reopened.Settings(); !got.ExcludeWhitespace || got.ImageLimitMiB != 5 {
		t.Fatalf("默认内容排除规则重新保存后丢失：%+v", got)
	}
}

// TestStorageQuotaReferences 验证共享文件只计一次、预览与图标计入容量且普通列表与外部文件大小豁免
func TestStorageQuotaReferences(t *testing.T) {
	s := storageTest(t, 1)
	protected := captureTest(t, s, &types.Record{ID: "saved", Kind: types.KindImage, ListID: "work"}, bytes.Repeat([]byte{1}, 2<<20), nil)
	icon := bytes.Repeat([]byte{2}, 128<<10)
	preview := bytes.Repeat([]byte{3}, 512<<10)
	one := captureTest(t, s, &types.Record{ID: "one", Kind: types.KindLink, URL: "https://one.example"}, nil, icon)
	if _, _, err := s.UpdatePreview(one.ID, preview, "png", func(r *types.Record) { r.PreviewState = "done" }); err != nil {
		t.Fatal(err)
	}
	one, _ = s.Get(one.ID)
	captureTest(t, s, &types.Record{ID: "two", Kind: types.KindLink, URL: "https://two.example", PreviewImage: one.PreviewImage}, nil, icon)
	captureTest(t, s, &types.Record{ID: "file", Kind: types.KindFile, Files: []types.FileRef{{Path: "/external/large", Size: 1 << 40}}, Bytes: 1 << 40}, nil, icon)
	captureTest(t, s, &types.Record{ID: "shared-saved", Kind: types.KindLink, URL: "https://saved.example", PreviewImage: protected.Image}, nil, nil)
	if len(s.List()) != 5 {
		t.Fatal("共享资源被重复计费或普通列表占用了容量")
	}
	captureTest(t, s, &types.Record{ID: "new", Kind: types.KindImage}, bytes.Repeat([]byte{4}, 700<<10), nil)
	for _, id := range []string{"saved", "new", "file", "shared-saved"} {
		if _, ok := s.Get(id); !ok {
			t.Fatalf("不应清理记录 %s", id)
		}
	}
	for _, id := range []string{"one", "two"} {
		if _, ok := s.Get(id); ok {
			t.Fatalf("超出容量的旧记录 %s 未清理", id)
		}
	}
	if _, err := os.Stat(s.BlobPath(one.PreviewImage)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("预览图失去最后引用后未回收")
	}
	if _, err := s.ReadBlob(one.Source.Icon); err != nil {
		t.Fatal("仍被文件记录引用的来源图标丢失")
	}
}

// TestStorageOversizeRejection 验证超大新图片拒绝落盘且不修改已有历史
func TestStorageOversizeRejection(t *testing.T) {
	s := storageTest(t, 1)
	old := captureTest(t, s, &types.Record{Kind: types.KindImage}, []byte("原始图片"), nil)
	before, _ := os.ReadFile(filepath.Join(s.Dir(), indexFile))
	large := bytes.Repeat([]byte{5}, (1<<20)+1)
	if _, _, err := s.Capture(&types.Record{Kind: types.KindImage}, large, []byte("new icon")); !errors.Is(err, ErrHistoryLimit) {
		t.Fatalf("未拒绝超大图片：%v", err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Dir(), indexFile))
	entries, _ := os.ReadDir(s.BlobDir())
	if !bytes.Equal(before, after) || len(entries) != 1 {
		t.Fatal("拒绝超大图片时修改了历史或留下文件")
	}
	if data, err := s.ReadBlob(old.Image); err != nil || string(data) != "原始图片" {
		t.Fatal("超大图片挤掉了旧历史")
	}
}

// TestStorageSavedQuotaExemption 验证缩小容量与重启均保留普通列表，移出后立即恢复容量清理
func TestStorageSavedQuotaExemption(t *testing.T) {
	s := storageTest(t, 4)
	r := captureTest(t, s, &types.Record{ID: "saved", Kind: types.KindImage, ListID: "work", CreatedAt: time.Now().AddDate(0, 0, -60)}, bytes.Repeat([]byte{6}, 2<<20), nil)
	settings := s.Settings()
	settings.HistoryLimitMiB, settings.MaxRecords, settings.RetentionDays = 1, 1, 1
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, ok := reopened.Get(r.ID); !ok {
		t.Fatal("普通列表记录被清理")
	}
	if _, ok := reopened.Update(r.ID, func(r *types.Record) { r.ListID = "" }); !ok {
		t.Fatal("移出普通列表失败")
	}
	if len(reopened.List()) != 0 {
		t.Fatal("移出普通列表后未清理超量或过期记录")
	}
	if _, err := os.Stat(reopened.BlobPath(r.Image)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("移出普通列表后的超量资源未回收")
	}
}

// TestStorageLegacyDedup 验证旧图片指纹迁移保留列表与标识且启动不合并已有分组记录
func TestStorageLegacyDedup(t *testing.T) {
	s := storageTest(t, 1)
	data := []byte("保持原始图片字节与透明度")
	name, err := s.SaveBlob(data, "png")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(s.Dir(), indexFile), index{Version: 1, Records: []*types.Record{
		{ID: "work", Kind: types.KindImage, Image: name, ListID: "work"},
		{ID: "archive", Kind: types.KindImage, Image: name, ListID: "archive"},
	}}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.List()) != 2 {
		t.Fatal("启动合并了用户已经整理的记录")
	}
	r, merged, err := reopened.Capture(&types.Record{Kind: types.KindImage}, data, nil)
	if err != nil || !merged || r.ID != "work" || r.ListID != "work" || len(reopened.List()) != 2 {
		t.Fatalf("旧图片合并丢失整理信息：%+v %v", r, err)
	}
	persisted, _ := os.ReadFile(filepath.Join(s.Dir(), indexFile))
	if !bytes.Contains(persisted, []byte(`"hash"`)) {
		t.Fatal("指纹没有持久化")
	}
	if raw, err := reopened.ReadBlob(name); err != nil || !bytes.Equal(raw, data) {
		t.Fatal("保存过程修改了原始图片")
	}
}

// TestStorageRestartDedup 验证各类内容重启后仍合并到原记录且保留整理信息
func TestStorageRestartDedup(t *testing.T) {
	s := storageTest(t, 1)
	for _, r := range []*types.Record{
		{Kind: types.KindText, Text: "内容", HTML: "<p>内容</p>"},
		{Kind: types.KindLink, URL: "https://example.test/"},
		{Kind: types.KindColor, Text: "#FF0000", Color: "#ff0000"},
		{Kind: types.KindFile, Files: []types.FileRef{{Path: "/one"}, {Path: "/two"}}},
		{Kind: types.KindImage, Image: "missing.png"},
	} {
		r.ListID = "work"
		captureTest(t, s, r, nil, nil)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, old := range reopened.List() {
		r := cloneRecord(old)
		r.ID, r.Hash, r.ListID = "", "", ""
		saved, merged, err := reopened.Capture(r, nil, nil)
		if err != nil || !merged || saved.ID != old.ID || saved.ListID != "work" {
			t.Fatalf("重启后 %s 去重失败：%+v %v", old.Kind, saved, err)
		}
	}
	if len(reopened.List()) != 5 {
		t.Fatal("重启后重复复制增加了记录")
	}
}

// TestStorageCommitFailure 验证索引写入失败时不丢旧引用并清除未提交的新文件
func TestStorageCommitFailure(t *testing.T) {
	s := storageTest(t, 1)
	r := captureTest(t, s, &types.Record{Kind: types.KindImage, Source: types.Source{AppName: "旧来源"}}, []byte("old image"), []byte("old icon"))
	indexPath := filepath.Join(s.Dir(), indexFile)
	before, _ := os.ReadFile(indexPath)
	if err := os.Mkdir(indexPath+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if s.Delete(r.ID) {
		t.Fatal("索引失败时报告删除成功")
	}
	if err := s.Clear(); err == nil {
		t.Fatal("索引失败时报告清空成功")
	}
	if _, _, err := s.Capture(&types.Record{Kind: types.KindImage}, []byte("new image"), []byte("new icon")); err == nil {
		t.Fatal("索引失败时报告采集成功")
	}
	if _, _, err := s.UpdatePreview(r.ID, []byte("new preview"), "png", func(r *types.Record) {
		r.Source.AppName = "新来源"
		r.Source.Icon = ""
	}); err == nil {
		t.Fatal("索引失败时报告更新成功")
	}
	after, _ := os.ReadFile(indexPath)
	entries, _ := os.ReadDir(s.BlobDir())
	saved, ok := s.Get(r.ID)
	if !bytes.Equal(before, after) || len(entries) != 2 || !ok || saved.Source.AppName != "旧来源" || saved.Source.Icon != r.Source.Icon {
		t.Fatal("索引失败损坏了内存、资源或原索引")
	}
	if err := os.Remove(indexPath + ".tmp"); err != nil {
		t.Fatal(err)
	}
}

// TestStorageLatePreviewAndIcons 验证删除与关闭后的预览不落盘，缓存图标在失去引用后可重新保存
func TestStorageLatePreviewAndIcons(t *testing.T) {
	s := storageTest(t, 1)
	icon := []byte("cached source icon")
	r := captureTest(t, s, &types.Record{Kind: types.KindLink, URL: "https://one.example"}, nil, icon)
	if !s.Delete(r.ID) {
		t.Fatal("删除失败")
	}
	if _, ok, err := s.UpdatePreview(r.ID, []byte("late preview"), "png", func(*types.Record) {}); err != nil || ok {
		t.Fatal("已删除记录仍接受预览")
	}
	entries, _ := os.ReadDir(s.BlobDir())
	if len(entries) != 0 {
		t.Fatal("删除留下来源图标或异步预览")
	}
	r = captureTest(t, s, &types.Record{Kind: types.KindLink, URL: "https://two.example"}, nil, icon)
	if _, err := s.ReadBlob(r.Source.Icon); err != nil {
		t.Fatal("缓存图标被回收后未重新保存")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpdatePreview(r.ID, []byte("closed preview"), "png", func(*types.Record) {}); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("关闭后仍接受预览")
	}
	if _, _, err := s.Capture(&types.Record{Kind: types.KindImage}, []byte("closed image"), nil); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("关闭后仍接受采集")
	}
	entries, _ = os.ReadDir(s.BlobDir())
	if len(entries) != 1 {
		t.Fatal("关闭后仍有新资源落盘")
	}
}

// TestStorageCorruptIndex 验证损坏索引阻止启动回收而成功启动清理孤立资源
func TestStorageCorruptIndex(t *testing.T) {
	s := storageTest(t, 1)
	orphan, err := s.SaveBlob([]byte("orphan"), "png")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{`{"records":`, `{}`, `null`} {
		if err := os.WriteFile(filepath.Join(s.Dir(), indexFile), []byte(invalid), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(s.Dir()); err == nil {
			t.Fatal("损坏索引被当作空历史")
		}
		if _, err := s.ReadBlob(orphan); err != nil {
			t.Fatal("索引加载失败时清理了资源")
		}
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.ReadBlob(orphan); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("启动未回收孤立资源")
	}
}

// TestStorageConcurrentCommits 验证并发保存、更新和刷新后索引完整且引用文件均存在
func TestStorageConcurrentCommits(t *testing.T) {
	s := storageTest(t, 1)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			r, _, err := s.Capture(&types.Record{Kind: types.KindImage}, []byte(fmt.Sprint(i)), []byte("shared icon"))
			if err != nil {
				t.Error(err)
				return
			}
			s.Update(r.ID, func(r *types.Record) { r.Source.AppName = "测试来源" })
			if n := s.CollectGarbage(); n != 0 {
				t.Errorf("并发回收删除了仍有引用的资源：%d", n)
			}
			if err := s.Flush(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	data, err := os.ReadFile(filepath.Join(s.Dir(), indexFile))
	var idx index
	if err != nil || json.Unmarshal(data, &idx) != nil || len(idx.Records) != 12 {
		t.Fatalf("并发提交丢记录或损坏索引：%s %v", data, err)
	}
	for name := range references(idx.Records) {
		if _, err := s.ReadBlob(name); err != nil {
			t.Fatalf("索引引用了不存在的资源：%s %v", name, err)
		}
	}
}

// TestStorageReplacementAndShrink 验证替换资源及时回收旧图标与预览且缩小容量立即清理历史
func TestStorageReplacementAndShrink(t *testing.T) {
	s := storageTest(t, 4)
	r := captureTest(t, s, &types.Record{Hash: "same", Kind: types.KindImage}, bytes.Repeat([]byte{7}, 600<<10), []byte("old icon"))
	if _, _, err := s.UpdatePreview(r.ID, []byte("old preview"), "jpg", func(*types.Record) {}); err != nil {
		t.Fatal(err)
	}
	old, _ := s.Get(r.ID)
	replaced := captureTest(t, s, &types.Record{Hash: "same", Kind: types.KindImage}, bytes.Repeat([]byte{8}, 600<<10), []byte("new icon"))
	if _, _, err := s.UpdatePreview(replaced.ID, []byte("new preview"), "jpg", func(*types.Record) {}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{old.Image, old.Source.Icon, old.PreviewImage} {
		if _, err := os.Stat(s.BlobPath(name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("失去引用的旧资源未回收：%s", name)
		}
	}
	newest := captureTest(t, s, &types.Record{Kind: types.KindImage}, bytes.Repeat([]byte{9}, 600<<10), nil)
	settings := s.Settings()
	settings.HistoryLimitMiB = 1
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 1 || s.List()[0].ID != newest.ID {
		t.Fatal("缩小容量没有优先保留最新历史")
	}
	entries, _ := os.ReadDir(s.BlobDir())
	if len(entries) != 1 {
		t.Fatal("缩小容量后未回收旧图与来源图标")
	}
}
