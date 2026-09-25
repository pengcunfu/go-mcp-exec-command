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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, path, content string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	return path
}

func TestReadFilePagination(t *testing.T) {
	server := NewBashServer()
	file := writeFixture(t, filepath.Join(t.TempDir(), "sample.txt"), "line1\nline2\nline3\n")

	result := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{"path": file})
	if result.TotalLines != 3 || result.StartLine != 1 || result.EndLine != 3 || result.Truncated {
		t.Fatalf("默认读取结果异常: %+v", result)
	}
	if result.Content != "line1\nline2\nline3" {
		t.Fatalf("内容 = %q", result.Content)
	}
	if result.Encoding != "utf-8" {
		t.Fatalf("编码 = %q", result.Encoding)
	}

	paged := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{
		"path": file, "offset": 2, "limit": 1,
	})
	if paged.Content != "line2" || paged.StartLine != 2 || paged.EndLine != 2 || !paged.Truncated {
		t.Fatalf("分页读取结果异常: %+v", paged)
	}

	tail := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{
		"path": file, "offset": -1,
	})
	if tail.Content != "line3" || tail.StartLine != 3 {
		t.Fatalf("负数偏移结果异常: %+v", tail)
	}
}

func TestReadFileEncodings(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()

	utf16File := filepath.Join(directory, "utf16.txt")
	if err := os.WriteFile(utf16File, encodeText("中文内容\n", encodingUTF16LE), 0o644); err != nil {
		t.Fatalf("写入 UTF-16 文件失败: %v", err)
	}
	utf16Result := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{"path": utf16File})
	if utf16Result.Encoding != "utf-16le" || utf16Result.Content != "中文内容" {
		t.Fatalf("UTF-16 读取结果异常: %+v", utf16Result)
	}

	binaryFile := filepath.Join(directory, "binary.bin")
	if err := os.WriteFile(binaryFile, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("写入二进制文件失败: %v", err)
	}
	binaryResult := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{"path": binaryFile})
	if !binaryResult.Binary || binaryResult.Content != "" {
		t.Fatalf("二进制文件应只返回元信息: %+v", binaryResult)
	}
}

func TestReadFileErrors(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()

	message := expectToolError(t, server, "read_file", map[string]interface{}{
		"path": filepath.Join(directory, "missing.txt"),
	})
	if !strings.Contains(message, "不存在") {
		t.Fatalf("错误信息未提示路径不存在: %s", message)
	}

	expectToolError(t, server, "read_file", map[string]interface{}{"path": directory})
	expectToolError(t, server, "read_file", map[string]interface{}{})
}

func TestWriteFile(t *testing.T) {
	server := NewBashServer()
	target := filepath.Join(t.TempDir(), "nested", "deep", "output.txt")

	result := callToolResult[FileWriteResult](t, server, "write_file", map[string]interface{}{
		"path":    target,
		"content": "hello world",
	})
	if !result.Created || result.BytesWritten != len("hello world") {
		t.Fatalf("写入结果异常: %+v", result)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "hello world" {
		t.Fatalf("文件内容不符: %q, %v", string(data), err)
	}

	overwrite := callToolResult[FileWriteResult](t, server, "write_file", map[string]interface{}{
		"path":    target,
		"content": "replaced",
	})
	if overwrite.Created {
		t.Fatal("覆盖写入不应标记为新建")
	}

	expectToolError(t, server, "write_file", map[string]interface{}{"path": target})
	expectToolError(t, server, "write_file", map[string]interface{}{"path": t.TempDir(), "content": "x"})
}

func TestWriteFileKeepsEncoding(t *testing.T) {
	server := NewBashServer()
	target := filepath.Join(t.TempDir(), "legacy.txt")

	if err := os.WriteFile(target, encodeText("初始内容", encodingUTF16LE), 0o644); err != nil {
		t.Fatalf("写入初始文件失败: %v", err)
	}

	result := callToolResult[FileWriteResult](t, server, "write_file", map[string]interface{}{
		"path":    target,
		"content": "新的内容",
	})
	if result.Encoding != "utf-16le" {
		t.Fatalf("应保持原有编码, 实际 = %q", result.Encoding)
	}

	readBack := callToolResult[FileReadResult](t, server, "read_file", map[string]interface{}{"path": target})
	if readBack.Encoding != "utf-16le" || readBack.Content != "新的内容" {
		t.Fatalf("回读结果异常: %+v", readBack)
	}
}

func TestEditFile(t *testing.T) {
	server := NewBashServer()

	t.Run("唯一匹配替换", func(t *testing.T) {
		file := writeFixture(t, filepath.Join(t.TempDir(), "code.go"), "func main() {\n\tprintln(1)\n}\n")

		result := callToolResult[FileEditResult](t, server, "edit_file", map[string]interface{}{
			"path": file, "old_string": "println(1)", "new_string": "println(2)",
		})
		if result.Replacements != 1 {
			t.Fatalf("替换次数 = %d, 期望 1", result.Replacements)
		}

		content, err := os.ReadFile(file)
		if err != nil || !strings.Contains(string(content), "println(2)") {
			t.Fatalf("文件内容未更新: %q, %v", string(content), err)
		}
	})

	t.Run("多处匹配需要显式声明", func(t *testing.T) {
		file := writeFixture(t, filepath.Join(t.TempDir(), "dup.txt"), "alpha\nbeta\nalpha\n")

		message := expectToolError(t, server, "edit_file", map[string]interface{}{
			"path": file, "old_string": "alpha", "new_string": "gamma",
		})
		if !strings.Contains(message, "replace_all") {
			t.Fatalf("错误信息应提示 replace_all: %s", message)
		}

		result := callToolResult[FileEditResult](t, server, "edit_file", map[string]interface{}{
			"path": file, "old_string": "alpha", "new_string": "gamma", "replace_all": true,
		})
		if result.Replacements != 2 {
			t.Fatalf("替换次数 = %d, 期望 2", result.Replacements)
		}
	})

	t.Run("兼容换行差异", func(t *testing.T) {
		file := writeFixture(t, filepath.Join(t.TempDir(), "crlf.txt"), "first\r\nsecond\r\n")

		result := callToolResult[FileEditResult](t, server, "edit_file", map[string]interface{}{
			"path": file, "old_string": "second\n", "new_string": "changed\n",
		})
		if result.Replacements != 1 {
			t.Fatalf("替换次数 = %d, 期望 1", result.Replacements)
		}

		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取文件失败: %v", err)
		}
		if string(content) != "first\r\nchanged\r\n" {
			t.Fatalf("换行风格未保持 CRLF: %q", string(content))
		}
	})

	t.Run("匹配失败与参数缺失", func(t *testing.T) {
		file := writeFixture(t, filepath.Join(t.TempDir(), "none.txt"), "content\n")

		message := expectToolError(t, server, "edit_file", map[string]interface{}{
			"path": file, "old_string": "not-there", "new_string": "x",
		})
		if !strings.Contains(message, "未在") {
			t.Fatalf("错误信息未提示未匹配: %s", message)
		}

		expectToolError(t, server, "edit_file", map[string]interface{}{"path": file, "new_string": "x"})
		expectToolError(t, server, "edit_file", map[string]interface{}{"path": file, "old_string": "", "new_string": "x"})
	})
}

func TestListDir(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()

	writeFixture(t, filepath.Join(directory, "visible.txt"), "a")
	writeFixture(t, filepath.Join(directory, ".hidden"), "b")
	if err := os.Mkdir(filepath.Join(directory, "sub"), 0o755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}

	visible := callToolResult[DirListResult](t, server, "list_dir", map[string]interface{}{"path": directory})
	if visible.Count != 2 {
		t.Fatalf("默认不应包含隐藏项, 实际数量 = %d", visible.Count)
	}

	all := callToolResult[DirListResult](t, server, "list_dir", map[string]interface{}{
		"path": directory, "show_hidden": true,
	})
	if all.Count != 3 {
		t.Fatalf("包含隐藏项时数量 = %d, 期望 3", all.Count)
	}

	var foundDir bool
	for _, entry := range all.Entries {
		if entry.Name == "sub" && entry.Type == "dir" {
			foundDir = true
		}
	}
	if !foundDir {
		t.Fatalf("未找到目录条目: %+v", all.Entries)
	}

	expectToolError(t, server, "list_dir", map[string]interface{}{"path": filepath.Join(directory, "visible.txt")})
}

func TestDirectoryTree(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()

	writeFixture(t, filepath.Join(directory, "a", "x.go"), "package a")
	writeFixture(t, filepath.Join(directory, "b.txt"), "b")
	writeFixture(t, filepath.Join(directory, ".secret", "hidden.txt"), "hidden")

	result := callToolResult[DirectoryTreeResult](t, server, "directory_tree", map[string]interface{}{
		"path": directory, "depth": 2,
	})
	if result.Dirs != 1 || result.Files != 2 {
		t.Fatalf("统计结果异常: dirs=%d files=%d", result.Dirs, result.Files)
	}
	for _, expected := range []string{"a/", "x.go", "b.txt"} {
		if !strings.Contains(result.Tree, expected) {
			t.Fatalf("树形输出缺少 %q:\n%s", expected, result.Tree)
		}
	}
	if strings.Contains(result.Tree, "hidden.txt") {
		t.Fatalf("默认不应包含隐藏目录:\n%s", result.Tree)
	}

	flat := callToolResult[DirectoryTreeResult](t, server, "directory_tree", map[string]interface{}{
		"path": directory, "depth": 1,
	})
	if strings.Contains(flat.Tree, "x.go") {
		t.Fatalf("depth=1 不应展开子目录:\n%s", flat.Tree)
	}
}

func TestSearchFiles(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()

	writeFixture(t, filepath.Join(directory, "main.go"), "package main")
	writeFixture(t, filepath.Join(directory, "util.go"), "package main")
	writeFixture(t, filepath.Join(directory, "sub", "a_test.go"), "package sub")
	writeFixture(t, filepath.Join(directory, "node_modules", "dep.go"), "package dep")
	writeFixture(t, filepath.Join(directory, ".git", "config"), "[core]")

	byName := callToolResult[SearchFilesResult](t, server, "search_files", map[string]interface{}{
		"path": directory, "pattern": "*.go",
	})
	if byName.Count != 4 {
		t.Fatalf("*.go 匹配数量 = %d, 期望 4: %+v", byName.Count, byName.Matches)
	}
	for _, match := range byName.Matches {
		if strings.HasPrefix(match.Relative, ".git/") {
			t.Fatalf("不应进入隐藏目录: %+v", match)
		}
	}

	excluded := callToolResult[SearchFilesResult](t, server, "search_files", map[string]interface{}{
		"path": directory, "pattern": "*.go", "exclude": []interface{}{"node_modules"},
	})
	if excluded.Count != 3 {
		t.Fatalf("排除 node_modules 后数量 = %d, 期望 3", excluded.Count)
	}

	recursive := callToolResult[SearchFilesResult](t, server, "search_files", map[string]interface{}{
		"path": directory, "pattern": "**/*_test.go",
	})
	if recursive.Count != 1 || recursive.Matches[0].Relative != "sub/a_test.go" {
		t.Fatalf("** 匹配结果异常: %+v", recursive.Matches)
	}

	limited := callToolResult[SearchFilesResult](t, server, "search_files", map[string]interface{}{
		"path": directory, "pattern": "*.go", "max_results": 1,
	})
	if limited.Count != 1 || !limited.Truncated {
		t.Fatalf("max_results 限制未生效: %+v", limited)
	}

	expectToolError(t, server, "search_files", map[string]interface{}{"path": directory})
}

func TestGetFileInfo(t *testing.T) {
	server := NewBashServer()
	directory := t.TempDir()
	file := writeFixture(t, filepath.Join(directory, "info.txt"), "12345")

	fileInfo := callToolResult[FileInfoResult](t, server, "get_file_info", map[string]interface{}{"path": file})
	if fileInfo.Type != "file" || fileInfo.Size != 5 {
		t.Fatalf("文件信息异常: %+v", fileInfo)
	}
	if fileInfo.Modified == "" || fileInfo.IsSymlink {
		t.Fatalf("文件信息字段异常: %+v", fileInfo)
	}

	directoryInfo := callToolResult[FileInfoResult](t, server, "get_file_info", map[string]interface{}{"path": directory})
	if directoryInfo.Type != "dir" {
		t.Fatalf("目录信息异常: %+v", directoryInfo)
	}
}
