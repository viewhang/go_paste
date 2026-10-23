package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/store"
	"gopaste/internal/types"
)

// TestManualLists 验证两种面板中的列表创建、记录移动、编辑删除与持久化
func TestManualLists(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		t.Run(fmt.Sprint(form), func(t *testing.T) {
			st, err := store.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			st.Add(&types.Record{ID: "one", Kind: types.KindText, Text: "合同草稿"})
			st.Add(&types.Record{ID: "two", Kind: types.KindText, Text: "其他内容"})
			a := NewApp()
			a.st, a.form = st, form
			a.reload()
			w, h := panelWidth, panelHeight
			if form == formBar {
				w, h = 1180, barHeight
			}
			tt := ui.NewTester(a.view, w, h)
			if len(a.tabs()) != 1 || a.tabs()[0].Name != "全部" {
				t.Fatal("默认列表未缩减为全部")
			}
			// click 点击界面控件并完成重绘
			click := func(label string) {
				t.Helper()
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			// choose 执行记录或列表的右键菜单操作
			choose := func(label string, path ...string) {
				t.Helper()
				if err := tt.RightClick(label); err != nil {
					t.Fatal(err)
				}
				if err := tt.ChooseMenuItem(path...); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			for _, name := range []string{"工作", "归档"} {
				click("添加列表")
				if err := tt.ChooseMenuItem("普通列表"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Type(name)
				click("创建")
				if a.currentTab().Name != name || len(a.vis) != 0 {
					t.Fatal("普通列表创建失败或自动收录了记录")
				}
			}
			click("全部")
			choose("合同草稿", "移动到", "工作")
			click("工作")
			if len(a.vis) != 1 || !tt.HasText("1 条") {
				t.Fatal("移动后列表内容或数量不正确")
			}
			choose("合同草稿", "移动到", "归档")
			if len(a.vis) != 0 {
				t.Fatal("跨列表移动后原列表仍包含记录")
			}
			click("归档")
			choose("归档", "编辑列表")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("已归档")
			click("保存")
			if a.currentTab().Name != "已归档" || len(a.vis) != 1 {
				t.Fatal("列表改名后丢失记录")
			}
			choose("合同草稿", "移动到", "全部")
			if len(a.vis) != 0 || len(st.List()) != 2 {
				t.Fatal("移出列表未保留全部历史")
			}
			click("全部")
			choose("合同草稿", "移动到", "已归档")
			click("已归档")
			choose("已归档", "删除列表")
			if a.currentTab().ID != "all" || len(a.vis) != 2 {
				t.Fatal("删除列表后未返回全部记录")
			}
			a.deleteTab(a.tabs()[0])
			if len(a.tabs()) != 2 {
				t.Fatal("全部入口被删除")
			}
			reopened, err := store.Open(st.Dir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reopened.Close() })
			r, _ := reopened.Get("one")
			if r.ListID != "" || len(reopened.Settings().CustomTabs) != 1 || len(reopened.List()) != 2 {
				t.Fatal("列表删除或记录归属未正确保存")
			}
		})
	}
}

// TestDragToManualLists 验证两种主题和面板中的拖入移动、跨列表移动与智能列表拒收
func TestDragToManualLists(t *testing.T) {
	for _, form := range []int{formBar, formPanel} {
		for _, theme := range []int{themeClassic, themeGlass} {
			t.Run(fmt.Sprintf("form-%d-theme-%d", form, theme), func(t *testing.T) {
				st, err := store.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = st.Close() })
				settings := st.Settings()
				settings.Theme = theme
				settings.CustomTabs = []types.Tab{
					{ID: "work", Name: "工作"},
					{ID: "archive", Name: "归档"},
					{ID: "smart", Name: "智能", Smart: true, Rules: []types.ListRule{{Field: types.RuleKind, Value: "image"}}},
				}
				if err := st.SetSettings(settings); err != nil {
					t.Fatal(err)
				}
				record, _ := st.Add(&types.Record{ID: "one", Kind: types.KindText, Text: "拖动内容"})
				a := NewApp()
				a.st, a.form = st, form
				a.reload()
				w, h := panelWidth, panelHeight
				if form == formBar {
					w, h = 1180, barHeight
				}
				tt := ui.NewTester(func(c *ui.Context) {
					a.view(c)
					ui.Box(c).Absolute().Bottom(3).Left(3).Size(20, 20).Label("拖动记录").Drag(record)
				}, w, h)
				for _, target := range []struct{ name, id string }{
					{"工作", "work"}, {"归档", "archive"}, {"智能", "archive"}, {"全部", "archive"},
				} {
					from, ok := tt.Find("拖动记录")
					if !ok {
						t.Fatal("拖拽来源未显示")
					}
					to, ok := tt.Find(target.name)
					if !ok {
						t.Fatalf("目标列表 %s 未显示", target.name)
					}
					tt.Press(from.X+10, from.Y+10)
					tt.Move(from.X+16, from.Y+10)
					tt.Move(to.X+to.W/2, to.Y+to.H/2)
					if current, ok := tt.Find(target.name); !ok || current != to {
						t.Fatal("拖入高亮改变了标签布局")
					}
					tt.Release(to.X+to.W/2, to.Y+to.H/2)
					saved, ok := st.Get(record.ID)
					if !ok || saved.ListID != target.id || len(st.List()) != 1 {
						t.Fatalf("拖到 %s 后归属不正确或重复创建了记录", target.name)
					}
					record = saved
					if target.id == "work" {
						if err := tt.Click("工作"); err != nil {
							t.Fatal(err)
						}
					} else if len(a.vis) != 0 {
						t.Fatal("跨列表移动后原列表仍显示该记录")
					}
				}
				reopened, err := store.Open(st.Dir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = reopened.Close() })
				if saved, ok := reopened.Get(record.ID); !ok || saved.ListID != "archive" {
					t.Fatal("拖拽移动后的列表归属未保存")
				}
			})
		}
	}
}

// TestSmartLists 验证智能列表的规则编辑、匹配方式切换、新记录收录与持久化
func TestSmartLists(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, r := range []*types.Record{
		{ID: "contract", Kind: types.KindFile, Files: []types.FileRef{{Name: "合同.PDF"}}},
		{ID: "budget", Kind: types.KindFile, Files: []types.FileRef{{Name: "预算.pdf"}}},
		{ID: "document", Kind: types.KindFile, Files: []types.FileRef{{Name: "合同.docx"}}},
	} {
		st.Add(r)
	}
	a := NewApp()
	a.st, a.form = st, formPanel
	a.reload()
	tt := ui.NewTester(a.view, panelWidth, panelHeight)
	// click 点击规则表单中的控件
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("添加列表")
	if err := tt.ChooseMenuItem("智能列表"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	click("创建")
	if a.prompt == nil || !tt.HasText("请输入列表名称") {
		t.Fatal("未拦截空列表名称")
	}
	click("列表名称输入框")
	tt.Type("合同 PDF")
	click("规则类型1")
	click("文件类型")
	click("规则内容1")
	tt.Type(".PDF")
	click("添加规则")
	click("创建")
	if a.prompt == nil || !tt.HasText("请填写完整的规则") {
		t.Fatal("未拦截空规则")
	}
	click("规则内容2")
	tt.Type("合同")
	click("创建")
	if a.prompt != nil || len(a.vis) != 1 || a.vis[0].ID != "contract" {
		t.Fatalf("智能列表未按全部规则匹配：表单 %v，结果 %v", a.prompt, a.vis)
	}
	tab := a.currentTab()
	a.editTab(tab)
	tt.Frame()
	click("规则内容2")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("预算")
	click("任意满足")
	click("取消")
	if a.currentTab().Rules[1].Value != "合同" || a.currentTab().MatchAny || len(a.vis) != 1 {
		t.Fatal("取消编辑改变了已保存规则")
	}
	a.editTab(tab)
	tt.Frame()
	click("规则内容2")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("预算")
	click("保存")
	if len(a.vis) != 1 || a.vis[0].ID != "budget" {
		t.Fatal("规则编辑后未重新匹配历史记录")
	}
	st.Add(&types.Record{ID: "new", Kind: types.KindFile, Files: []types.FileRef{{Name: "新预算.pdf"}}})
	a.reload()
	tt.Frame()
	if len(a.vis) != 2 {
		t.Fatal("新复制记录未自动进入智能列表")
	}
	a.editTab(a.currentTab())
	tt.Frame()
	click("任意满足")
	click("保存")
	if !a.currentTab().MatchAny || len(a.vis) != 3 {
		t.Fatal("任意满足未收录仅符合一条规则的记录")
	}
	reopened, err := store.Open(st.Dir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	saved := reopened.Settings().CustomTabs[0]
	if !saved.Smart || !saved.MatchAny || len(saved.Rules) != 2 || saved.Rules[1].Value != "预算" {
		t.Fatal("智能列表规则未保存")
	}
}

// TestBarAddListPosition 验证两种主题的新增入口紧随最后一个列表
func TestBarAddListPosition(t *testing.T) {
	for _, theme := range []int{themeClassic, themeGlass} {
		for _, tabs := range [][]types.Tab{nil, {{ID: "work", Name: "工作"}, {ID: "archive", Name: "归档"}}} {
			a := NewApp()
			a.settings = types.DefaultSettings()
			a.settings.Theme, a.settings.CustomTabs = theme, tabs
			initTestStore(t, a)
			tt := ui.NewTester(a.view, 1180, barHeight)
			last := "全部"
			if len(tabs) > 0 {
				last = tabs[len(tabs)-1].Name
			}
			label, ok := tt.Find(last)
			if !ok {
				t.Fatal("未显示最后一个列表")
			}
			add, ok := tt.Find("添加列表")
			if gap := add.X - label.X - label.W; !ok || gap < 0 || gap > 20 {
				t.Fatalf("新增入口未紧随列表：列表 %+v，入口 %+v", label, add)
			}
		}
	}
}

// TestBarAddListVisible 验证列表溢出后可滚动至新增入口并创建普通或智能列表
func TestBarAddListVisible(t *testing.T) {
	a := NewApp()
	a.settings = types.DefaultSettings()
	for i := range 20 {
		a.settings.CustomTabs = append(a.settings.CustomTabs, types.Tab{ID: fmt.Sprint(i), Name: "较长的列表名称"})
	}
	initTestStore(t, a)
	tt := ui.NewTester(a.view, 760, barHeight)
	for _, choice := range []string{"普通列表", "智能列表"} {
		tt.Scroll(500, 24, 10000, 0)
		if err := tt.Click("添加列表"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(choice); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.prompt == nil {
			t.Fatal("长条新增列表入口未打开")
		}
		if a.prompt.tab.Smart {
			tt.Scroll(380, 118, 0, 300)
			if err := tt.Click("添加规则"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Scroll(380, 118, 0, 300)
			if err := tt.Click("删除规则2"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(a.prompt.tab.Rules) != 1 {
				t.Fatal("贴底面板无法增删智能规则")
			}
		}
		if err := tt.Click("取消"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
}
