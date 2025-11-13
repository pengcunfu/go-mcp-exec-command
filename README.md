# Go MCP Exec Command

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![Version](https://img.shields.io/badge/Version-0.0.1-green.svg)](https://github.com/pengcunfu/go-mcp-exec-command)

一个用 Go 语言开发的 MCP (Model Context Protocol) 服务器，专门用于执行系统命令并解决跨平台兼容性问题。

## 主要功能

### 🔧 跨平台命令转换
- **Linux `&&` 转 Windows `;`**: 自动将 Linux 风格的命令连接符转换为 Windows 兼容格式
- **常见命令映射**: 自动转换常见的 Linux 命令到 Windows 等价命令
  - `ls` → `dir`
  - `cat` → `type`
  - `grep` → `findstr`
  - `which` → `where`
  - `rm` → `del`
  - `cp` → `copy`
  - `mv` → `move`
  - 等等...

### 🛡️ Windsurf 兼容性
- **Invoke-Session 处理**: 自动处理 Windsurf 添加的 `Invoke-Session ""` 前缀
- **引号转义**: 智能处理嵌套引号问题，避免命令执行失败
- **特殊字符处理**: 正确处理命令中的特殊字符

### ⚡ 其他特性
- **超时控制**: 可配置命令执行超时时间
- **工作目录**: 支持指定命令执行的工作目录
- **详细输出**: 提供执行时间、平台信息等调试信息
- **错误处理**: 完整的错误信息和退出码

## 安装和使用

### 1. 构建项目
```bash
cd go-mcp-exec-command
go mod tidy
go build -o exec-command-server main.go
```

### 2. 配置 MCP 客户端
将以下配置添加到你的 MCP 配置文件中：

```json
{
  "mcpServers": {
    "go-mcp-exec-command": {
      "command": "go",
      "args": ["run", "main.go"],
      "cwd": "d:\\Data\\Desktop\\Plugins\\go-mcp-exec-command"
    }
  }
}
```

### 3. 使用工具

#### 基本用法
```json
{
  "command": "ls -la && cat README.md"
}
```

#### 高级用法
```json
{
  "command": "mkdir -p test && cd test && echo 'hello' > file.txt",
  "working_dir": "/path/to/directory",
  "timeout": 60,
  "auto_convert": true
}
```

## API 参数

### 输入参数
- `command` (必需): 要执行的命令
- `working_dir` (可选): 工作目录
- `timeout` (可选): 超时时间（秒，默认30）
- `auto_convert` (可选): 是否自动转换跨平台命令（默认true）

### 输出格式
```json
{
  "success": true,
  "output": "命令输出内容",
  "error": "错误信息（如果有）",
  "exit_code": 0,
  "duration": "123ms",
  "command": "实际执行的命令",
  "platform": "windows"
}
```

## 智能命令处理

### 🧠 智能检测机制
本工具采用智能检测机制，区分不同的命令执行场景：

1. **Windsurf 直接执行**: 自动检测 `Invoke-Session` 前缀并进行清理
2. **MCP 工具执行**: 直接执行用户提供的正常命令
3. **跨平台转换**: 只在检测到 Linux 命令时才进行转换

### 🔍 检测规则
- **Invoke-Session 检测**: 自动识别并处理 Windsurf 添加的前缀
- **Linux 命令检测**: 智能识别 `ls`、`cat`、`grep`、`&&` 等 Linux 特有语法
- **按需转换**: 只有在 Windows 平台且检测到 Linux 命令时才进行转换

## 解决的问题

### 1. Windsurf Invoke-Session 问题
**问题**: Windsurf 在直接执行命令时会添加 `Invoke-Session ""` 前缀，导致包含引号的命令执行失败。

**解决方案**: 
- 智能检测 `Invoke-Session` 前缀
- 自动清理和转义特殊字符
- 处理嵌套引号问题

### 2. 跨平台命令兼容性
**问题**: 大模型经常生成 Linux 风格的命令，在 Windows 上无法直接执行。

**解决方案**: 
- 智能检测 Linux 命令和语法
- 自动转换为 Windows 等价命令
- 支持 `&&` 转 `;`、`ls` 转 `dir` 等常见转换

### 3. 命令执行场景区分
**问题**: 不同执行场景需要不同的处理方式。

**解决方案**: 
- 区分 Windsurf 直接执行和 MCP 工具执行
- 只在必要时进行命令处理和转换
- 保持原始命令的完整性

## 测试

运行测试以验证功能：

```bash
go run test_commands.go
```

## 开发

### 项目结构
```
go-mcp-exec-command/
├── main.go              # 主服务器代码
├── test_commands.go     # 测试代码
├── go.mod              # Go 模块文件
├── mcp-config.json     # MCP 配置示例
└── README.md           # 文档
```

### 扩展功能
你可以通过修改以下函数来扩展功能：
- `convertLinuxCommands()`: 添加更多命令映射
- `sanitizeCommand()`: 增强命令清理逻辑
- `handleNestedQuotes()`: 改进引号处理

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 Apache License 2.0 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 作者

- **pengcunfu** - [GitHub](https://github.com/pengcunfu)

## 仓库

- GitHub: [https://github.com/pengcunfu/go-mcp-exec-command](https://github.com/pengcunfu/go-mcp-exec-command)
