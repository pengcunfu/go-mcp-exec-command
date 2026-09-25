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

import "testing"

func TestDetectEncoding(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		expected textEncoding
	}{
		{"纯 ASCII 视为 UTF-8", []byte("hello"), encodingUTF8},
		{"UTF-8 BOM", []byte{0xEF, 0xBB, 0xBF, 'h', 'i'}, encodingUTF8BOM},
		{"UTF-16LE BOM", []byte{0xFF, 0xFE, 'h', 0x00}, encodingUTF16LE},
		{"UTF-16BE BOM", []byte{0xFE, 0xFF, 0x00, 'h'}, encodingUTF16BE},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := detectEncoding(testCase.data); actual != testCase.expected {
				t.Fatalf("detectEncoding = %v, 期望 %v", actual, testCase.expected)
			}
		})
	}
}

func TestDecodeAndEncodeRoundTrip(t *testing.T) {
	content := "第一行\nsecond line\n"

	encodings := []textEncoding{encodingUTF8, encodingUTF8BOM, encodingUTF16LE, encodingUTF16BE}
	for _, encoding := range encodings {
		t.Run(encoding.String(), func(t *testing.T) {
			data := encodeText(content, encoding)

			if actual := detectEncoding(data); actual != encoding {
				t.Fatalf("写入后 detectEncoding = %v, 期望 %v", actual, encoding)
			}

			decoded, detected := decodeText(data)
			if detected != encoding {
				t.Fatalf("decodeText 编码 = %v, 期望 %v", detected, encoding)
			}
			if decoded != content {
				t.Fatalf("解码结果 = %q, 期望 %q", decoded, content)
			}
		})
	}
}

func TestLooksBinary(t *testing.T) {
	if looksBinary([]byte("普通文本内容")) {
		t.Fatal("普通文本被误判为二进制")
	}
	if !looksBinary([]byte{0x50, 0x4B, 0x00, 0x01}) {
		t.Fatal("含 NUL 的内容应判定为二进制")
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		content  string
		expected int
	}{
		{"", 0},
		{"one", 1},
		{"one\n", 1},
		{"one\ntwo\nthree\n", 3},
		{"one\ntwo", 2},
	}

	for _, testCase := range cases {
		if actual := len(splitLines(testCase.content)); actual != testCase.expected {
			t.Fatalf("splitLines(%q) 行数 = %d, 期望 %d", testCase.content, actual, testCase.expected)
		}
	}
}

func TestSelectLines(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}

	cases := []struct {
		name              string
		offset            int
		limit             int
		expectedStart     int
		expectedEnd       int
		expectedTruncated bool
		expectedCount     int
	}{
		{"完整读取", 1, 10, 1, 5, false, 5},
		{"从中间读取", 2, 2, 2, 3, true, 2},
		{"负数偏移取末尾", -2, 10, 4, 5, false, 2},
		{"偏移越界", 10, 10, 10, 5, false, 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			start, end, truncated, selected := selectLines(lines, testCase.offset, testCase.limit)
			if start != testCase.expectedStart || end != testCase.expectedEnd {
				t.Fatalf("区间 = [%d, %d], 期望 [%d, %d]", start, end, testCase.expectedStart, testCase.expectedEnd)
			}
			if truncated != testCase.expectedTruncated {
				t.Fatalf("truncated = %v, 期望 %v", truncated, testCase.expectedTruncated)
			}
			if len(selected) != testCase.expectedCount {
				t.Fatalf("返回行数 = %d, 期望 %d", len(selected), testCase.expectedCount)
			}
		})
	}
}

func TestNormalizeEOL(t *testing.T) {
	if actual := normalizeEOL("a\nb\n", "\r\n"); actual != "a\r\nb\r\n" {
		t.Fatalf("转换为 CRLF 失败: %q", actual)
	}
	if actual := normalizeEOL("a\r\nb\r\n", "\n"); actual != "a\nb\n" {
		t.Fatalf("转换为 LF 失败: %q", actual)
	}
	if actual := dominantEOL("a\r\nb\r\nc\n"); actual != "\r\n" {
		t.Fatalf("主换行符判定 = %q, 期望 CRLF", actual)
	}
	if actual := dominantEOL("a\nb\nc\n"); actual != "\n" {
		t.Fatalf("主换行符判定 = %q, 期望 LF", actual)
	}
}

func TestGlobPatternMatch(t *testing.T) {
	cases := []struct {
		pattern         string
		name            string
		relative        string
		expectedToMatch bool
	}{
		{"*.go", "main.go", "main.go", true},
		{"*.go", "main.txt", "main.txt", false},
		{"*.go", "main.go", "src/main.go", true},
		{"**/*_test.go", "a_test.go", "pkg/sub/a_test.go", true},
		{"**/*_test.go", "a.go", "pkg/sub/a.go", false},
		{"src/**/*.go", "config.go", "src/app/config.go", true},
		{"src/**/*.go", "config.go", "vendor/config.go", false},
		{"?ain.go", "main.go", "main.go", true},
		{"[mM]ain.go", "Main.go", "Main.go", true},
	}

	for _, testCase := range cases {
		matcher := newGlobPattern(testCase.pattern)
		if actual := matcher.match(testCase.name, testCase.relative); actual != testCase.expectedToMatch {
			t.Fatalf("pattern %q 匹配 %q（相对 %q）= %v, 期望 %v",
				testCase.pattern, testCase.name, testCase.relative, actual, testCase.expectedToMatch)
		}
	}
}

func TestArgHelpers(t *testing.T) {
	args := map[string]interface{}{
		"text":   "value",
		"num":    float64(7),
		"flag":   true,
		"list":   []interface{}{"a", "b"},
		"csv":    "x, y ,z",
		"quoted": "12",
	}

	if value, _ := argString(args, "text", "default"); value != "value" {
		t.Fatalf("argString = %q", value)
	}
	if value, _ := argString(args, "missing", "default"); value != "default" {
		t.Fatalf("argString 默认值 = %q", value)
	}
	if value, _ := argInt(args, "num", 0); value != 7 {
		t.Fatalf("argInt = %d", value)
	}
	if value, _ := argInt(args, "quoted", 0); value != 12 {
		t.Fatalf("argInt 解析字符串 = %d", value)
	}
	if value, _ := argBool(args, "flag", false); !value {
		t.Fatal("argBool 应为 true")
	}
	if value, _ := argStringList(args, "list"); len(value) != 2 {
		t.Fatalf("argStringList 数组 = %v", value)
	}
	if value, _ := argStringList(args, "csv"); len(value) != 3 || value[1] != "y" {
		t.Fatalf("argStringList 逗号分隔 = %v", value)
	}
	if _, err := argInt(args, "text", 0); err == nil {
		t.Fatal("非数字字符串应返回错误")
	}
	if value, err := argStringList(args, "missing"); err != nil || value != nil {
		t.Fatalf("缺失参数应返回 nil: %v, %v", value, err)
	}
}

func TestResolvePath(t *testing.T) {
	absolute, err := resolvePath("~")
	if err != nil {
		t.Fatalf("解析 ~ 失败: %v", err)
	}
	if absolute == "" || absolute == "~" {
		t.Fatalf("~ 未展开: %q", absolute)
	}

	if _, err := resolvePath("   "); err == nil {
		t.Fatal("空路径应返回错误")
	}
}
