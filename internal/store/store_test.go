package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopaste/internal/types"
)

// openTest opens a store in a fixed directory under .mygo: a temporary
// directory needs a cleanup step some sandboxes deny.
func openTest(t *testing.T) *Store {
	t.Helper()
	dir := filepath.Join("..", "..", ".mygo", "storetest")
	_ = os.RemoveAll(dir)
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func imageRecord(hash string) *types.Record {
	return &types.Record{Hash: hash, Kind: types.KindImage, CreatedAt: time.Now()}
}

// TestMoveToFront 验证移至最前保留记录信息、其他记录顺序与重启后的排序
func TestMoveToFront(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created := time.Now().Add(-time.Hour)
	one, _ := s.Add(&types.Record{ID: "one", Kind: types.KindText, Text: "第一条", CreatedAt: created, ListID: "work"})
	s.Add(&types.Record{ID: "two", Kind: types.KindText, Text: "第二条"})
	s.Add(&types.Record{ID: "three", Kind: types.KindText, Text: "第三条"})
	before := s.List()
	for _, id := range []string{one.ID, one.ID, "missing"} {
		if err := s.MoveToFront(id); err != nil {
			t.Fatal(err)
		}
	}
	list := s.List()
	if len(list) != 3 || list[0].ID != "one" || list[1].ID != "three" || list[2].ID != "two" {
		t.Fatal("移至最前改变了其他记录顺序或产生重复记录")
	}
	if before[0].ID != "three" || before[1].ID != "two" || before[2].ID != "one" {
		t.Fatal("调整顺序修改了此前读取的列表快照")
	}
	if list[0] != one || !list[0].CreatedAt.Equal(created) || list[0].ListID != "work" {
		t.Fatal("调整顺序改变了记录信息")
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if list := reopened.List(); len(list) != 3 || list[0].ID != "one" || list[1].ID != "three" || list[2].ID != "two" {
		t.Fatal("重新读取后记录顺序丢失")
	}
}

// TestAddMergesSameContent 验证重复复制保留记录标识、列表归属与链接标题
func TestAddMergesSameContent(t *testing.T) {
	s := openTest(t)
	first, ok := s.Add(&types.Record{Hash: "h1", Kind: types.KindLink, URL: "https://a.example/x", Title: "A"})
	if ok {
		t.Fatal("first add reported a merge")
	}
	firstID := first.ID
	if _, ok := s.Update(firstID, func(r *types.Record) { r.ListID = "work" }); !ok {
		t.Fatal("移入普通列表失败")
	}
	if first.ListID != "" {
		t.Fatal("记录修改影响了此前取得的只读快照")
	}

	second, merged := s.Add(&types.Record{Hash: "h1", Kind: types.KindLink, URL: "https://a.example/x"})
	if !merged {
		t.Fatal("second add did not merge")
	}
	if second.ID != firstID {
		t.Fatalf("id changed: %s != %s", second.ID, firstID)
	}
	if second.ListID != "work" {
		t.Fatalf("合并后列表归属丢失：%+v", second)
	}
	if second.Title != "A" {
		t.Fatalf("lost the title learned for the link: %q", second.Title)
	}
	if n := len(s.List()); n != 1 {
		t.Fatalf("want 1 record, got %d", n)
	}
}

func TestDeleteRemovesBlob(t *testing.T) {
	s := openTest(t)
	name, err := s.SaveBlob([]byte("picture"), "png")
	if err != nil {
		t.Fatalf("save blob: %v", err)
	}
	r, _ := s.Add(&types.Record{Hash: "h2", Kind: types.KindImage, Image: name})
	s.Delete(r.ID)
	if _, err := os.Stat(s.BlobPath(name)); !os.IsNotExist(err) {
		t.Fatalf("blob still there: %v", err)
	}
}

func TestPruneRemovesBlob(t *testing.T) {
	s := openTest(t)
	st := s.Settings()
	st.MaxRecords = 2
	if err := s.SetSettings(st); err != nil {
		t.Fatalf("settings: %v", err)
	}
	var names []string
	for i, h := range []string{"a", "b", "c"} {
		name, err := s.SaveBlob([]byte{byte(i)}, "png")
		if err != nil {
			t.Fatalf("save blob: %v", err)
		}
		names = append(names, name)
		r := imageRecord(h)
		r.Image = name
		s.Add(r)
	}
	if n := len(s.List()); n != 2 {
		t.Fatalf("want 2 records kept, got %d", n)
	}
	if _, err := os.Stat(s.BlobPath(names[0])); !os.IsNotExist(err) {
		t.Fatal("the picture of the pruned record is still on disk")
	}
}

// TestPruneKeepsManualLists 验证普通列表免于自动清理且不占历史配额，移出后恢复清理
func TestPruneKeepsManualLists(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	settings := st.Settings()
	settings.MaxRecords, settings.RetentionDays = 2, 1
	settings.CustomTabs = []types.Tab{{ID: "saved", Name: "保存"}}
	if err := st.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	image, err := st.SaveBlob([]byte("保存的图片"), "png")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -30)
	st.Add(&types.Record{ID: "saved-old", Kind: types.KindImage, ListID: "saved", Image: image, CreatedAt: old})
	for _, id := range []string{"history-one", "saved-one", "history-two", "saved-two", "history-three", "saved-three"} {
		r := &types.Record{ID: id, Kind: types.KindText}
		if id == "saved-one" || id == "saved-two" || id == "saved-three" {
			r.ListID = "saved"
		}
		st.Add(r)
	}
	st.Add(&types.Record{ID: "expired", Kind: types.KindText, CreatedAt: old})
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(st.Dir())
	if err != nil {
		t.Fatal(err)
	}
	st = reopened
	for _, id := range []string{"saved-old", "saved-one", "saved-two", "saved-three", "history-two", "history-three"} {
		if _, ok := st.Get(id); !ok {
			t.Fatalf("记录 %s 被错误清理或未持久化", id)
		}
	}
	if len(st.List()) != 6 {
		t.Fatalf("普通列表记录占用了历史配额或未清理过期记录：%d", len(st.List()))
	}
	if _, ok := st.Get("history-one"); ok {
		t.Fatal("超量历史未清理")
	}
	if _, ok := st.Get("expired"); ok {
		t.Fatal("过期历史未清理")
	}
	if n := st.CollectGarbage(); n != 0 {
		t.Fatal("分组中的图片被当作垃圾回收")
	}
	if _, err := st.ReadBlob(image); err != nil {
		t.Fatalf("分组中的图片丢失：%v", err)
	}
	st.Update("saved-old", func(r *types.Record) { r.ListID = "" })
	st.Update("saved-one", func(r *types.Record) { r.ListID = "" })
	st.Add(&types.Record{ID: "new-history", Kind: types.KindText})
	for _, id := range []string{"saved-old", "saved-one"} {
		if _, ok := st.Get(id); ok {
			t.Fatalf("移出分组后记录 %s 未恢复自动清理", id)
		}
	}
	if _, err := os.Stat(st.BlobPath(image)); !os.IsNotExist(err) {
		t.Fatalf("移出分组的过期图片未回收：%v", err)
	}
}

// TestCollectGarbage 验证回收新近产生的孤立资源并保留仍有引用的图片
func TestCollectGarbage(t *testing.T) {
	s := openTest(t)
	keep, err := s.SaveBlob([]byte("keep"), "png")
	if err != nil {
		t.Fatalf("save blob: %v", err)
	}
	s.Add(&types.Record{Hash: "h3", Kind: types.KindImage, Image: keep})

	orphan, err := s.SaveBlob([]byte("orphan"), "png")
	if err != nil {
		t.Fatalf("save blob: %v", err)
	}
	if n := s.CollectGarbage(); n != 1 {
		t.Fatalf("want 1 file cleaned, got %d", n)
	}
	if _, err := os.Stat(s.BlobPath(keep)); err != nil {
		t.Fatalf("removed a picture still in use: %v", err)
	}
	if _, err := os.Stat(s.BlobPath(orphan)); !os.IsNotExist(err) {
		t.Fatal("the orphan is still on disk")
	}
}
