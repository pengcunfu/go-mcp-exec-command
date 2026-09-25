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
	"fmt"
	"runtime"
)

// 测试直接命令执行
func TestDirectExecution() {
	server := &BashServer{}
	
	fmt.Println("=== 测试直接命令执行 ===")
	fmt.Printf("当前平台: %s\n", runtime.GOOS)
	
	// 测试简单命令
	testCases := []string{
		"echo hello world",
		"dir",  // Windows 命令
	}
	
	for _, cmd := range testCases {
		fmt.Printf("测试命令: %s\n", cmd)
		req := CommandRequest{
			Command: cmd,
			Timeout: 10,
		}
		
		result := server.executeCommand(req)
		fmt.Printf("执行结果: 成功=%t, 输出长度=%d\n\n", result.Success, len(result.Output))
	}
}

// 测试操作系统信息获取功能
func TestOSInfo() {
	server := &BashServer{}
	
	fmt.Println("=== 测试操作系统信息获取 ===")
	
	// 获取操作系统信息
	osInfo := server.getOSInfo()
	
	// 格式化输出
	jsonData, err := json.MarshalIndent(osInfo, "", "  ")
	if err != nil {
		fmt.Printf("JSON序列化失败: %v\n", err)
		return
	}
	
	fmt.Println("操作系统信息:")
	fmt.Println(string(jsonData))
	
	fmt.Println("=== 操作系统信息测试完成 ===")
}

func RunTests() {
	fmt.Println("=== 测试直接命令执行 ===")
	TestDirectExecution()
	
	fmt.Println("=== 测试操作系统信息 ===")
	TestOSInfo()
}
