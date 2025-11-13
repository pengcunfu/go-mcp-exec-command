package main

import (
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

func RunTests() {
	fmt.Println("=== 测试跨平台命令转换 ===")
	TestCommandConversion()
	
	fmt.Println("=== 测试引号处理 ===")
	TestQuoteHandling()
}
