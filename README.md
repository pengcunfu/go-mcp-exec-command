# Go MCP Exec Command

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![Version](https://img.shields.io/badge/Version-0.0.1-green.svg)](https://github.com/pengcunfu/go-mcp-exec-command)

一个用 Go 语言开发的 MCP (Model Context Protocol) 服务器，用于直接执行系统命令。

## 主要功能

### ⚡ 直接命令执行
- **原样执行**: 直接执行提供的命令，不做任何转换或处理
- **跨平台支持**: 
  - **Windows**: 使用 PowerShell 执行命令
  - **Linux/macOS**: 使用 bash 执行命令
- **系统信息获取**: 提供详细的操作系统信息

### 🛠️ 核心特性
- **超时控制**: 可配置命令执行超时时间（默认30秒）
- **工作目录**: 支持指定命令执行的工作目录
- **错误处理**: 完整的错误信息和退出码
- **执行统计**: 提供命令执行时间和平台信息

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
      "command": "./exec-command-server"
    }
  }
}
```

### 3. 使用工具

#### 获取系统信息
```json
{
  "name": "get_os_info"
}
```

#### Windows PowerShell 命令
```json
{
  "name": "exec_command",
  "arguments": {
    "command": "Get-Process | Select-Object -First 5"
  }
}
```

#### Linux/macOS bash 命令
```json
{
  "name": "exec_command", 
  "arguments": {
    "command": "ps aux | head -5"
  }
}
```

#### 指定工作目录和超时
```json
{
  "name": "exec_command",
  "arguments": {
    "command": "dir",
    "working_dir": "C:\\Users",
    "timeout": 10
  }
}
```

## API 参考

### 工具列表

#### 1. exec_command
执行系统命令

**输入参数**:
- `command` (必需): 要执行的命令
- `working_dir` (可选): 工作目录
- `timeout` (可选): 超时时间（秒，默认30）

**输出格式**:
```json
{
  "success": true,
  "output": "命令输出内容",
  "error": "错误信息（如果有）",
  "exit_code": 0,
  "duration": "123ms",
  "command": "执行的命令",
  "platform": "windows"
}
```

#### 2. get_os_info
获取操作系统信息

**输入参数**: 无

**输出格式**:
```json
{
  "os": "windows",
  "architecture": "amd64",
  "version": "Windows 11",
  "hostname": "DESKTOP-ABC123",
  "username": "user",
  "details": "详细系统信息..."
}
```

## 使用建议

### 💡 最佳实践
1. **先获取系统信息**: 使用 `get_os_info` 了解目标系统
2. **使用对应命令**: 
   - Windows 系统使用 PowerShell 命令
   - Linux/macOS 系统使用 bash 命令
3. **设置合理超时**: 根据命令复杂度设置适当的超时时间

### 🔧 命令示例

**Windows PowerShell 命令**:
- `Get-Process` - 获取进程列表
- `Get-ChildItem` - 列出文件和目录
- `Test-Path "C:\path"` - 测试路径是否存在
- `New-Item -ItemType Directory -Path "C:\newdir"` - 创建目录

**Linux/macOS bash 命令**:
- `ps aux` - 获取进程列表
- `ls -la` - 列出文件和目录
- `test -d /path` - 测试目录是否存在
- `mkdir -p /newdir` - 创建目录

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
└── README.md           # 文档
```

### 扩展功能
你可以通过修改以下函数来扩展功能：
- `executeCommand()`: 修改命令执行逻辑
- `getOSInfo()`: 增强系统信息获取
- `getWindowsVersion()`, `getLinuxVersion()`, `getMacOSVersion()`: 改进版本检测

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 Apache License 2.0 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 作者

- **pengcunfu** - [GitHub](https://github.com/pengcunfu)

## 仓库

- GitHub: [https://github.com/pengcunfu/go-mcp-exec-command](https://github.com/pengcunfu/go-mcp-exec-command)
