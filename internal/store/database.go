package store

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/plugins/sqlite"

	"gopaste/internal/types"
)

// openDatabase 创建数据库并将旧版历史与设置一次性原子迁移
func (s *Store) openDatabase() (err error) {
	ctx := context.Background()
	s.db, err = sqlite.Open(ctx, filepath.Join(s.dir, databaseFile), sqlite.OpenOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			s.db.Close()
		}
	}()
	rows, err := s.db.Query(ctx, "PRAGMA user_version")
	if err != nil {
		return err
	}
	version := rows.Values[0][0].(int64)
	if version > 2 {
		return errors.New("store: unsupported database version")
	}
	if _, err = s.db.Execute(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	var statements []sqlite.Statement
	if version == 0 {
		statements = []sqlite.Statement{
			{SQL: `CREATE TABLE records (id TEXT PRIMARY KEY, position INTEGER NOT NULL, kind TEXT NOT NULL, list_id TEXT NOT NULL, meta TEXT NOT NULL, summary TEXT NOT NULL, payload TEXT NOT NULL, search TEXT NOT NULL, extensions TEXT NOT NULL)`},
			{SQL: "CREATE INDEX records_order ON records(position DESC)"},
			{SQL: "CREATE INDEX records_list ON records(list_id, position DESC)"},
		}
		data, readErr := os.ReadFile(filepath.Join(s.dir, indexFile))
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		if readErr == nil {
			var idx index
			if err = json.Unmarshal(data, &idx); err != nil {
				return err
			}
			if idx.Version != 1 {
				return errors.New("store: unsupported history index")
			}
			for i, r := range idx.Records {
				if r == nil || r.ID == "" {
					continue
				}
				if r.Hash == "" {
					r.Hash = recordHash(r)
				}
				if r.PreviewState == "loading" {
					r.PreviewState = ""
				}
				statement, e := recordStatement(r, int64(len(idx.Records)-i))
				if e != nil {
					return e
				}
				statements = append(statements, statement)
			}
		}
	}
	if version < 2 {
		data, readErr := os.ReadFile(filepath.Join(s.dir, legacySettingsFile))
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		if readErr == nil {
			s.settings.ExcludedAppsInitialized = false
			if err = json.Unmarshal(data, &s.settings); err != nil {
				return err
			}
		}
		normalizeSettings(&s.settings)
		statement, e := settingsStatement(s.settings)
		if e != nil {
			return e
		}
		statements = append(statements,
			sqlite.Statement{SQL: "CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK(id=1), data TEXT NOT NULL)"},
			statement,
			sqlite.Statement{SQL: "PRAGMA user_version=2"},
		)
		if _, err = s.db.Transaction(ctx, statements); err != nil {
			return err
		}
	}
	rows, err = s.db.Query(ctx, "SELECT data FROM settings WHERE id=1")
	if err != nil {
		return err
	}
	if len(rows.Values) == 0 {
		return errors.New("store: missing settings")
	}
	if err = json.Unmarshal([]byte(rows.Values[0][0].(string)), &s.settings); err != nil {
		return err
	}
	normalizeSettings(&s.settings)
	// 迁移提交后移除旧文件，重启时以数据库为准
	for _, name := range []string{indexFile, legacySettingsFile} {
		if err = os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	rows, err = s.db.Query(ctx, "SELECT meta, position FROM records ORDER BY position DESC")
	if err != nil {
		return err
	}
	s.positions = make(map[string]int64, len(rows.Values))
	for _, row := range rows.Values {
		r := new(types.Record)
		if err = json.Unmarshal([]byte(row[0].(string)), r); err != nil {
			return err
		}
		s.records = append(s.records, r)
		s.byID[r.ID] = r
		s.positions[r.ID] = row[1].(int64)
	}
	// 中断的链接任务允许下次启动重新调度
	for _, r := range slices.Clone(s.records) {
		if r.PreviewState == "loading" {
			full, e := s.getLocked(r.ID)
			if e != nil {
				return e
			}
			full.PreviewState = ""
			next := slices.Clone(s.records)
			for i, item := range next {
				if item.ID == r.ID {
					next[i] = full
				}
			}
			if err = s.commitLocked(next, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// settingsStatement 生成整份应用设置的原子保存语句
func settingsStatement(settings types.Settings) (sqlite.Statement, error) {
	data, err := json.Marshal(settings)
	if err != nil {
		return sqlite.Statement{}, err
	}
	return sqlite.Statement{
		SQL:  "INSERT INTO settings(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data",
		Args: []any{string(data)},
	}, nil
}

// recordMeta 提取去重、排序和资源清理所需的常驻元数据
func recordMeta(r *types.Record) *types.Record {
	return &types.Record{ID: r.ID, Kind: r.Kind, CreatedAt: r.CreatedAt, ListID: r.ListID, Hash: r.Hash, Image: r.Image, PreviewImage: r.PreviewImage, PreviewState: r.PreviewState, Source: types.Source{Icon: r.Source.Icon}}
}

// recordStatement 生成单条记录及其有限摘要和大小写无关检索字段
func recordStatement(r *types.Record, position int64) (sqlite.Statement, error) {
	payload, err := json.Marshal(r)
	if err != nil {
		return sqlite.Statement{}, err
	}
	meta, err := json.Marshal(recordMeta(r))
	if err != nil {
		return sqlite.Statement{}, err
	}
	summary := *r
	summary.Text = summaryText(r.Text)
	summary.HTML = ""
	summary.Title = summaryText(r.Title)
	summary.URL = summaryText(r.URL)
	summary.FileCount = len(r.Files)
	summary.Files = r.Files[:min(3, len(r.Files))]
	preview, err := json.Marshal(summary)
	if err != nil {
		return sqlite.Statement{}, err
	}
	fields := []string{r.Text, r.Title, r.URL, r.Color, r.Source.AppName}
	extensions := []string{}
	if r.Kind == types.KindImage {
		extensions = append(extensions, strings.ToLower(r.ImageFormat))
		if r.ImageFormat == "jpeg" {
			extensions = append(extensions, "jpg")
		}
	}
	for _, f := range r.Files {
		fields = append(fields, f.Name, f.Path)
		if !f.Dir {
			// 保存所有点号后缀以保留 tar.gz 等复合扩展名规则
			name := strings.ToLower(f.Name)
			for i, c := range name {
				if c == '.' && i+1 < len(name) {
					extensions = append(extensions, name[i+1:])
				}
			}
		}
	}
	for i := range fields {
		fields[i] = strings.ToLower(fields[i])
	}
	search, _ := json.Marshal(fields)
	exts, _ := json.Marshal(extensions)
	return sqlite.Statement{SQL: `INSERT INTO records(id,position,kind,list_id,meta,summary,payload,search,extensions) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET position=excluded.position,kind=excluded.kind,list_id=excluded.list_id,meta=excluded.meta,summary=excluded.summary,payload=excluded.payload,search=excluded.search,extensions=excluded.extensions`, Args: []any{r.ID, position, string(r.Kind), r.ListID, string(meta), string(preview), string(payload), string(search), string(exts)}}, nil
}

// summaryText 截取卡片所需的前五百一十二个字符并保留截断提示
func summaryText(text string) string {
	text = strings.TrimSpace(text)
	n := 0
	for i := range text {
		if n == 512 {
			return strings.Clone(text[:i]) + "…"
		}
		n++
	}
	return text
}

// readRecordsLocked 从指定内容列读取记录
func (s *Store) readRecordsLocked(column, clause string, args ...any) ([]*types.Record, error) {
	rows, err := s.db.Query(context.Background(), "SELECT "+column+" FROM records "+clause, args...)
	if err != nil {
		return nil, err
	}
	records := make([]*types.Record, 0, len(rows.Values))
	for _, row := range rows.Values {
		r := new(types.Record)
		if err := json.Unmarshal([]byte(row[0].(string)), r); err != nil {
			return nil, err
		}
		r.Summary = column == "summary"
		records = append(records, r)
	}
	return records, nil
}

// commitLocked 原子保存附加语句、记录与顺序，成功后更新元数据并回收资源
func (s *Store) commitLocked(next []*types.Record, candidates map[string]int64, statements ...sqlite.Statement) error {
	if candidates == nil {
		candidates = make(map[string]int64)
	}
	for name := range references(s.records) {
		candidates[name] = 0
	}
	positions := make(map[string]int64, len(next))
	var top int64
	for _, p := range s.positions {
		top = max(top, p)
	}
	kept := make(map[string]*types.Record, len(next))
	metadata := make([]*types.Record, 0, len(next))
	for i, r := range next {
		position := s.positions[r.ID]
		if i == 0 && (len(s.records) == 0 || s.records[0].ID != r.ID) {
			position = top + 1
		}
		if s.byID[r.ID] != r {
			statement, err := recordStatement(r, position)
			if err != nil {
				return err
			}
			statements = append(statements, statement)
			r = recordMeta(r)
		} else if position != s.positions[r.ID] {
			statements = append(statements, sqlite.Statement{SQL: "UPDATE records SET position=? WHERE id=?", Args: []any{position, r.ID}})
		}
		positions[r.ID] = position
		kept[r.ID] = r
		metadata = append(metadata, r)
	}
	for id := range s.byID {
		if kept[id] == nil {
			statements = append(statements, sqlite.Statement{SQL: "DELETE FROM records WHERE id=?", Args: []any{id}})
		}
	}
	if len(statements) > 0 {
		if _, err := s.db.Transaction(context.Background(), statements); err != nil {
			return err
		}
	}
	s.records, s.byID, s.positions = metadata, kept, positions
	s.removeUnusedLocked(candidates)
	return nil
}

// IDs 返回符合搜索和列表规则的有序记录标识
func (s *Store) IDs(tab types.Tab, query string) ([]*types.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	where, args := queryClause(tab, query)
	rows, err := s.db.Query(context.Background(), "SELECT id FROM records WHERE "+where+" ORDER BY position DESC", args...)
	if err != nil {
		return nil, err
	}
	records := make([]*types.Record, 0, len(rows.Values))
	for _, row := range rows.Values {
		records = append(records, &types.Record{ID: row[0].(string)})
	}
	return records, nil
}

// Summaries 按指定标识批量读取当前页的卡片摘要
func (s *Store) Summaries(ids []string) ([]*types.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return s.readRecordsLocked("summary", "WHERE id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")", args...)
}

// PendingLinks 返回尚未获取预览的完整链接记录
func (s *Store) PendingLinks() ([]*types.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readRecordsLocked("payload", `WHERE kind='link' AND coalesce(json_extract(meta,'$.previewState'),'')=''`)
}

// queryClause 使用参数化 SQL 保留 Unicode 子串和智能列表匹配规则
func queryClause(tab types.Tab, query string) (string, []any) {
	conditions := []string{"1"}
	var args []any
	keyword := func(value string) string {
		args = append(args, strings.ToLower(value))
		return "EXISTS (SELECT 1 FROM json_each(search) WHERE instr(value,?)>0)"
	}
	if tab.ID != "all" {
		if !tab.Smart {
			conditions = append(conditions, "list_id=?")
			args = append(args, tab.ID)
		} else {
			rules := []string{}
			for _, rule := range tab.Rules {
				switch rule.Field {
				case types.RuleKind:
					rules = append(rules, "kind=?")
					args = append(args, rule.Value)
				case types.RuleKeyword:
					rules = append(rules, keyword(rule.Value))
				case types.RuleExtension:
					exts := []string{}
					for _, ext := range strings.FieldsFunc(strings.ToLower(rule.Value), func(r rune) bool { return r == ',' || r == '，' || unicode.IsSpace(r) }) {
						ext = strings.TrimPrefix(ext, ".")
						if ext == "" {
							continue
						}
						exts = append(exts, "value=?")
						args = append(args, ext)
					}
					if len(exts) == 0 {
						rules = append(rules, "0")
					} else {
						rules = append(rules, "EXISTS (SELECT 1 FROM json_each(extensions) WHERE "+strings.Join(exts, " OR ")+")")
					}
				default:
					rules = append(rules, "0")
				}
			}
			join := " AND "
			if tab.MatchAny {
				join = " OR "
			}
			if len(rules) == 0 {
				conditions = append(conditions, "0")
			} else {
				conditions = append(conditions, "("+strings.Join(rules, join)+")")
			}
		}
	}
	if q := strings.TrimSpace(query); q != "" {
		conditions = append(conditions, keyword(q))
	}
	return strings.Join(conditions, " AND "), args
}
