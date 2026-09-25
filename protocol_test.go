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
	"path/filepath"
	"testing"
)

var expectedToolNames = []string{
	"mcp-bash-server",
	"get_os_info",
	"read_file",
	"write_file",
	"edit_file",
	"list_dir",
	"directory_tree",
	"search_files",
	"get_file_info",
	"list_processes",
	"get_env",
	"get_disk_usage",
	"get_system_info",
}

func TestInitialize(t *testing.T) {
	server := NewBashServer()

	response := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	if response == nil || response.Error != nil {
		t.Fatalf("初始化失败: %+v", response)
	}

	result, ok := response.Result.(InitializeResult)
	if !ok {
		t.Fatalf("初始化结果类型异常: %T", response.Result)
	}
	if result.ProtocolVersion != protocolVersion {
		t.Fatalf("协议版本 = %q, 期望 %q", result.ProtocolVersion, protocolVersion)
	}
	if result.ServerInfo.Name != serverName {
		t.Fatalf("服务名 = %q, 期望 %q", result.ServerInfo.Name, serverName)
	}
}

func TestPing(t *testing.T) {
	server := NewBashServer()

	response := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 2, Method: "ping"})
	if response == nil || response.Error != nil {
		t.Fatalf("ping 失败: %+v", response)
	}
	if response.ID != 2 {
		t.Fatalf("响应 ID = %v, 期望 2", response.ID)
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	server := NewBashServer()

	for _, method := range []string{"notifications/initialized", "notifications/cancelled"} {
		if response := server.handleRequest(MCPRequest{JSONRPC: "2.0", Method: method}); response != nil {
			t.Fatalf("通知 %s 不应返回响应: %+v", method, response)
		}
	}
}

func TestToolsList(t *testing.T) {
	server := NewBashServer()

	response := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 3, Method: "tools/list"})
	if response == nil || response.Error != nil {
		t.Fatalf("获取工具列表失败: %+v", response)
	}

	payload, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("工具列表结果类型异常: %T", response.Result)
	}
	tools, ok := payload["tools"].([]Tool)
	if !ok {
		t.Fatalf("工具列表字段类型异常: %T", payload["tools"])
	}

	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Description == "" {
			t.Fatalf("工具 %s 缺少描述", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Fatalf("工具 %s 缺少输入结构", tool.Name)
		}
	}

	if len(tools) != len(expectedToolNames) {
		t.Fatalf("工具数量 = %d, 期望 %d", len(tools), len(expectedToolNames))
	}
	for _, name := range expectedToolNames {
		if !names[name] {
			t.Fatalf("工具列表缺少 %s", name)
		}
	}
}

func TestUnknownToolAndMethod(t *testing.T) {
	server := NewBashServer()

	unknownTool := server.handleRequest(MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  map[string]interface{}{"name": "not-exist", "arguments": map[string]interface{}{}},
	})
	if unknownTool == nil || unknownTool.Error == nil || unknownTool.Error.Code != -32601 {
		t.Fatalf("未知工具应返回 -32601: %+v", unknownTool)
	}

	unknownMethod := server.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 5, Method: "unknown/method"})
	if unknownMethod == nil || unknownMethod.Error == nil || unknownMethod.Error.Code != -32601 {
		t.Fatalf("未知方法应返回 -32601: %+v", unknownMethod)
	}
}

func TestToolFailureReturnsContent(t *testing.T) {
	server := NewBashServer()

	outcome := callTool(t, server, "read_file", map[string]interface{}{
		"path": filepath.Join(t.TempDir(), "missing.txt"),
	})
	if outcome.response.Error != nil {
		t.Fatalf("工具执行失败应以内容形式返回, 而不是协议错误: %+v", outcome.response.Error)
	}
	if !outcome.isError || outcome.text == "" {
		t.Fatalf("工具失败结果异常: %+v", outcome)
	}
}

func TestDecodeParams(t *testing.T) {
	params, err := decodeParams[CallToolParams](map[string]interface{}{
		"name":      "read_file",
		"arguments": map[string]interface{}{"path": "a.txt"},
	})
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	if params.Name != "read_file" || params.Arguments["path"] != "a.txt" {
		t.Fatalf("参数解析结果异常: %+v", params)
	}

	if _, err := decodeParams[CallToolParams](nil); err != nil {
		t.Fatalf("nil 参数不应报错: %v", err)
	}
}
