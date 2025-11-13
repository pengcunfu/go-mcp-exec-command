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
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
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


func (s *ExecCommandServer) executeCommand(req CommandRequest) CommandResponse {
	startTime := time.Now()

	// 设置默认值
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	// 直接使用原始命令，不做任何处理
	command := req.Command

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
		cmd = exec.CommandContext(ctx, "powershell", "-Command", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	response := CommandResponse{
		Success:  err == nil,
		Output:   string(output),
		Duration: duration.String(),
		Command:  command,
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
				Description: "直接执行系统命令，不做任何处理或转换",
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
