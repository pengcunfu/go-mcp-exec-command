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
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

const (
	serverName    = "mcp-bash-server"
	serverVersion = "0.1.0"

	protocolVersion = "2024-11-05"

	// 单条请求的最大长度，避免写入大文件时触发 scanner 默认 64KB 上限。
	maxRequestBytes = 16 * 1024 * 1024
)

// ToolHandler 处理一次工具调用，返回值会被序列化为 JSON 文本。
type ToolHandler func(args map[string]interface{}) (interface{}, error)

type RegisteredTool struct {
	Tool    Tool
	Handler ToolHandler
}

type BashServer struct {
	tools    []RegisteredTool
	toolByID map[string]RegisteredTool
}

func NewBashServer() *BashServer {
	server := &BashServer{}
	server.tools = server.registerTools()
	server.toolByID = make(map[string]RegisteredTool, len(server.tools))
	for _, tool := range server.tools {
		server.toolByID[tool.Tool.Name] = tool
	}
	return server
}

func (s *BashServer) registerTools() []RegisteredTool {
	tools := make([]RegisteredTool, 0, 16)
	tools = append(tools, s.execTools()...)
	tools = append(tools, s.fileTools()...)
	tools = append(tools, s.systemTools()...)
	return tools
}

func (s *BashServer) handleRequest(req MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.respond(req.ID, InitializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: ServerInfo{
				Name:    serverName,
				Version: serverVersion,
			},
		})

	case "ping":
		return s.respond(req.ID, map[string]interface{}{})

	case "tools/list":
		tools := make([]Tool, 0, len(s.tools))
		for _, tool := range s.tools {
			tools = append(tools, tool.Tool)
		}
		return s.respond(req.ID, map[string]interface{}{"tools": tools})

	case "tools/call":
		return s.handleToolCall(req)
	}

	// 通知类消息按协议不需要响应。
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil
	}

	return s.respondError(req.ID, -32601, "Method not found: "+req.Method)
}

func (s *BashServer) handleToolCall(req MCPRequest) *MCPResponse {
	params, err := decodeParams[CallToolParams](req.Params)
	if err != nil {
		return s.respondError(req.ID, -32602, "解析工具参数失败: "+err.Error())
	}

	tool, ok := s.toolByID[params.Name]
	if !ok {
		return s.respondError(req.ID, -32601, "Unknown tool: "+params.Name)
	}

	result, err := tool.Handler(params.Arguments)
	if err != nil {
		return s.respond(req.ID, CallToolResult{
			Content: []ToolContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}

	text, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return s.respondError(req.ID, -32603, "序列化工具结果失败: "+err.Error())
	}

	return s.respond(req.ID, CallToolResult{
		Content: []ToolContent{{Type: "text", Text: string(text)}},
	})
}

func (s *BashServer) respond(id interface{}, result interface{}) *MCPResponse {
	return &MCPResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *BashServer) respondError(id interface{}, code int, message string) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
}

// decodeParams 通过 JSON 中转把未定型的参数转换为目标结构体。
func decodeParams[T any](params interface{}) (T, error) {
	var result T
	if params == nil {
		return result, nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (s *BashServer) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), maxRequestBytes)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("解析请求失败: %v", err)
			continue
		}

		response := s.handleRequest(req)
		if response == nil {
			continue
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("序列化响应失败: %v", err)
			continue
		}
		fmt.Println(string(responseBytes))
	}

	return scanner.Err()
}

func main() {
	server := NewBashServer()

	log.Printf("启动 %s v%s", serverName, serverVersion)
	log.Printf("平台: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("已注册工具: %d 个", len(server.tools))

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}
