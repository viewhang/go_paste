package app

import (
	"errors"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"gopaste/internal/locales"
	"gopaste/internal/store"
	"gopaste/internal/types"
)

// tabs 返回全部记录入口与用户创建的列表
func (a *App) tabs() []types.Tab {
	return append([]types.Tab{{ID: "all", Name: locales.All()}}, a.settings.CustomTabs...)
}

// currentTab 返回当前选中的列表
func (a *App) currentTab() types.Tab {
	tabs := a.tabs()
	if a.tab >= 0 && a.tab < len(tabs) {
		return tabs[a.tab]
	}
	return tabs[0]
}

// listMatches 按普通列表归属或智能列表的全部、任意规则筛选记录
func listMatches(tab types.Tab, r *types.Record) bool {
	if tab.ID == "all" {
		return true
	}
	if !tab.Smart {
		return r.ListID == tab.ID
	}
	for _, rule := range tab.Rules {
		matched := ruleMatches(rule, r)
		if tab.MatchAny && matched {
			return true
		}
		if !tab.MatchAny && !matched {
			return false
		}
	}
	return !tab.MatchAny && len(tab.Rules) > 0
}

// ruleMatches 按内容类型、文件扩展名或关键词匹配记录
func ruleMatches(rule types.ListRule, r *types.Record) bool {
	switch rule.Field {
	case types.RuleKind:
		return string(r.Kind) == rule.Value
	case types.RuleKeyword:
		return matches(r, strings.ToLower(rule.Value))
	case types.RuleExtension:
		for _, ext := range ruleExtensions(rule.Value) {
			if r.Kind == types.KindImage && (strings.EqualFold(r.ImageFormat, ext) || r.ImageFormat == "jpeg" && ext == "jpg") {
				return true
			}
			for _, f := range r.Files {
				if !f.Dir && strings.HasSuffix(strings.ToLower(f.Name), "."+ext) {
					return true
				}
			}
		}
	}
	return false
}

// ruleExtensions 解析由逗号或空白分隔的文件扩展名
func ruleExtensions(value string) []string {
	var extensions []string
	for _, ext := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ',' || r == '，' || unicode.IsSpace(r)
	}) {
		if ext = strings.TrimPrefix(ext, "."); ext != "" {
			extensions = append(extensions, ext)
		}
	}
	return extensions
}

// newTab 打开普通列表或智能列表的创建表单
func (a *App) newTab(smart bool) {
	a.editTab(types.Tab{Smart: smart})
}

// editTab 编辑列表名称与智能规则并保留取消前的原始设置
func (a *App) editTab(tab types.Tab) {
	tab.Rules = slices.Clone(tab.Rules)
	title := locales.EditManualList
	if tab.Smart {
		title = locales.EditSmartList
	}
	okText := locales.Save
	if tab.ID == "" {
		title = locales.NewManualList
		if tab.Smart {
			title = locales.NewSmartList
			tab.Rules = []types.ListRule{{Field: types.RuleKind, Value: string(types.KindFile)}}
		}
		okText = locales.Create
	}
	a.prompt = &promptState{title: title, label: locales.ListName, value: tab.Name, okText: okText, tab: &tab}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// saveTab 校验并保存列表名称与规则，新列表保存后自动选中
func (a *App) saveTab(tab types.Tab, name string) error {
	tab.Name = strings.TrimSpace(name)
	if tab.Name == "" {
		return errors.New(locales.ListNameRequired())
	}
	if tab.Smart {
		if len(tab.Rules) == 0 {
			return errors.New(locales.ListRuleRequired())
		}
		for i := range tab.Rules {
			rule := &tab.Rules[i]
			rule.Value = strings.TrimSpace(rule.Value)
			if rule.Value == "" || rule.Field == types.RuleExtension && len(ruleExtensions(rule.Value)) == 0 {
				return errors.New(locales.ListRuleIncomplete())
			}
		}
	} else {
		tab.Rules = nil
	}
	creating := tab.ID == ""
	if creating {
		tab.ID = store.NewID()
	}
	a.setSetting(func(s *types.Settings) {
		s.CustomTabs = slices.Clone(s.CustomTabs)
		if creating {
			s.CustomTabs = append(s.CustomTabs, tab)
		} else {
			for i := range s.CustomTabs {
				if s.CustomTabs[i].ID == tab.ID {
					s.CustomTabs[i] = tab
					break
				}
			}
		}
	})
	if creating {
		a.tab = len(a.tabs()) - 1
		a.query = ""
	}
	a.sel = 0
	a.vis = a.filtered()
	a.list.ScrollIntoView(0)
	a.strip.X = 0
	return nil
}

// deleteTab 删除列表入口并将普通列表中的记录保留在全部记录中
func (a *App) deleteTab(tab types.Tab) {
	if tab.ID == "all" {
		return
	}
	selected := a.currentTab().ID
	a.setSetting(func(s *types.Settings) {
		s.CustomTabs = slices.DeleteFunc(slices.Clone(s.CustomTabs), func(t types.Tab) bool { return t.ID == tab.ID })
	})
	if a.st != nil {
		for _, r := range a.records {
			if r.ListID == tab.ID {
				a.st.Update(r.ID, func(r *types.Record) { r.ListID = "" })
			}
		}
		a.reload()
	}
	a.tab = 0
	for i, remaining := range a.tabs() {
		if remaining.ID == selected {
			a.tab = i
			break
		}
	}
	a.sel = 0
	a.vis = a.filtered()
	a.list.ScrollIntoView(0)
	a.strip.X = 0
}

// tabMenu 提供用户列表的编辑与删除操作
func (a *App) tabMenu(m *ui.Menu, tab types.Tab) {
	if m.Item(locales.EditList()).Chosen() {
		a.later(func() { a.editTab(tab) })
	}
	if m.Item(locales.DeleteList()).Chosen() {
		a.later(func() { a.deleteTab(tab) })
	}
}

// listDropTarget 高亮普通列表的拖入状态并在放下记录后移动归属
func (a *App) listDropTarget(chip ui.Element, p palette, tab types.Tab) {
	if tab.ID == "all" || tab.Smart {
		return
	}
	if r, over := ui.DragOver[*types.Record](chip); over && r.ListID != tab.ID {
		chip.Background(p.accentSoft).InsetShadow(0, 0, 0, 2, p.accentLine)
	}
	if r, dropped := ui.Drop[*types.Record](chip); dropped && r.ListID != tab.ID {
		a.later(func() { a.moveToList(r, tab) })
	}
}

// moveToList 将记录移入指定普通列表或从普通列表移出
func (a *App) moveToList(r *types.Record, tab types.Tab) {
	if a.st == nil {
		return
	}
	id := tab.ID
	if id == "all" {
		id = ""
	}
	if _, ok, err := a.st.UpdatePreview(r.ID, nil, "", func(r *types.Record) { r.ListID = id }); err != nil {
		a.storageError(err)
		return
	} else if !ok {
		return
	}
	a.reload()
	a.vis = a.filtered()
	a.clampSel()
	a.notify(locales.MovedToList(tab.Name))
}

// listRules 显示智能列表的匹配方式、规则编辑与增删入口
func (a *App) listRules(c *ui.Context, ps *promptState) {
	mode := 0
	if ps.tab.MatchAny {
		mode = 1
	}
	ui.Segmented(c, &mode, locales.MatchAll(), locales.MatchAny()).Label(locales.MatchMode())
	ps.tab.MatchAny = mode == 1
	remove := -1
	for i := range ps.tab.Rules {
		rule := &ps.tab.Rules[i]
		ui.Row(c).Key(i).Gap(6).AlignItems(ui.Center).Children(func() {
			fields := []types.RuleField{types.RuleKind, types.RuleExtension, types.RuleKeyword}
			labels := []string{locales.RuleKind(), locales.RuleExtension(), locales.RuleKeyword()}
			choice := labels[slices.Index(fields, rule.Field)]
			ui.Select(c, &choice, labels).Width(112).Label(locales.RuleTypeLabel(i + 1))
			if field := fields[slices.Index(labels, choice)]; field != rule.Field {
				rule.Field, rule.Value = field, ""
				if field == types.RuleKind {
					rule.Value = string(types.KindText)
				}
			}
			if rule.Field == types.RuleKind {
				kinds := []types.Kind{types.KindText, types.KindImage, types.KindLink, types.KindColor, types.KindFile}
				labels := []string{locales.TextKind(), locales.ImageKind(), locales.LinkKind(), locales.ColorKind(), locales.FileKind()}
				kind := labels[slices.Index(kinds, types.Kind(rule.Value))]
				ui.Select(c, &kind, labels).Grow(1).Label(locales.RuleValueLabel(i + 1))
				rule.Value = string(kinds[slices.Index(labels, kind)])
			} else {
				placeholder := locales.Keyword()
				if rule.Field == types.RuleExtension {
					placeholder = locales.ExtensionExample()
				}
				ui.TextInput(c, &rule.Value).Grow(1).Placeholder(placeholder).Label(locales.RuleValueLabel(i + 1))
			}
			if ui.Button(c, "×").Label(locales.RemoveRuleLabel(i + 1)).Clicked() {
				remove = i
			}
		})
	}
	if remove >= 0 {
		ps.tab.Rules = slices.Delete(ps.tab.Rules, remove, remove+1)
	}
	if ui.Button(c, locales.AddRule()).Clicked() {
		ps.tab.Rules = append(ps.tab.Rules, types.ListRule{Field: types.RuleKeyword})
	}
}
