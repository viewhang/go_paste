package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"

	"gopaste/internal/types"
)

// TestDatabaseMigrationAndSummaries 验证迁移原文、分页摘要、完整检索和重启后的顺序
func TestDatabaseMigrationAndSummaries(t *testing.T) {
	dir := t.TempDir()
	text := strings.Repeat("中文🙂", 2000) + "末尾Ω"
	record := &types.Record{ID: "long", Kind: types.KindText, Text: text, HTML: "<p>" + text + "</p>", ListID: "saved"}
	files := &types.Record{ID: "files", Kind: types.KindFile, ListID: "saved", Files: []types.FileRef{{Name: "one.txt", Path: "/one.txt"}, {Name: "two.md", Path: "/two.md"}, {Name: "three.txt", Path: "/three.txt"}, {Name: "最后Ω.tar.gz", Path: "/四.tar.gz"}, {Name: "目录.zip", Dir: true}}}
	if err := writeJSON(filepath.Join(dir, indexFile), index{Version: 1, Records: []*types.Record{record, files}}); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Join(dir, indexFile)); !os.IsNotExist(err) {
		t.Fatal("迁移后仍保留活动 JSON 索引")
	}
	full, ok := s.Get("long")
	if !ok || full.Text != text || full.HTML != record.HTML || full.ListID != "saved" || full.Hash == "" {
		t.Fatal("迁移丢失原文或整理信息")
	}
	pages, err := s.Summaries([]string{"long", "files"})
	if err != nil || len(pages) != 2 {
		t.Fatal(err)
	}
	for _, r := range pages {
		if !r.Summary || r.HTML != "" {
			t.Fatal("摘要包含富文本")
		}
		if r.ID == "long" && (len([]rune(r.Text)) > 513 || r.Text == text) {
			t.Fatal("正文未限制长度")
		}
		if r.ID == "files" && (len(r.Files) != 3 || r.FileCount != 5) {
			t.Fatal("文件摘要数量错误")
		}
	}
	for _, tc := range []struct {
		tab   types.Tab
		query string
		want  string
	}{
		{types.Tab{ID: "all"}, "末尾ω", "long"},
		{types.Tab{ID: "all"}, "四.tar", "files"},
		{types.Tab{Smart: true, Rules: []types.ListRule{{Field: types.RuleExtension, Value: "tar.gz"}}}, "", "files"},
		{types.Tab{Smart: true, Rules: []types.ListRule{{Field: types.RuleExtension, Value: "zip"}}}, "", ""},
		{types.Tab{Smart: true, Rules: []types.ListRule{{Field: types.RuleKeyword, Value: "末尾Ω"}, {Field: types.RuleKind, Value: "text"}}}, "", "long"},
	} {
		rows, err := s.IDs(tc.tab, tc.query)
		if err != nil {
			t.Fatal(err)
		}
		if tc.want == "" {
			if len(rows) != 0 {
				t.Fatal("错误匹配目录扩展名")
			}
		} else if len(rows) != 1 || rows[0].ID != tc.want {
			t.Fatalf("搜索摘要之外的内容失败：%+v %+v", tc, rows)
		}
	}
	for _, r := range s.records {
		if r.Text != "" || r.HTML != "" || len(r.Files) != 0 {
			t.Fatal("常驻元数据持有完整内容")
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rows := reopened.List()
	if len(rows) != 2 || rows[0].ID != "long" || rows[1].ID != "files" || len(rows[1].Files) != 5 {
		t.Fatal("重启后内容或顺序丢失")
	}
}

// TestPendingPreviewRecovery 验证重启统一重置中断预览且保留内容、顺序与已完成状态
func TestPendingPreviewRecovery(t *testing.T) {
	s := storageTest(t, 1)
	for _, state := range []string{"loading", "done", ""} {
		captureTest(t, s, &types.Record{Kind: types.KindLink, Text: "预览内容 " + state, URL: "https://example.com/" + state, PreviewState: state}, nil, nil)
	}
	want := s.List()
	ids := make([]string, len(want))
	states := make(map[string]string, len(want))
	for i, r := range want {
		if r.PreviewState == "loading" {
			r.PreviewState = ""
		}
		ids[i], states[r.ID] = r.ID, r.PreviewState
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.List(); !reflect.DeepEqual(got, want) {
		t.Fatal("重启未恢复预览状态或改变了记录内容与顺序")
	}
	summaries, err := reopened.Summaries(ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, records := range [][]*types.Record{reopened.records, summaries} {
		if len(records) != len(want) {
			t.Fatal("重启丢失元数据或摘要")
		}
		for _, r := range records {
			if r.PreviewState != states[r.ID] {
				t.Fatal("元数据或摘要中的预览状态与原文不一致")
			}
		}
	}
	pending, err := reopened.PendingLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatal("中断预览未重新加入待处理记录")
	}
	for _, r := range pending {
		if r.PreviewState != "" {
			t.Fatal("已完成预览被重新加入待处理记录")
		}
	}
}

// TestDatabaseTransactionRollback 验证事务中途失败时不产生部分写入
func TestDatabaseTransactionRollback(t *testing.T) {
	s := storageTest(t, 1)
	before := captureTest(t, s, &types.Record{ID: "old", Kind: types.KindText, Text: "old"}, nil, nil)
	settings := s.Settings()
	settings.MaxRecords = 1
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Execute(context.Background(), `CREATE TRIGGER reject_delete BEFORE DELETE ON records BEGIN SELECT RAISE(ABORT,'test rejection'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Capture(&types.Record{ID: "new", Kind: types.KindText, Text: "new"}, nil, nil); err == nil {
		t.Fatal("事务应被触发器终止")
	}
	if _, ok := s.Get("new"); ok {
		t.Fatal("失败事务残留新记录")
	}
	got, ok := s.Get("old")
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(got)
	if !ok || string(a) != string(b) {
		t.Fatal("失败事务改变旧记录")
	}
}

// TestSettingsDatabaseMigration 验证两种旧存储迁移完整设置并在重启后以数据库为准
func TestSettingsDatabaseMigration(t *testing.T) {
	for _, source := range []string{"json", "sqlite-v1"} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			record := &types.Record{ID: "saved", Kind: types.KindText, Text: "保留历史", ListID: "work"}
			if source == "json" {
				if err := writeJSON(filepath.Join(dir, indexFile), index{Version: 1, Records: []*types.Record{record}}); err != nil {
					t.Fatal(err)
				}
			} else {
				s, err := Open(dir)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				captureTest(t, s, record, nil, nil)
				if _, err := s.db.Transaction(context.Background(), []sqlite.Statement{{SQL: "DROP TABLE settings"}, {SQL: "PRAGMA user_version=1"}}); err != nil {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
			want := types.DefaultSettings()
			want.Hotkey, want.Theme, want.Form = "Alt+V", 1, 1
			want.Shortcuts = map[string]string{"copy": "Alt+C", "settings": "Alt+,"}
			want.ExcludedApps = []types.ExcludedApp{}
			want.ExcludeWhitespace, want.LinkPreview, want.ImageLimitMiB = false, false, 0
			want.CustomTabs = []types.Tab{{ID: "work", Name: "工作"}, {ID: "smart", Name: "智能", Smart: true, MatchAny: true, Rules: []types.ListRule{{Field: types.RuleKeyword, Value: "中文🙂"}, {Field: types.RuleKind, Value: "text"}}}}
			if err := writeJSON(filepath.Join(dir, legacySettingsFile), want); err != nil {
				t.Fatal(err)
			}
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if !reflect.DeepEqual(s.Settings(), want) {
				t.Fatalf("迁移改变设置：%+v", s.Settings())
			}
			for _, name := range []string{indexFile, legacySettingsFile} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("迁移后仍保留 %s", name)
				}
			}
			rows, err := s.db.Query(context.Background(), "PRAGMA user_version")
			if err != nil || rows.Values[0][0] != int64(2) {
				t.Fatalf("数据库版本未更新：%v %v", rows, err)
			}
			want.ExcludedApps = []types.ExcludedApp{{ID: "com.example.Editor", Name: "编辑器", Path: "/Applications/Editor.app"}}
			want.PanelWidth, want.PanelHeight, want.BarHeight = 700, 800, 300
			if err := s.SetSettings(want); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, legacySettingsFile)); !os.IsNotExist(err) {
				t.Fatal("保存设置重新生成了 JSON 文件")
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(filepath.Join(dir, legacySettingsFile), types.DefaultSettings()); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if !reflect.DeepEqual(reopened.Settings(), want) {
				t.Fatal("重启丢失设置或被旧 JSON 覆盖")
			}
			if got, ok := reopened.Get(record.ID); !ok || got.Text != record.Text || got.ListID != record.ListID {
				t.Fatal("迁移丢失历史或列表归属")
			}
			if _, err := os.Stat(filepath.Join(dir, legacySettingsFile)); !os.IsNotExist(err) {
				t.Fatal("未清理旧 JSON 设置")
			}
		})
	}
}

// TestSettingsMigrationFailure 验证设置解析失败时保留旧文件且不提交部分迁移
func TestSettingsMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	if err := writeJSON(filepath.Join(dir, indexFile), index{Version: 1, Records: []*types.Record{{ID: "old", Kind: types.KindText, Text: "保留历史"}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacySettingsFile), []byte(`{"hotkey":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("损坏设置应终止迁移")
	}
	for _, name := range []string{indexFile, legacySettingsFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("失败迁移删除了旧文件：%v", err)
		}
	}
	db, err := sqlite.Open(context.Background(), filepath.Join(dir, databaseFile), sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "PRAGMA user_version")
	if err != nil || rows.Values[0][0] != int64(0) {
		t.Fatalf("失败迁移提交了版本：%v %v", rows, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(dir, legacySettingsFile), types.DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok := s.Get("old"); !ok {
		t.Fatal("修复设置后重试丢失历史")
	}
}

// TestSettingsWriteRollback 验证普通设置写入与联动清理失败时均保留设置、记录及图片
func TestSettingsWriteRollback(t *testing.T) {
	for _, prune := range []bool{false, true} {
		name := "settings-only"
		if prune {
			name = "settings-and-pruning"
		}
		t.Run(name, func(t *testing.T) {
			s := storageTest(t, 1)
			for _, id := range []string{"old", "new"} {
				captureTest(t, s, &types.Record{ID: id, Kind: types.KindImage}, []byte(id), nil)
			}
			before, records := s.Settings(), s.List()
			changed := before
			changed.Theme = 1
			reject, allow := "PRAGMA query_only=ON", "PRAGMA query_only=OFF"
			if prune {
				changed.MaxRecords = 1
				reject = `CREATE TRIGGER reject_delete BEFORE DELETE ON records BEGIN SELECT RAISE(ABORT,'test rejection'); END`
				allow = "DROP TRIGGER reject_delete"
			}
			if _, err := s.db.Execute(context.Background(), reject); err != nil {
				t.Fatal(err)
			}
			if err := s.SetSettings(changed); err == nil {
				t.Fatal("写入受阻时未报告失败")
			}
			if !reflect.DeepEqual(s.Settings(), before) || !reflect.DeepEqual(s.List(), records) {
				t.Fatal("失败写入改变了设置或历史")
			}
			rows, err := s.db.Query(context.Background(), "SELECT data FROM settings WHERE id=1")
			if err != nil {
				t.Fatal(err)
			}
			var persisted types.Settings
			if err := json.Unmarshal([]byte(rows.Values[0][0].(string)), &persisted); err != nil || !reflect.DeepEqual(persisted, before) {
				t.Fatalf("失败写入改变数据库设置：%v", err)
			}
			for _, r := range records {
				if data, err := s.ReadBlob(r.Image); err != nil || string(data) != r.ID {
					t.Fatal("失败写入删除了原图")
				}
			}
			if _, err := s.db.Execute(context.Background(), allow); err != nil {
				t.Fatal(err)
			}
			if err := s.SetSettings(changed); err != nil {
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
			if !reflect.DeepEqual(reopened.Settings(), changed) {
				t.Fatal("保存设置后重启丢失修改")
			}
			wantCount := 2
			if prune {
				wantCount = 1
			}
			if got := reopened.List(); len(got) != wantCount || got[0].ID != "new" {
				t.Fatal("设置保存成功后未正确保留历史")
			}
		})
	}
}

// TestSmartListMatchingModes 验证全部满足与任意满足的区别及无规则时的匹配结果
func TestSmartListMatchingModes(t *testing.T) {
	tab := types.Tab{ID: "smart", Smart: true, Rules: []types.ListRule{
		{Field: types.RuleKind, Value: string(types.KindFile)},
		{Field: types.RuleKeyword, Value: "合同"},
	}}
	for _, tc := range []struct {
		record types.Record
		all    bool
		any    bool
	}{
		{types.Record{Kind: types.KindFile, Files: []types.FileRef{{Name: "合同.pdf"}}}, true, true},
		{types.Record{Kind: types.KindFile, Files: []types.FileRef{{Name: "预算.pdf"}}}, false, true},
		{types.Record{Kind: types.KindText, Text: "合同"}, false, true},
		{types.Record{Kind: types.KindText, Text: "其他"}, false, false},
	} {
		s := storageTest(t, 1)
		captureTest(t, s, &tc.record, nil, nil)
		for _, any := range []bool{false, true} {
			tab.MatchAny = any
			want := tc.all
			if any {
				want = tc.any
			}
			rows, err := s.IDs(tab, "")
			if err != nil {
				t.Fatal(err)
			}
			if got := len(rows) != 0; got != want {
				t.Errorf("任意满足 %v，记录 %v：得到 %v，期望 %v", any, tc.record, got, want)
			}
		}
	}
	s := storageTest(t, 1)
	captureTest(t, s, &types.Record{Kind: types.KindText, Text: "合同"}, nil, nil)
	tab.Rules = nil
	for _, any := range []bool{false, true} {
		tab.MatchAny = any
		rows, err := s.IDs(tab, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 0 {
			t.Fatal("无规则的智能列表不应收录记录")
		}
	}
}

// TestListRules 验证内容类型、扩展名与关键词的匹配范围
func TestListRules(t *testing.T) {
	for _, tc := range []struct {
		rule types.ListRule
		rec  types.Record
		want bool
	}{
		{types.ListRule{Field: types.RuleKind, Value: "image"}, types.Record{Kind: types.KindImage}, true},
		{types.ListRule{Field: types.RuleKind, Value: "file"}, types.Record{Kind: types.KindText}, false},
		{types.ListRule{Field: types.RuleExtension, Value: ".pdf， docx"}, types.Record{Files: []types.FileRef{{Name: "报告.DOCX"}}}, true},
		{types.ListRule{Field: types.RuleExtension, Value: "png"}, types.Record{Kind: types.KindImage, ImageFormat: "png"}, true},
		{types.ListRule{Field: types.RuleExtension, Value: "jpg"}, types.Record{Kind: types.KindImage, ImageFormat: "jpeg"}, true},
		{types.ListRule{Field: types.RuleExtension, Value: "pdf"}, types.Record{Files: []types.FileRef{{Name: "目录.pdf", Dir: true}}}, false},
		{types.ListRule{Field: types.RuleKeyword, Value: "project"}, types.Record{Text: "PROJECT notes"}, true},
		{types.ListRule{Field: types.RuleKeyword, Value: "合同"}, types.Record{Files: []types.FileRef{{Name: "合同.pdf"}}}, true},
	} {
		s := storageTest(t, 1)
		captureTest(t, s, &tc.rec, nil, nil)
		rows, err := s.IDs(types.Tab{Smart: true, Rules: []types.ListRule{tc.rule}}, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := len(rows) != 0; got != tc.want {
			t.Errorf("规则 %v 匹配 %v 得到 %v", tc.rule, tc.rec, got)
		}
	}
}
