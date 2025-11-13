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

// 测试跨平台命令转换
func TestCommandConversion() {
	server := &ExecCommandServer{}
	
	testCases := []struct {
		input    string
		expected string
		platform string
	}{
		{
			input:    "ls -la && cat file.txt",
			expected: "dir ; type file.txt",
			platform: "windows",
		},
		{
			input:    "mkdir -p test && cd test",
			expected: "mkdir test ; cd test",
			platform: "windows",
		},
		{
			input:    "grep 'pattern' file.txt && echo 'found'",
			expected: "findstr 'pattern' file.txt ; echo 'found'",
			platform: "windows",
		},
	}
	
	for _, tc := range testCases {
		if runtime.GOOS == "windows" {
			result := server.convertCommand(tc.input)
			fmt.Printf("输入: %s\n输出: %s\n期望: %s\n匹配: %t\n\n", 
				tc.input, result, tc.expected, result == tc.expected)
		}
	}
}

// 测试引号处理
func TestQuoteHandling() {
	server := &ExecCommandServer{}
	
	testCases := []string{
		`Invoke-Session "echo \"hello world\""`,
		`"echo \"nested quotes\""`,
		`echo "simple quotes"`,
	}
	
	for _, tc := range testCases {
		result := server.sanitizeCommand(tc)
		fmt.Printf("输入: %s\n输出: %s\n\n", tc, result)
	}
}

// 测试操作系统信息获取功能
func TestOSInfo() {
	server := &ExecCommandServer{}
	
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
	
	fmt.Println("=== 操作系统信息测试完成 ===\n")
}

func RunTests() {
	fmt.Println("=== 测试跨平台命令转换 ===")
	TestCommandConversion()
	
	fmt.Println("=== 测试引号处理 ===")
	TestQuoteHandling()
	
	fmt.Println("=== 测试操作系统信息 ===")
	TestOSInfo()
}
