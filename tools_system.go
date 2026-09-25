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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemPercent float64 `json:"mem_percent,omitempty"`
	MemBytes   int64   `json:"mem_bytes,omitempty"`
	Command    string  `json:"command,omitempty"`
}

type ProcessListResult struct {
	Platform  string        `json:"platform"`
	Filter    string        `json:"filter,omitempty"`
	Count     int           `json:"count"`
	Processes []ProcessInfo `json:"processes"`
}

type EnvResult struct {
	Count      int               `json:"count"`
	Variables  map[string]string `json:"variables"`
	MaskedKeys []string          `json:"masked_keys,omitempty"`
}

type DiskUsage struct {
	Device      string  `json:"device"`
	Mount       string  `json:"mount"`
	FileSystem  string  `json:"file_system,omitempty"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskUsageResult struct {
	Path  string      `json:"path,omitempty"`
	Count int         `json:"count"`
	Disks []DiskUsage `json:"disks"`
}

type SystemInfoResult struct {
	OS               string `json:"os"`
	Architecture     string `json:"architecture"`
	Version          string `json:"version,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
	Username         string `json:"username,omitempty"`
	HomeDir          string `json:"home_dir,omitempty"`
	WorkingDir       string `json:"working_dir,omitempty"`
	CPUCores         int    `json:"cpu_cores"`
	GoVersion        string `json:"go_version"`
	TotalMemoryBytes int64  `json:"total_memory_bytes,omitempty"`
	FreeMemoryBytes  int64  `json:"free_memory_bytes,omitempty"`
	UptimeSeconds    int64  `json:"uptime_seconds,omitempty"`
}

// 环境变量中形如 TOKEN、API_KEY、PASSWORD 的键会被脱敏。
var sensitiveEnvPattern = regexp.MustCompile(`(?i)(^|_)(password|passwd|pwd|secret|token|api_?key|access_?key|private_?key|credential|auth)(_|$)`)

var (
	dfLinePattern          = regexp.MustCompile(`^(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)%\s+(.+)$`)
	darwinBootTimePattern  = regexp.MustCompile(`sec\s*=\s*(\d+)`)
	darwinFreePagesPattern = regexp.MustCompile(`Pages free:\s+(\d+)`)
)

func (s *BashServer) systemTools() []RegisteredTool {
	return []RegisteredTool{
		{
			Tool: Tool{
				Name:        "list_processes",
				Description: "列出当前系统中的进程，返回统一字段（PID、名称、内存、命令行等）。Windows 使用 tasklist，Linux/macOS 使用 ps，调用方无需关心平台命令差异。部分字段在特定平台上不可用时会省略。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filter": map[string]interface{}{
							"type":        "string",
							"description": "按进程名或命令行做不区分大小写的子串过滤（可选）",
						},
					},
					"required": []string{},
				},
			},
			Handler: s.handleListProcesses,
		},
		{
			Tool: Tool{
				Name:        "get_env",
				Description: "读取环境变量。可按名称筛选，默认对包含 TOKEN、SECRET、PASSWORD、API_KEY 等敏感字样的变量值脱敏。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"keys": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "只读取指定的变量名列表（可选，默认返回全部）",
						},
						"mask": map[string]interface{}{
							"type":        "boolean",
							"description": "是否对敏感变量的值脱敏（默认 true）",
						},
					},
					"required": []string{},
				},
			},
			Handler: s.handleGetEnv,
		},
		{
			Tool: Tool{
				Name:        "get_disk_usage",
				Description: "获取磁盘容量信息，返回总容量、已用、可用与使用率，单位统一为字节。不传 path 时返回所有磁盘，传入 path 时只返回该路径所在磁盘。",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "要查询的路径，支持 ~ 与相对路径（可选）",
						},
					},
					"required": []string{},
				},
			},
			Handler: s.handleGetDiskUsage,
		},
		{
			Tool: Tool{
				Name:        "get_system_info",
				Description: "获取运行环境概况：操作系统、架构、主机名、CPU 核心数、内存、运行时长等，用于判断资源情况。系统版本详情请使用 get_os_info。不同平台上无法获取的字段会被省略。",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				},
			},
			Handler: s.handleGetSystemInfo,
		},
	}
}

func (s *BashServer) handleListProcesses(args map[string]interface{}) (interface{}, error) {
	filter, err := argString(args, "filter", "")
	if err != nil {
		return nil, err
	}

	var processes []ProcessInfo
	if runtime.GOOS == "windows" {
		processes, err = listProcessesWindows()
	} else {
		processes, err = listProcessesUnix()
	}
	if err != nil {
		return nil, err
	}

	filter = strings.TrimSpace(filter)
	if filter != "" {
		needle := strings.ToLower(filter)
		matched := make([]ProcessInfo, 0, len(processes))
		for _, process := range processes {
			if strings.Contains(strings.ToLower(process.Name), needle) ||
				strings.Contains(strings.ToLower(process.Command), needle) {
				matched = append(matched, process)
			}
		}
		processes = matched
	}
	if processes == nil {
		processes = []ProcessInfo{}
	}

	sort.Slice(processes, func(i, j int) bool { return processes[i].PID < processes[j].PID })

	return ProcessListResult{
		Platform:  runtime.GOOS,
		Filter:    filter,
		Count:     len(processes),
		Processes: processes,
	}, nil
}

func listProcessesWindows() ([]ProcessInfo, error) {
	output, err := runCommand("tasklist", "/FO", "CSV", "/NH")
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(output))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 tasklist 输出失败: %w", err)
	}

	processes := make([]ProcessInfo, 0, len(records))
	for _, record := range records {
		if len(record) < 5 {
			continue
		}
		name := strings.TrimSpace(record[0])
		if name == "" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			continue
		}
		processes = append(processes, ProcessInfo{
			PID:      pid,
			Name:     name,
			MemBytes: parseTasklistMemory(record[4]),
			Command:  name,
		})
	}
	return processes, nil
}

// parseTasklistMemory 解析 tasklist 的内存字段，如 "11,200 K"。
func parseTasklistMemory(text string) int64 {
	cleaned := strings.NewReplacer(",", "", "K", "", "k", "", " ", "").Replace(text)
	value, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0
	}
	return value * 1024
}

func listProcessesUnix() ([]ProcessInfo, error) {
	output, err := runCommand("ps", "-eo", "pid=,user=,%cpu=,%mem=,rss=,comm=,args=")
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, 256)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		process := ProcessInfo{PID: pid, User: fields[1], Name: fields[5], Command: fields[5]}
		process.CPUPercent, _ = strconv.ParseFloat(fields[2], 64)
		process.MemPercent, _ = strconv.ParseFloat(fields[3], 64)
		if resident, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			process.MemBytes = resident * 1024
		}
		if len(fields) > 6 {
			process.Command = strings.Join(fields[6:], " ")
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func (s *BashServer) handleGetEnv(args map[string]interface{}) (interface{}, error) {
	keys, err := argStringList(args, "keys")
	if err != nil {
		return nil, err
	}
	mask, err := argBool(args, "mask", true)
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		wanted[strings.ToUpper(key)] = true
	}

	result := EnvResult{Variables: map[string]string{}}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name, value := parts[0], parts[1]
		if len(wanted) > 0 && !wanted[strings.ToUpper(name)] {
			continue
		}
		if mask && sensitiveEnvPattern.MatchString(name) {
			result.MaskedKeys = append(result.MaskedKeys, name)
			value = "****"
		}
		result.Variables[name] = value
	}
	sort.Strings(result.MaskedKeys)
	result.Count = len(result.Variables)
	return result, nil
}

func (s *BashServer) handleGetDiskUsage(args map[string]interface{}) (interface{}, error) {
	rawPath, err := argString(args, "path", "")
	if err != nil {
		return nil, err
	}

	result := DiskUsageResult{}
	target := ""
	if strings.TrimSpace(rawPath) != "" {
		target, err = resolvePath(rawPath)
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(target); statErr != nil {
			return nil, wrapPathError(rawPath, statErr)
		}
		result.Path = target
	}

	if runtime.GOOS == "windows" {
		result.Disks, err = diskUsageWindows(target)
	} else {
		result.Disks, err = diskUsageUnix(target)
	}
	if err != nil {
		return nil, err
	}

	result.Count = len(result.Disks)
	return result, nil
}

type windowsVolume struct {
	DeviceID   string `json:"DeviceID"`
	FileSystem string `json:"FileSystem"`
	Size       int64  `json:"Size"`
	FreeSpace  int64  `json:"FreeSpace"`
}

func diskUsageWindows(target string) ([]DiskUsage, error) {
	script := "Get-CimInstance Win32_LogicalDisk | Select-Object DeviceID,FileSystem,Size,FreeSpace | ConvertTo-Json -Compress"
	output, err := runPowerShell(script)
	if err != nil {
		return nil, err
	}

	volumes, err := decodeWindowsVolumes(output)
	if err != nil {
		return nil, err
	}

	drive := ""
	if target != "" {
		drive = filepath.VolumeName(target)
	}

	disks := make([]DiskUsage, 0, len(volumes))
	for _, volume := range volumes {
		if drive != "" && !strings.EqualFold(volume.DeviceID, drive) {
			continue
		}
		if volume.Size <= 0 {
			continue
		}
		disks = append(disks, DiskUsage{
			Device:      volume.DeviceID,
			Mount:       volume.DeviceID,
			FileSystem:  volume.FileSystem,
			TotalBytes:  volume.Size,
			UsedBytes:   volume.Size - volume.FreeSpace,
			FreeBytes:   volume.FreeSpace,
			UsedPercent: usedPercent(volume.Size, volume.Size-volume.FreeSpace),
		})
	}

	if len(disks) == 0 {
		if target != "" {
			return nil, fmt.Errorf("未找到 %s 所在磁盘的用量信息", target)
		}
		return nil, fmt.Errorf("未获取到任何磁盘用量信息")
	}
	return disks, nil
}

func decodeWindowsVolumes(output string) ([]windowsVolume, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("未获取到磁盘信息")
	}

	var volumes []windowsVolume
	if err := json.Unmarshal([]byte(trimmed), &volumes); err == nil {
		return volumes, nil
	}

	// 只有一个卷时 PowerShell 会返回对象而不是数组
	var single windowsVolume
	if err := json.Unmarshal([]byte(trimmed), &single); err == nil && single.DeviceID != "" {
		return []windowsVolume{single}, nil
	}

	return nil, fmt.Errorf("解析磁盘信息失败: %s", truncate(trimmed, 200))
}

func diskUsageUnix(target string) ([]DiskUsage, error) {
	args := []string{"-Pk"}
	if target != "" {
		args = append(args, target)
	}

	output, err := runCommand("df", args...)
	if err != nil {
		return nil, err
	}

	disks := make([]DiskUsage, 0, 8)
	for _, line := range strings.Split(output, "\n") {
		matches := dfLinePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) == 0 {
			continue
		}
		blocks, blocksErr := strconv.ParseInt(matches[2], 10, 64)
		used, usedErr := strconv.ParseInt(matches[3], 10, 64)
		available, availableErr := strconv.ParseInt(matches[4], 10, 64)
		if blocksErr != nil || usedErr != nil || availableErr != nil {
			continue
		}

		total := blocks * 1024
		usedBytes := used * 1024
		disks = append(disks, DiskUsage{
			Device:      matches[1],
			Mount:       matches[6],
			TotalBytes:  total,
			UsedBytes:   usedBytes,
			FreeBytes:   available * 1024,
			UsedPercent: usedPercent(total, usedBytes),
		})
	}

	if len(disks) == 0 {
		return nil, fmt.Errorf("未获取到任何磁盘用量信息")
	}
	return disks, nil
}

func usedPercent(total, used int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(used)/float64(total)*1000) / 10
}

func (s *BashServer) handleGetSystemInfo(args map[string]interface{}) (interface{}, error) {
	result := SystemInfoResult{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUCores:     runtime.NumCPU(),
		GoVersion:    runtime.Version(),
		Username:     currentUsername(),
		Version:      s.getOSInfo().Version,
	}
	if hostname, err := os.Hostname(); err == nil {
		result.Hostname = hostname
	}
	if home, err := os.UserHomeDir(); err == nil {
		result.HomeDir = home
	}
	if workingDir, err := os.Getwd(); err == nil {
		result.WorkingDir = workingDir
	}

	fillResourceInfo(&result)
	return result, nil
}

// fillResourceInfo 采集内存与运行时长，无法获取时保持字段为空。
func fillResourceInfo(info *SystemInfoResult) {
	switch runtime.GOOS {
	case "windows":
		fillWindowsResources(info)
	case "linux":
		fillLinuxResources(info)
	case "darwin":
		fillDarwinResources(info)
	}
}

func fillWindowsResources(info *SystemInfoResult) {
	script := "$os = Get-CimInstance Win32_OperatingSystem; " +
		"[pscustomobject]@{TotalMemoryBytes = ([int64]$os.TotalVisibleMemorySize) * 1KB; " +
		"FreeMemoryBytes = ([int64]$os.FreePhysicalMemory) * 1KB; " +
		"UptimeSeconds = [int64]((Get-Date) - $os.LastBootUpTime).TotalSeconds} | ConvertTo-Json -Compress"

	output, err := runPowerShell(script)
	if err != nil {
		return
	}

	var payload struct {
		TotalMemoryBytes int64 `json:"TotalMemoryBytes"`
		FreeMemoryBytes  int64 `json:"FreeMemoryBytes"`
		UptimeSeconds    int64 `json:"UptimeSeconds"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err != nil {
		return
	}

	info.TotalMemoryBytes = payload.TotalMemoryBytes
	info.FreeMemoryBytes = payload.FreeMemoryBytes
	if payload.UptimeSeconds > 0 {
		info.UptimeSeconds = payload.UptimeSeconds
	}
}

func fillLinuxResources(info *SystemInfoResult) {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				continue
			}
			switch fields[0] {
			case "MemTotal:":
				info.TotalMemoryBytes = value * 1024
			case "MemAvailable:":
				info.FreeMemoryBytes = value * 1024
			}
		}
	}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
				info.UptimeSeconds = int64(seconds)
			}
		}
	}
}

func fillDarwinResources(info *SystemInfoResult) {
	command := "sysctl -n hw.memsize; sysctl -n hw.pagesize; sysctl -n kern.boottime; vm_stat"
	output, err := runCommand("/bin/sh", "-c", command)
	if err != nil {
		return
	}

	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		if value, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64); err == nil {
			info.TotalMemoryBytes = value
		}
	}

	var pageSize int64
	if len(lines) > 1 {
		pageSize, _ = strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	}

	if len(lines) > 2 {
		if matches := darwinBootTimePattern.FindStringSubmatch(lines[2]); len(matches) > 1 {
			if bootTime, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				if uptime := time.Now().Unix() - bootTime; uptime > 0 {
					info.UptimeSeconds = uptime
				}
			}
		}
	}

	if pageSize > 0 {
		for _, line := range lines[3:] {
			matches := darwinFreePagesPattern.FindStringSubmatch(line)
			if len(matches) < 2 {
				continue
			}
			if pages, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				info.FreeMemoryBytes = pages * pageSize
			}
			break
		}
	}
}
