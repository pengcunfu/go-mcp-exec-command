package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	serverName    = "go-mcp-exec-command"
	serverVersion = "0.0.1"
)

// MCP 协议结构
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CommandRequest struct {
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
	AutoConvert bool   `json:"auto_convert,omitempty"`
}

type CommandResponse struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
	Command  string `json:"command"`
	Platform string `json:"platform"`
}

type ExecCommandServer struct{}

func NewExecCommandServer() *ExecCommandServer {
	return &ExecCommandServer{}
}

// 跨平台命令转换
func (s *ExecCommandServer) convertCommand(command string) string {
	if runtime.GOOS != "windows" {
		return command
	}

	// Windows 平台转换
	converted := command

	// 1. 替换 && 为 ;
	converted = strings.ReplaceAll(converted, " && ", " ; ")
	converted = strings.ReplaceAll(converted, "&&", ";")

	// 2. 处理常见的 Linux 命令
	converted = s.convertLinuxCommands(converted)

	// 3. 处理路径分隔符
	converted = s.convertPathSeparators(converted)

	return converted
}

// 转换常见的 Linux 命令到 Windows 等价命令
func (s *ExecCommandServer) convertLinuxCommands(command string) string {
	// 常见命令映射
	replacements := map[string]string{
		"ls -la":    "dir",
		"ls -l":     "dir",
		"ls":        "dir",
		"cat ":      "type ",
		"grep ":     "findstr ",
		"which ":    "where ",
		"pwd":       "cd",
		"rm -rf ":   "rmdir /s /q ",
		"rm -f ":    "del /f ",
		"rm ":       "del ",
		"cp -r ":    "xcopy /e /i ",
		"cp ":       "copy ",
		"mv ":       "move ",
		"mkdir -p ": "mkdir ",
		"touch ":    "echo. > ",
	}

	result := command
	for linux, windows := range replacements {
		if strings.Contains(result, linux) {
			result = strings.ReplaceAll(result, linux, windows)
		}
	}

	return result
}

// 转换路径分隔符
func (s *ExecCommandServer) convertPathSeparators(command string) string {
	if runtime.GOOS != "windows" {
		return command
	}

	// 简单的路径转换，避免过度转换
	words := strings.Fields(command)
	for i, word := range words {
		// 如果包含 / 且看起来像路径
		if strings.Contains(word, "/") && !strings.HasPrefix(word, "http") {
			words[i] = strings.ReplaceAll(word, "/", "\\")
		}
	}

	return strings.Join(words, " ")
}

// 处理 Windsurf Invoke-Session 引号问题
func (s *ExecCommandServer) sanitizeCommand(command string) string {
	// 移除可能导致问题的 Invoke-Session 前缀
	if strings.HasPrefix(command, "Invoke-Session") {
		// 提取实际命令
		parts := strings.SplitN(command, "\"", 3)
		if len(parts) >= 3 {
			command = parts[1]
		}
	}

	// 处理嵌套引号问题
	command = s.handleNestedQuotes(command)

	return command
}

// 处理嵌套引号
func (s *ExecCommandServer) handleNestedQuotes(command string) string {
	// 如果命令被双引号包围，且内部有引号，需要特殊处理
	if strings.HasPrefix(command, "\"") && strings.HasSuffix(command, "\"") {
		// 移除外层引号
		inner := command[1 : len(command)-1]

		// 转义内部引号
		inner = strings.ReplaceAll(inner, "\"", "\\\"")

		return inner
	}

	// 转义单独的引号
	return strings.ReplaceAll(command, "\"", "\\\"")
}

func (s *ExecCommandServer) executeCommand(req CommandRequest) CommandResponse {
	startTime := time.Now()

	// 设置默认值
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	// 处理命令
	originalCommand := req.Command
	processedCommand := s.sanitizeCommand(req.Command)

	if req.AutoConvert {
		processedCommand = s.convertCommand(processedCommand)
	}

	// 创建执行上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	// 设置工作目录
	workingDir := req.WorkingDir
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	// 执行命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", processedCommand)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", processedCommand)
	}

	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	response := CommandResponse{
		Success:  err == nil,
		Output:   string(output),
		Duration: duration.String(),
		Command:  processedCommand,
		Platform: runtime.GOOS,
	}

	if err != nil {
		response.Error = err.Error()
		if exitError, ok := err.(*exec.ExitError); ok {
			response.ExitCode = exitError.ExitCode()
		} else {
			response.ExitCode = -1
		}
	}

	// 添加调试信息
	debugInfo := fmt.Sprintf("\n--- 调试信息 ---\n原始命令: %s\n处理后命令: %s\n平台: %s\n工作目录: %s\n执行时间: %s\n",
		originalCommand, processedCommand, runtime.GOOS, workingDir, duration)

	response.Output = response.Output + debugInfo

	return response
}

func (s *ExecCommandServer) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				ServerInfo: ServerInfo{
					Name:    serverName,
					Version: serverVersion,
				},
			},
		}

	case "tools/list":
		tools := []Tool{
			{
				Name:        "exec_command",
				Description: "执行系统命令，支持跨平台命令转换和特殊字符处理",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "要执行的命令",
						},
						"working_dir": map[string]interface{}{
							"type":        "string",
							"description": "工作目录（可选）",
						},
						"timeout": map[string]interface{}{
							"type":        "integer",
							"description": "超时时间（秒，默认30秒）",
						},
						"auto_convert": map[string]interface{}{
							"type":        "boolean",
							"description": "是否自动转换跨平台命令（默认true）",
						},
					},
					"required": []string{"command"},
				},
			},
		}
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": tools,
			},
		}

	case "tools/call":
		var params CallToolParams
		if paramBytes, err := json.Marshal(req.Params); err == nil {
			json.Unmarshal(paramBytes, &params)
		}

		if params.Name == "exec_command" {
			var cmdReq CommandRequest
			if argBytes, err := json.Marshal(params.Arguments); err == nil {
				json.Unmarshal(argBytes, &cmdReq)
			}

			// 默认启用自动转换
			if params.Arguments["auto_convert"] == nil {
				cmdReq.AutoConvert = true
			}

			result := s.executeCommand(cmdReq)
			resultBytes, _ := json.Marshal(result)

			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": string(resultBytes),
						},
					},
				},
			}
		}

		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Unknown tool: " + params.Name,
			},
		}

	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found: " + req.Method,
			},
		}
	}
}

func (s *ExecCommandServer) Run() error {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("解析请求失败: %v", err)
			continue
		}

		response := s.handleRequest(req)

		if responseBytes, err := json.Marshal(response); err == nil {
			fmt.Println(string(responseBytes))
		}
	}

	return scanner.Err()
}

func main() {
	server := NewExecCommandServer()

	log.Printf("启动 %s v%s", serverName, serverVersion)
	log.Printf("平台: %s", runtime.GOOS)

	if err := server.Run(); err != nil {
		log.Fatalf("服务器运行失败: %v", err)
	}
}
