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
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	defaultCommandTimeout = 30
	minCommandTimeout     = 1
	maxCommandTimeout     = 86400
)

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

func (s *BashServer) execTools() []RegisteredTool {
	return []RegisteredTool{
		{
			Tool: Tool{
				Name:        "mcp-bash-server",
				Description: "跨平台执行系统命令。服务端会根据运行平台自动选择 shell（Windows 使用 PowerShell，Linux/macOS 使用 sh），调用方无需关心 shell 差异。命令将原样执行，不做任何转换或处理。如需按平台调整命令语法，可先调用 get_os_info 获取系统信息。",
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
			Handler: s.handleExecCommand,
		},
		{
			Tool: Tool{
				Name:        "get_os_info",
				Description: "获取操作系统信息，包括类型、版本、架构、主机名等。仅在需要按平台调整命令语法时调用。",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
			},
			Handler: s.handleGetOSInfo,
		},
	}
}

func (s *BashServer) handleExecCommand(args map[string]interface{}) (interface{}, error) {
	command, err := requireString(args, "command")
	if err != nil {
		return nil, err
	}
	workingDir, err := argString(args, "working_dir", "")
	if err != nil {
		return nil, err
	}
	timeout, err := argInt(args, "timeout", defaultCommandTimeout)
	if err != nil {
		return nil, err
	}

	return s.executeCommand(CommandRequest{
		Command:    command,
		WorkingDir: workingDir,
		Timeout:    timeout,
	}), nil
}

func (s *BashServer) handleGetOSInfo(args map[string]interface{}) (interface{}, error) {
	return s.getOSInfo(), nil
}

func (s *BashServer) executeCommand(req CommandRequest) CommandResponse {
	startTime := time.Now()

	// 设置默认值
	if req.Timeout == 0 {
		req.Timeout = defaultCommandTimeout
	}
	if req.Timeout < minCommandTimeout {
		req.Timeout = minCommandTimeout
	}
	if req.Timeout > maxCommandTimeout {
		req.Timeout = maxCommandTimeout
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
		// -NoProfile / -NonInteractive 避免用户配置与交互式提示污染输出或导致挂起。
		// 仅追加输出编码设置，不改变命令本身的错误处理语义。
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", powershellPrelude+command)
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
func (s *BashServer) getOSInfo() OSInfoResponse {
	response := OSInfoResponse{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		response.Hostname = hostname
	}

	response.Username = currentUsername()

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
func (s *BashServer) getWindowsVersion() (string, string) {
	// 较新的 Windows 已移除 wmic，优先使用 PowerShell CIM
	if output, err := runPowerShell("(Get-CimInstance Win32_OperatingSystem).Caption"); err == nil {
		if caption := strings.TrimSpace(output); caption != "" {
			if version := windowsVersionName(caption); version != "" {
				return version, caption
			}
		}
	}

	// 备用方法：使用wmic命令获取Windows版本
	cmd := exec.Command("wmic", "os", "get", "Caption,Version,BuildNumber", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		// 再备用方法：使用ver命令
		cmd = exec.Command("cmd", "/c", "ver")
		if output, err = cmd.Output(); err != nil {
			return "未知", "无法获取Windows版本信息"
		}
	}

	outputStr := string(output)
	if version := windowsVersionName(outputStr); version != "" {
		return version, outputStr
	}

	return "Windows (未知版本)", outputStr
}

// windowsVersionPattern 匹配 ver 命令的输出，兼容中文系统的“[版本 ...]”。
var windowsVersionPattern = regexp.MustCompile(`\[(?:Version|版本)\s*([^\]]+)\]`)

// windowsVersionName 从系统描述文本中识别 Windows 版本名称。
func windowsVersionName(text string) string {
	candidates := []struct{ keyword, name string }{
		{"Windows 11", "Windows 11"},
		{"Windows 10", "Windows 10"},
		{"Windows 8.1", "Windows 8.1"},
		{"Windows 8", "Windows 8"},
		{"Windows 7", "Windows 7"},
		{"Windows Server 2022", "Windows Server 2022"},
		{"Windows Server 2019", "Windows Server 2019"},
		{"Windows Server 2016", "Windows Server 2016"},
		{"Windows Server", "Windows Server"},
	}
	for _, candidate := range candidates {
		if strings.Contains(text, candidate.keyword) {
			return candidate.name
		}
	}

	if matches := windowsVersionPattern.FindStringSubmatch(text); len(matches) > 1 {
		return "Windows " + strings.TrimSpace(matches[1])
	}
	return ""
}

// 获取Linux版本信息
func (s *BashServer) getLinuxVersion() (string, string) {
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
func (s *BashServer) getMacOSVersion() (string, string) {
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
