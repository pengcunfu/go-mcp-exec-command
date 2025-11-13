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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
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

type OSInfoResponse struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
	Username     string `json:"username"`
	Details      string `json:"details"`
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

// 检测命令是否需要跨平台转换
func (s *ExecCommandServer) needsCrossPlatformConversion(command string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	
	// 检测常见的Linux命令和语法
	linuxIndicators := []string{
		" && ",  // Linux命令连接符
		"ls ",   // Linux命令
		"cat ",
		"grep ",
		"which ",
		"rm ",
		"cp ",
		"mv ",
		"mkdir -p",
		"touch ",
		"pwd",
	}
	
	commandLower := strings.ToLower(command)
	for _, indicator := range linuxIndicators {
		if strings.Contains(commandLower, indicator) {
			return true
		}
	}
	
	// 检测是否以Linux命令开头
	linuxCommands := []string{"ls", "cat", "grep", "which", "rm", "cp", "mv", "pwd"}
	firstWord := strings.Fields(command)
	if len(firstWord) > 0 {
		for _, cmd := range linuxCommands {
			if strings.ToLower(firstWord[0]) == cmd {
				return true
			}
		}
	}
	
	return false
}

func (s *ExecCommandServer) executeCommand(req CommandRequest) CommandResponse {
	startTime := time.Now()

	// 设置默认值
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	// 处理命令
	originalCommand := req.Command
	processedCommand := req.Command
	
	// 检测是否是Windsurf的Invoke-Session命令，如果是则需要清理
	if strings.HasPrefix(req.Command, "Invoke-Session") {
		processedCommand = s.sanitizeCommand(req.Command)
	}
	
	// 只有在明确需要跨平台转换时才转换
	if req.AutoConvert && s.needsCrossPlatformConversion(processedCommand) {
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

// 获取详细的操作系统信息
func (s *ExecCommandServer) getOSInfo() OSInfoResponse {
	response := OSInfoResponse{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		response.Hostname = hostname
	}

	// 获取用户名
	if username := os.Getenv("USER"); username != "" {
		response.Username = username
	} else if username := os.Getenv("USERNAME"); username != "" {
		response.Username = username
	}

	// 根据操作系统获取详细版本信息
	switch runtime.GOOS {
	case "windows":
		response.Version, response.Details = s.getWindowsVersion()
	case "linux":
		response.Version, response.Details = s.getLinuxVersion()
	case "darwin":
		response.Version, response.Details = s.getMacOSVersion()
	default:
		response.Version = "未知版本"
		response.Details = "不支持的操作系统"
	}

	return response
}

// 获取Windows版本信息
func (s *ExecCommandServer) getWindowsVersion() (string, string) {
	// 使用wmic命令获取Windows版本
	cmd := exec.Command("wmic", "os", "get", "Caption,Version,BuildNumber", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		// 备用方法：使用ver命令
		cmd = exec.Command("cmd", "/c", "ver")
		if output, err = cmd.Output(); err != nil {
			return "未知", "无法获取Windows版本信息"
		}
	}

	outputStr := string(output)
	
	// 解析Windows版本
	if strings.Contains(outputStr, "Windows 11") {
		return "Windows 11", outputStr
	} else if strings.Contains(outputStr, "Windows 10") {
		return "Windows 10", outputStr
	} else if strings.Contains(outputStr, "Windows 8.1") {
		return "Windows 8.1", outputStr
	} else if strings.Contains(outputStr, "Windows 8") {
		return "Windows 8", outputStr
	} else if strings.Contains(outputStr, "Windows 7") {
		return "Windows 7", outputStr
	} else if strings.Contains(outputStr, "Windows Server 2022") {
		return "Windows Server 2022", outputStr
	} else if strings.Contains(outputStr, "Windows Server 2019") {
		return "Windows Server 2019", outputStr
	} else if strings.Contains(outputStr, "Windows Server 2016") {
		return "Windows Server 2016", outputStr
	} else if strings.Contains(outputStr, "Windows Server") {
		return "Windows Server", outputStr
	}

	// 使用正则表达式提取版本号
	re := regexp.MustCompile(`Microsoft Windows \[Version ([^\]]+)\]`)
	if matches := re.FindStringSubmatch(outputStr); len(matches) > 1 {
		return "Windows " + matches[1], outputStr
	}

	return "Windows (未知版本)", outputStr
}

// 获取Linux版本信息
func (s *ExecCommandServer) getLinuxVersion() (string, string) {
	// 尝试读取/etc/os-release文件
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(content), "\n")
		var name, version string
		for _, line := range lines {
			if strings.HasPrefix(line, "NAME=") {
				name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
			} else if strings.HasPrefix(line, "VERSION=") {
				version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
			}
		}
		if name != "" {
			if version != "" {
				return name + " " + version, string(content)
			}
			return name, string(content)
		}
	}

	// 备用方法：使用lsb_release命令
	cmd := exec.Command("lsb_release", "-d")
	if output, err := cmd.Output(); err == nil {
		line := strings.TrimSpace(string(output))
		if strings.HasPrefix(line, "Description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Description:")), line
		}
	}

	// 再备用方法：检查常见的发行版文件
	distroFiles := []string{
		"/etc/redhat-release",
		"/etc/debian_version",
		"/etc/ubuntu-release",
		"/etc/centos-release",
	}

	for _, file := range distroFiles {
		if content, err := os.ReadFile(file); err == nil {
			return strings.TrimSpace(string(content)), string(content)
		}
	}

	return "Linux (未知发行版)", "无法获取Linux版本信息"
}

// 获取macOS版本信息
func (s *ExecCommandServer) getMacOSVersion() (string, string) {
	cmd := exec.Command("sw_vers")
	output, err := cmd.Output()
	if err != nil {
		return "macOS (未知版本)", "无法获取macOS版本信息"
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	var productName, productVersion string

	for _, line := range lines {
		if strings.HasPrefix(line, "ProductName:") {
			productName = strings.TrimSpace(strings.TrimPrefix(line, "ProductName:"))
		} else if strings.HasPrefix(line, "ProductVersion:") {
			productVersion = strings.TrimSpace(strings.TrimPrefix(line, "ProductVersion:"))
		}
	}

	if productName != "" && productVersion != "" {
		return productName + " " + productVersion, outputStr
	}

	return "macOS", outputStr
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
				Description: "执行系统命令，智能处理Windsurf的Invoke-Session前缀，自动检测并转换Linux命令到Windows",
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
			{
				Name:        "get_os_info",
				Description: "获取详细的操作系统信息，包括版本、架构、主机名等",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
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

		if params.Name == "get_os_info" {
			result := s.getOSInfo()
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
