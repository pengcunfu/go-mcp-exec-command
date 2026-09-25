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
	"encoding/json"
	"testing"
)

type toolOutcome struct {
	response *MCPResponse
	isError  bool
	text     string
}

func callTool(t *testing.T, server *BashServer, name string, args map[string]interface{}) toolOutcome {
	t.Helper()

	response := server.handleRequest(MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]interface{}{"name": name, "arguments": args},
	})
	if response == nil {
		t.Fatalf("工具 %s 未返回响应", name)
	}

	outcome := toolOutcome{response: response}
	if response.Error != nil {
		return outcome
	}

	result, ok := response.Result.(CallToolResult)
	if !ok {
		t.Fatalf("工具 %s 返回类型异常: %T", name, response.Result)
	}
	if len(result.Content) == 0 {
		t.Fatalf("工具 %s 未返回内容", name)
	}
	outcome.isError = result.IsError
	outcome.text = result.Content[0].Text
	return outcome
}

func decodeToolResult[T any](t *testing.T, outcome toolOutcome) T {
	t.Helper()

	if outcome.response.Error != nil {
		t.Fatalf("工具调用返回协议错误: %s", outcome.response.Error.Message)
	}
	if outcome.isError {
		t.Fatalf("工具调用失败: %s", outcome.text)
	}

	var result T
	if err := json.Unmarshal([]byte(outcome.text), &result); err != nil {
		t.Fatalf("解析工具结果失败: %v (%s)", err, outcome.text)
	}
	return result
}

func callToolResult[T any](t *testing.T, server *BashServer, name string, args map[string]interface{}) T {
	t.Helper()
	return decodeToolResult[T](t, callTool(t, server, name, args))
}

func expectToolError(t *testing.T, server *BashServer, name string, args map[string]interface{}) string {
	t.Helper()

	outcome := callTool(t, server, name, args)
	if outcome.response.Error != nil {
		return outcome.response.Error.Message
	}
	if !outcome.isError {
		t.Fatalf("工具 %s 预期失败但执行成功: %s", name, outcome.text)
	}
	return outcome.text
}
