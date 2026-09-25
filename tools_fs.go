/*
Copyright 2025 pengcunfu

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultReadLimit = 1000
	maxReadLimit     = 5000

	defaultTreeDepth = 3
	maxTreeDepth     = 10
	maxTreeEntries   = 2000

	defaultSearchLimit = 100
	maxSearchLimit     = 1000
)

type FileReadResult struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Encoding   string `json:"encoding"`
	Binary     bool   `json:"binary"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Truncated  bool   `json:"truncated"`
	Content    string `json:"content"`
}

type FileWriteResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
	Created      bool   `json:"created"`
	Encoding     string `json:"encoding"`
}

type FileEditResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
	BytesWritten int    `json:"bytes_written"`
}

type DirEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified string `json:"modified,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
}

type DirListResult struct {
	Path    string     `json:"path"`
	Count   int        `json:"count"`
	Entries []DirEntry `json:"entries"`
}

type DirectoryTreeResult struct {
	Root      string `json:"root"`
	Tree      string `json:"tree"`
	Dirs      int    `json:"dirs"`
	Files     int    `json:"files"`
	Truncated bool   `json:"truncated"`
}

type SearchMatch struct {
	Path     string `json:"path"`
	Relative string `json:"relative"`
}

type SearchFilesResult struct {
	Root      string        `json:"root"`
	Pattern   string        `json:"pattern"`
	Count     int           `json:"count"`
	Truncated bool          `json:"truncated"`
	Matches   []SearchMatch `json:"matches"`
}

type FileInfoResult struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	Mode        string `json:"mode"`
	Permissions string `json:"permissions"`
	Modified    string `json:"modified"`
	IsSymlink   bool   `json:"is_symlink"`
	Target      string `json:"target,omitempty"`
}

func (s *BashServer) fileTools() []RegisteredTool {
	return []RegisteredTool{
		{
			Tool: Tool{
				Name:        "read_file",
				Description: "读取文本文件内容。自动识别并解码 UTF-8 / UTF-8 BOM / UTF-16LE / UTF-16BE，二进制文件只返回元信息不返回内容。支持按行分页，行号从 1 开始，offset 为负数时表示从文件末尾倒数。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "文件路径，支持 ~ 与相对路径",
						},
						"offset": map[string]interface{}{
							"type":        "integer",
							"description": "起始行号（从 1 开始，负数表示从末尾倒数，默认 1）",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "最多返回的行数（默认 1000，最大 5000）",
						},
					},
					"required": []string{"path"},
				},
			},
			Handler: s.handleReadFile,
		},
		{
			Tool: Tool{
				Name:        "write_file",
				Description: "写入文本文件，目录不存在时会自动创建。默认以 UTF-8 写入；如果目标文件原本是 UTF-8 BOM 或 UTF-16 编码，则保持原编码写回。原有文件权限会被保留。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "文件路径，支持 ~ 与相对路径",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "要写入的完整内容",
						},
						"create_dirs": map[string]interface{}{
							"type":        "boolean",
							"description": "父目录不存在时是否自动创建（默认 true）",
						},
					},
					"required": []string{"path", "content"},
				},
			},
			Handler: s.handleWriteFile,
		},
		{
			Tool: Tool{
				Name:        "edit_file",
				Description: "在文件中精确替换一段文本。匹配时会自动兼容 LF 与 CRLF 换行差异，写回时保持文件原有编码与换行风格。old_string 必须唯一匹配，除非设置 replace_all=true。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "文件路径，支持 ~ 与相对路径",
						},
						"old_string": map[string]interface{}{
							"type":        "string",
							"description": "要被替换的原文本，必须与文件内容完全一致（含缩进）",
						},
						"new_string": map[string]interface{}{
							"type":        "string",
							"description": "替换后的新文本，传空字符串表示删除",
						},
						"replace_all": map[string]interface{}{
							"type":        "boolean",
							"description": "是否替换全部匹配项（默认 false，仅替换唯一匹配）",
						},
					},
					"required": []string{"path", "old_string", "new_string"},
				},
			},
			Handler: s.handleEditFile,
		},
		{
			Tool: Tool{
				Name:        "list_dir",
				Description: "列出目录下的直接子项，按名称排序。返回类型、大小与修改时间，无需关心平台差异。Windows 的隐藏文件按文件属性识别，其他平台按点号前缀识别。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "目录路径，支持 ~ 与相对路径",
						},
						"show_hidden": map[string]interface{}{
							"type":        "boolean",
							"description": "是否包含隐藏项（默认 false）",
						},
					},
					"required": []string{"path"},
				},
			},
			Handler: s.handleListDir,
		},
		{
			Tool: Tool{
				Name:        "directory_tree",
				Description: "以树形文本展示目录结构，便于快速了解项目布局。符号链接不会被递归展开，权限不足的目录会被跳过。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "根目录路径，支持 ~ 与相对路径",
						},
						"depth": map[string]interface{}{
							"type":        "integer",
							"description": "递归深度，1 表示只列出直接子项（默认 3，最大 10）",
						},
						"show_hidden": map[string]interface{}{
							"type":        "boolean",
							"description": "是否包含隐藏项（默认 false）",
						},
					},
					"required": []string{"path"},
				},
			},
			Handler: s.handleDirectoryTree,
		},
		{
			Tool: Tool{
				Name:        "search_files",
				Description: "按 glob 通配符递归查找文件，支持 *、?、[] 与 ** 任意层级匹配；pattern 含路径分隔符时按相对路径匹配，否则只匹配文件名。遍历时默认跳过以 . 开头的目录，避免进入 .git 等目录。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "glob 通配符，如 *.go、**/*_test.go、src/**/config.*",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "搜索根目录，支持 ~ 与相对路径（默认当前目录）",
						},
						"exclude": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "排除的 glob 列表，如 [\"node_modules\", \"*.log\"]",
						},
						"show_hidden": map[string]interface{}{
							"type":        "boolean",
							"description": "是否进入以 . 开头的隐藏目录（默认 false）",
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "最多返回的匹配数量（默认 100，最大 1000）",
						},
					},
					"required": []string{"pattern"},
				},
			},
			Handler: s.handleSearchFiles,
		},
		{
			Tool: Tool{
				Name:        "get_file_info",
				Description: "获取单个文件或目录的元信息，包括类型、大小、权限位与修改时间。符号链接会返回其指向目标，不跟随解析。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "文件或目录路径，支持 ~ 与相对路径",
						},
					},
					"required": []string{"path"},
				},
			},
			Handler: s.handleGetFileInfo,
		},
	}
}

func (s *BashServer) handleReadFile(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}
	offset, err := argInt(args, "offset", 1)
	if err != nil {
		return nil, err
	}
	limit, err := argInt(args, "limit", defaultReadLimit)
	if err != nil {
		return nil, err
	}

	absolute, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("这是一个目录而不是文件: %s，列出目录内容请使用 list_dir", rawPath)
	}

	content, encoding, binary, err := loadTextFile(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}

	result := FileReadResult{
		Path:     absolute,
		Size:     info.Size(),
		Encoding: encoding.String(),
	}
	if binary {
		result.Binary = true
		return result, nil
	}

	lines := splitLines(content)
	start, end, truncated, selected := selectLines(lines, offset, limit)

	result.TotalLines = len(lines)
	result.StartLine = start
	result.EndLine = end
	result.Truncated = truncated
	result.Content = strings.Join(selected, "\n")
	return result, nil
}

// selectLines 按行号区间取出行，offset 为负数时从末尾倒数。
func selectLines(lines []string, offset, limit int) (start, end int, truncated bool, selected []string) {
	total := len(lines)
	if total == 0 {
		return 0, 0, false, nil
	}

	if offset < 0 {
		offset = total + offset + 1
	}
	if offset < 1 {
		offset = 1
	}
	if offset > total {
		return offset, total, false, nil
	}

	if limit <= 0 {
		limit = defaultReadLimit
	}
	if limit > maxReadLimit {
		limit = maxReadLimit
	}

	end = offset + limit - 1
	if end >= total {
		end = total
	} else {
		truncated = true
	}
	return offset, end, truncated, lines[offset-1 : end]
}

func (s *BashServer) handleWriteFile(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}
	if value, ok := args["content"]; !ok || value == nil {
		return nil, fmt.Errorf("缺少必填参数 %q", "content")
	}
	content, err := argString(args, "content", "")
	if err != nil {
		return nil, err
	}
	createDirs, err := argBool(args, "create_dirs", true)
	if err != nil {
		return nil, err
	}

	absolute, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(absolute); statErr == nil && info.IsDir() {
		return nil, fmt.Errorf("这是一个目录而不是文件: %s", rawPath)
	}

	if createDirs {
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			return nil, fmt.Errorf("创建父目录失败: %w", err)
		}
	}

	encoding := encodingUTF8
	permissions := os.FileMode(0o644)
	created := true
	if info, statErr := os.Stat(absolute); statErr == nil {
		created = false
		permissions = info.Mode().Perm()
		encoding = fileEncoding(absolute)
	}

	data := encodeText(content, encoding)
	if err := os.WriteFile(absolute, data, permissions); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	return FileWriteResult{
		Path:         absolute,
		BytesWritten: len(data),
		Created:      created,
		Encoding:     encoding.String(),
	}, nil
}

func (s *BashServer) handleEditFile(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}
	oldText, err := requireString(args, "old_string")
	if err != nil {
		return nil, err
	}
	newText, err := argString(args, "new_string", "")
	if err != nil {
		return nil, err
	}
	replaceAll, err := argBool(args, "replace_all", false)
	if err != nil {
		return nil, err
	}

	absolute, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("这是一个目录而不是文件: %s", rawPath)
	}

	content, encoding, binary, err := loadTextFile(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if binary {
		return nil, fmt.Errorf("二进制文件不支持文本替换: %s", rawPath)
	}

	// 统一到文件自身的换行风格，避免跨平台编辑时因 CRLF/LF 差异匹配失败
	eol := dominantEOL(content)
	search := normalizeEOL(oldText, eol)
	replacement := normalizeEOL(newText, eol)

	count := strings.Count(content, search)
	if count == 0 {
		return nil, fmt.Errorf("未在 %s 中找到 old_string，请确认内容与缩进完全一致", rawPath)
	}
	if count > 1 && !replaceAll {
		return nil, fmt.Errorf("old_string 在 %s 中匹配到 %d 处，请提供更精确的内容或设置 replace_all=true", rawPath, count)
	}

	replacements := 1
	updated := strings.Replace(content, search, replacement, 1)
	if replaceAll {
		replacements = count
		updated = strings.ReplaceAll(content, search, replacement)
	}

	data := encodeText(updated, encoding)
	if err := os.WriteFile(absolute, data, info.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	return FileEditResult{
		Path:         absolute,
		Replacements: replacements,
		BytesWritten: len(data),
	}, nil
}

func (s *BashServer) handleListDir(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}
	showHidden, err := argBool(args, "show_hidden", false)
	if err != nil {
		return nil, err
	}

	absolute, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("不是目录: %s，读取文件内容请使用 read_file", rawPath)
	}

	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}

	result := DirListResult{Path: absolute, Entries: []DirEntry{}}
	for _, entry := range entries {
		fullPath := filepath.Join(absolute, entry.Name())
		hidden := isHidden(entry.Name(), fullPath)
		if hidden && !showHidden {
			continue
		}

		item := DirEntry{
			Name:   entry.Name(),
			Path:   fullPath,
			Type:   entryType(entry),
			Hidden: hidden,
		}
		if entryInfo, infoErr := entry.Info(); infoErr == nil {
			item.Size = entryInfo.Size()
			item.Modified = entryInfo.ModTime().Format(time.RFC3339)
		}
		result.Entries = append(result.Entries, item)
	}
	result.Count = len(result.Entries)
	return result, nil
}

func (s *BashServer) handleDirectoryTree(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}
	depth, err := argInt(args, "depth", defaultTreeDepth)
	if err != nil {
		return nil, err
	}
	showHidden, err := argBool(args, "show_hidden", false)
	if err != nil {
		return nil, err
	}
	if depth < 1 {
		depth = 1
	}
	if depth > maxTreeDepth {
		depth = maxTreeDepth
	}

	root, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("不是目录: %s", rawPath)
	}

	result := &DirectoryTreeResult{Root: root, Tree: filepath.Base(root) + "/\n"}
	builder := &treeBuilder{result: result}
	s.walkTree(root, "", depth, showHidden, builder)

	if result.Truncated {
		result.Tree += fmt.Sprintf("... 结果已截断（超过 %d 项）\n", maxTreeEntries)
	}
	return result, nil
}

// treeBuilder 在递归过程中累积树形文本与统计信息。
type treeBuilder struct {
	result *DirectoryTreeResult
}

func (b *treeBuilder) count() int {
	return b.result.Dirs + b.result.Files
}

func (s *BashServer) walkTree(dir, prefix string, depth int, showHidden bool, builder *treeBuilder) {
	if depth <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// 目录优先展示，各自保持名称升序（os.ReadDir 已排序）
	dirs := make([]os.DirEntry, 0, len(entries))
	files := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !showHidden && isHidden(entry.Name(), filepath.Join(dir, entry.Name())) {
			continue
		}
		if entry.IsDir() {
			dirs = append(dirs, entry)
			continue
		}
		files = append(files, entry)
	}
	ordered := append(dirs, files...)

	for index, entry := range ordered {
		if builder.count() >= maxTreeEntries {
			builder.result.Truncated = true
			return
		}

		branch := "├── "
		childPrefix := prefix + "│   "
		if index == len(ordered)-1 {
			branch = "└── "
			childPrefix = prefix + "    "
		}

		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			builder.result.Dirs++
			builder.result.Tree += prefix + branch + entry.Name() + "/\n"
			s.walkTree(fullPath, childPrefix, depth-1, showHidden, builder)
			continue
		}

		builder.result.Files++
		line := prefix + branch + entry.Name()
		if entry.Type()&os.ModeSymlink != 0 {
			if target := linkTarget(fullPath); target != "" {
				line += " -> " + target
			}
		}
		builder.result.Tree += line + "\n"
	}
}

func (s *BashServer) handleSearchFiles(args map[string]interface{}) (interface{}, error) {
	patternText, err := requireString(args, "pattern")
	if err != nil {
		return nil, err
	}
	rawPath, err := argString(args, "path", ".")
	if err != nil {
		return nil, err
	}
	excludes, err := argStringList(args, "exclude")
	if err != nil {
		return nil, err
	}
	showHidden, err := argBool(args, "show_hidden", false)
	if err != nil {
		return nil, err
	}
	maxResults, err := argInt(args, "max_results", defaultSearchLimit)
	if err != nil {
		return nil, err
	}
	if maxResults < 1 {
		maxResults = defaultSearchLimit
	}
	if maxResults > maxSearchLimit {
		maxResults = maxSearchLimit
	}

	root, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("不是目录: %s", rawPath)
	}

	matcher := newGlobPattern(patternText)
	excludeMatchers := make([]globPattern, 0, len(excludes))
	for _, exclude := range excludes {
		excludeMatchers = append(excludeMatchers, newGlobPattern(exclude))
	}
	isExcluded := func(name, relative string) bool {
		for _, excludeMatcher := range excludeMatchers {
			if excludeMatcher.match(name, relative) {
				return true
			}
		}
		return false
	}

	result := SearchFilesResult{Root: root, Pattern: patternText, Matches: []SearchMatch{}}
	walkErr := filepath.WalkDir(root, func(currentPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if currentPath == root {
			return nil
		}

		name := entry.Name()
		relative := relPath(root, currentPath)

		if entry.IsDir() {
			// 统一跳过点号目录（如 .git），除非显式要求包含隐藏项
			if !showHidden && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if isExcluded(name, relative) {
				return fs.SkipDir
			}
			return nil
		}

		if !matcher.match(name, relative) || isExcluded(name, relative) {
			return nil
		}

		result.Matches = append(result.Matches, SearchMatch{Path: currentPath, Relative: relative})
		if len(result.Matches) >= maxResults {
			result.Truncated = true
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", walkErr)
	}

	result.Count = len(result.Matches)
	return result, nil
}

func (s *BashServer) handleGetFileInfo(args map[string]interface{}) (interface{}, error) {
	rawPath, err := requireString(args, "path")
	if err != nil {
		return nil, err
	}

	absolute, err := resolvePath(rawPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, wrapPathError(rawPath, err)
	}

	result := FileInfoResult{
		Path:        absolute,
		Name:        info.Name(),
		Type:        fileType(info.Mode()),
		Size:        info.Size(),
		Mode:        info.Mode().String(),
		Permissions: fmt.Sprintf("%04o", info.Mode().Perm()),
		Modified:    info.ModTime().Format(time.RFC3339),
		IsSymlink:   info.Mode()&os.ModeSymlink != 0,
	}
	if result.IsSymlink {
		result.Target = linkTarget(absolute)
	}
	return result, nil
}

// globPattern 是可跨平台的 glob 匹配器，支持 ** 表示任意层级目录。
type globPattern struct {
	segments []string
	nameOnly bool
}

func newGlobPattern(pattern string) globPattern {
	trimmed := strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(pattern)), "./")

	segments := make([]string, 0, 4)
	for _, segment := range strings.Split(trimmed, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	return globPattern{
		segments: segments,
		nameOnly: !strings.ContainsAny(trimmed, `/\`),
	}
}

func (g globPattern) match(name, relative string) bool {
	if len(g.segments) == 0 {
		return false
	}
	if g.nameOnly {
		return matchSegment(g.segments[0], name)
	}
	return matchSegments(g.segments, strings.Split(relative, "/"))
}

func matchSegments(pattern, parts []string) bool {
	if len(pattern) == 0 {
		return len(parts) == 0
	}
	if pattern[0] == "**" {
		for index := 0; index <= len(parts); index++ {
			if matchSegments(pattern[1:], parts[index:]) {
				return true
			}
		}
		return false
	}
	if len(parts) == 0 {
		return false
	}
	if !matchSegment(pattern[0], parts[0]) {
		return false
	}
	return matchSegments(pattern[1:], parts[1:])
}

func matchSegment(pattern, name string) bool {
	matched, err := path.Match(pattern, name)
	return err == nil && matched
}

func entryType(entry os.DirEntry) string {
	switch {
	case entry.Type()&os.ModeSymlink != 0:
		return "symlink"
	case entry.IsDir():
		return "dir"
	case entry.Type().IsRegular():
		return "file"
	}
	return "other"
}

func fileType(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "dir"
	case mode.IsRegular():
		return "file"
	}
	return "other"
}

func linkTarget(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}
