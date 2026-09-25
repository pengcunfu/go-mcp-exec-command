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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------- 工具参数解析 ----------
//
// MCP 客户端传入的参数是未定型的 JSON，这里统一做类型转换与默认值处理，
// 并对数字、布尔等常见宽松写法（如 "true"、"10"）保持兼容。

func argString(args map[string]interface{}, key, def string) (string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return def, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("参数 %q 必须是字符串", key)
	}
	return text, nil
}

func requireString(args map[string]interface{}, key string) (string, error) {
	text, err := argString(args, key, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("缺少必填参数 %q", key)
	}
	return text, nil
}

func argInt(args map[string]interface{}, key string, def int) (int, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return def, nil
	}
	switch typed := value.(type) {
	case float64:
		return int(typed), nil
	case string:
		number, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("参数 %q 必须是整数", key)
		}
		return number, nil
	}
	return 0, fmt.Errorf("参数 %q 必须是整数", key)
}

func argBool(args map[string]interface{}, key string, def bool) (bool, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return def, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, fmt.Errorf("参数 %q 必须是布尔值", key)
		}
		return parsed, nil
	}
	return false, fmt.Errorf("参数 %q 必须是布尔值", key)
}

// argStringList 接受字符串数组，也接受逗号分隔的字符串。
func argStringList(args map[string]interface{}, key string) ([]string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok {
		return splitList(text), nil
	}
	items, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("参数 %q 必须是字符串数组", key)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("参数 %q 必须是字符串数组", key)
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}

func splitList(text string) []string {
	parts := strings.Split(text, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ---------- 路径处理 ----------

// resolvePath 将用户提供的路径（支持 ~ 与相对路径）解析为绝对路径。
func resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("无法解析用户主目录: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("无法解析路径 %q: %w", path, err)
	}
	return absolute, nil
}

// relPath 返回统一使用 / 分隔的相对路径，便于跨平台输出。
func relPath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func wrapPathError(path string, err error) error {
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("路径不存在: %s", path)
	case os.IsPermission(err):
		return fmt.Errorf("没有权限访问: %s", path)
	}
	return fmt.Errorf("访问路径失败 %s: %w", path, err)
}

func currentUsername() string {
	if username := os.Getenv("USER"); username != "" {
		return username
	}
	return os.Getenv("USERNAME")
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

// ---------- 外部命令调用 ----------

// runCommand 执行外部命令并只返回标准输出，标准错误用于补充错误信息。
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return "", commandError(name, err, stderr.String())
	}
	return string(output), nil
}

func commandError(name string, err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	// PowerShell 在非交互模式下会向 stderr 写入 CLIXML 进度记录，这里剔除。
	if index := strings.Index(stderr, "#< CLIXML"); index >= 0 {
		stderr = strings.TrimSpace(stderr[:index])
	}
	if stderr == "" {
		return fmt.Errorf("%s 执行失败: %w", name, err)
	}
	return fmt.Errorf("%s 执行失败: %w (%s)", name, err, truncate(stderr, 300))
}

// powershellPrelude 统一 PowerShell 的输出编码与进度输出。
//
// Windows 上 PowerShell 默认按 OEM 代码页输出，中文等非 ASCII 内容会变成乱码，
// 这里强制以 UTF-8 输出，保证调用方始终拿到 UTF-8 文本。
const powershellPrelude = "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;$ProgressPreference='SilentlyContinue';"

// runPowerShell 执行 PowerShell 脚本，屏蔽进度输出以保证 stdout 只包含脚本结果。
func runPowerShell(script string) (string, error) {
	script = "$ErrorActionPreference='Stop';" + powershellPrelude + script
	return runCommand("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
}
