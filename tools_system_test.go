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
	"runtime"
	"strings"
	"testing"
)

func TestGetEnvMasking(t *testing.T) {
	server := NewBashServer()
	t.Setenv("MCP_TEST_TOKEN", "supersecret")

	masked := callToolResult[EnvResult](t, server, "get_env", map[string]interface{}{
		"keys": []interface{}{"mcp_test_token"},
	})
	if masked.Count != 1 || masked.Variables["MCP_TEST_TOKEN"] != "****" {
		t.Fatalf("敏感变量未脱敏: %+v", masked)
	}
	if len(masked.MaskedKeys) != 1 || masked.MaskedKeys[0] != "MCP_TEST_TOKEN" {
		t.Fatalf("脱敏变量列表异常: %+v", masked.MaskedKeys)
	}

	plain := callToolResult[EnvResult](t, server, "get_env", map[string]interface{}{
		"keys": []interface{}{"MCP_TEST_TOKEN"}, "mask": false,
	})
	if plain.Variables["MCP_TEST_TOKEN"] != "supersecret" {
		t.Fatalf("mask=false 时应返回真实值: %+v", plain.Variables)
	}

	all := callToolResult[EnvResult](t, server, "get_env", nil)
	if all.Count == 0 {
		t.Fatal("未读取到任何环境变量")
	}
}

func TestSensitiveEnvPattern(t *testing.T) {
	sensitive := []string{"GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "MYSQL_PASSWORD", "OPENAI_API_KEY", "DB_CREDENTIAL"}
	for _, name := range sensitive {
		if !sensitiveEnvPattern.MatchString(name) {
			t.Fatalf("%s 应被识别为敏感变量", name)
		}
	}

	insensitive := []string{"PATH", "GOPRIVATE", "AUTHORIZATION_HEADER", "TEMP", "GOPROXY"}
	for _, name := range insensitive {
		if sensitiveEnvPattern.MatchString(name) {
			t.Fatalf("%s 不应被识别为敏感变量", name)
		}
	}
}

func TestListProcesses(t *testing.T) {
	server := NewBashServer()

	result := callToolResult[ProcessListResult](t, server, "list_processes", nil)
	if result.Platform != runtime.GOOS {
		t.Fatalf("平台 = %q, 期望 %q", result.Platform, runtime.GOOS)
	}
	if result.Count == 0 {
		t.Fatal("未获取到任何进程")
	}

	var foundSelf bool
	for _, process := range result.Processes {
		// Windows 的 System Idle Process 就是 PID 0，属于系统真实数据
		if process.PID < 0 || process.Name == "" {
			t.Fatalf("进程信息不完整: %+v", process)
		}
		if process.PID == os.Getpid() {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Fatalf("进程列表中未包含测试进程 PID %d", os.Getpid())
	}

	empty := callToolResult[ProcessListResult](t, server, "list_processes", map[string]interface{}{
		"filter": "mcp-nonexistent-process-xyz",
	})
	if empty.Count != 0 {
		t.Fatalf("无匹配的过滤条件应返回空列表, 实际 = %d", empty.Count)
	}
}

func TestGetDiskUsage(t *testing.T) {
	server := NewBashServer()

	result := callToolResult[DiskUsageResult](t, server, "get_disk_usage", nil)
	if result.Count == 0 {
		t.Fatal("未获取到任何磁盘信息")
	}
	for _, disk := range result.Disks {
		if disk.TotalBytes <= 0 {
			t.Fatalf("磁盘总容量异常: %+v", disk)
		}
		if disk.UsedPercent < 0 || disk.UsedPercent > 100 {
			t.Fatalf("使用率超出范围: %+v", disk)
		}
		if disk.Device == "" || disk.Mount == "" {
			t.Fatalf("磁盘标识缺失: %+v", disk)
		}
	}

	single := callToolResult[DiskUsageResult](t, server, "get_disk_usage", map[string]interface{}{
		"path": t.TempDir(),
	})
	if single.Count != 1 {
		t.Fatalf("指定路径应只返回所在磁盘, 实际 = %d", single.Count)
	}
	if single.Path == "" {
		t.Fatal("指定路径时未回填 path 字段")
	}
}

func TestGetSystemInfo(t *testing.T) {
	server := NewBashServer()

	result := callToolResult[SystemInfoResult](t, server, "get_system_info", nil)
	if result.OS != runtime.GOOS || result.Architecture != runtime.GOARCH {
		t.Fatalf("系统信息不匹配: %+v", result)
	}
	if result.CPUCores <= 0 || result.GoVersion == "" {
		t.Fatalf("CPU 或 Go 版本信息缺失: %+v", result)
	}
	if result.Hostname == "" || result.HomeDir == "" || result.WorkingDir == "" {
		t.Fatalf("基础路径信息缺失: %+v", result)
	}

	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if result.TotalMemoryBytes <= 0 {
			t.Fatalf("未获取到内存总量: %+v", result)
		}
		if result.UptimeSeconds <= 0 {
			t.Fatalf("未获取到运行时长: %+v", result)
		}
	}
}

func TestGetOSInfo(t *testing.T) {
	server := NewBashServer()

	result := callToolResult[OSInfoResponse](t, server, "get_os_info", nil)
	if result.OS == "" || result.Architecture == "" {
		t.Fatalf("系统信息不完整: %+v", result)
	}
	if result.Version == "" {
		t.Fatalf("系统版本为空: %+v", result)
	}
}

func TestWindowsVersionName(t *testing.T) {
	cases := []struct {
		text     string
		expected string
	}{
		{"Microsoft Windows 11 专业版", "Windows 11"},
		{"Microsoft Windows 10 Pro", "Windows 10"},
		{"Microsoft Windows 8.1 Enterprise", "Windows 8.1"},
		{"Microsoft Windows Server 2022 Standard", "Windows Server 2022"},
		{"Microsoft Windows [Version 10.0.22631.1]", "Windows 10.0.22631.1"},
		{"Microsoft Windows [版本 10.0.26100.1]", "Windows 10.0.26100.1"},
		{"无法识别的描述", ""},
	}

	for _, testCase := range cases {
		if actual := windowsVersionName(testCase.text); actual != testCase.expected {
			t.Fatalf("windowsVersionName(%q) = %q, 期望 %q", testCase.text, actual, testCase.expected)
		}
	}
}

func TestExecCommand(t *testing.T) {
	server := NewBashServer()

	success := callToolResult[CommandResponse](t, server, "mcp-bash-server", map[string]interface{}{
		"command": "echo hello",
	})
	if !success.Success || !strings.Contains(success.Output, "hello") {
		t.Fatalf("命令执行结果异常: %+v", success)
	}
	if success.ExitCode != 0 || success.Platform != runtime.GOOS {
		t.Fatalf("命令执行元信息异常: %+v", success)
	}

	failure := callToolResult[CommandResponse](t, server, "mcp-bash-server", map[string]interface{}{
		"command": "exit 3",
	})
	if failure.Success || failure.ExitCode != 3 {
		t.Fatalf("失败命令的退出码异常: %+v", failure)
	}

	expectToolError(t, server, "mcp-bash-server", map[string]interface{}{})
}

func TestExecCommandNonASCIROutput(t *testing.T) {
	server := NewBashServer()

	response := callToolResult[CommandResponse](t, server, "mcp-bash-server", map[string]interface{}{
		"command": "echo 中文输出测试",
	})
	if !response.Success {
		t.Fatalf("命令执行失败: %+v", response)
	}
	if !strings.Contains(response.Output, "中文输出测试") {
		t.Fatalf("非 ASCII 输出未按 UTF-8 解码: %q", response.Output)
	}
}

func TestExecuteCommandTimeoutClamp(t *testing.T) {
	server := NewBashServer()

	response := server.executeCommand(CommandRequest{Command: "echo ok", Timeout: -5})
	if !response.Success {
		t.Fatalf("超时时间被钳制后仍应执行成功: %+v", response)
	}
	if response.Duration == "" {
		t.Fatal("未记录执行耗时")
	}
}
