# MCP Bash Server

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![Version](https://img.shields.io/badge/Version-0.1.0-green.svg)](https://github.com/pengcunfu/mcp-bash-server)

一个基于 MCP (Model Context Protocol) 的跨平台工具集合，用 Go 语言开发。它为 AI 提供一组基础的系统操作工具，屏蔽 Windows / Linux / macOS 的系统级差异，让 AI 以统一的方式执行命令、读写文件、查询进程与环境。

## 项目定位

**目标**：把平台差异收敛到服务端，让 AI 只关注「要做什么」，而不必关心「在哪个系统上怎么做」。

**当前实现**：

- **命令执行**：服务端按运行平台自动选择并调用 shell —— Windows 走 PowerShell，Linux / macOS 走 `sh` —— 调用方无需感知 shell 差异。命令内容由调用方提供，服务端原样执行；需要按平台调整命令语法时可先调用 `get_os_info`。
- **文件系统**：统一的文件读写、精确替换、目录遍历、glob 搜索与元信息查询。自动识别并解码 UTF-8 / UTF-8 BOM / UTF-16LE / UTF-16BE，写入时保持文件原编码；编辑时自动兼容 LF 与 CRLF 差异。隐藏文件在 Windows 按文件属性识别，其他平台按点号前缀识别。
- **进程与系统**：统一的进程列表、环境变量、磁盘容量与运行环境概况，屏蔽 `tasklist` / `ps` / `df` / CIM 等平台命令差异，返回结构化字段而非裸文本。
- **零运行时依赖**：仅使用 Go 标准库，编译为单一可执行文件。

## 主要功能

### ⚡ 命令执行
- **自动选择 shell**: 服务端按运行平台自动选择 PowerShell 或 sh，调用方无需关心
- **原样执行**: 直接执行提供的命令，不做任何转换或处理
- **超时控制**: 可配置命令执行超时时间（默认 30 秒）
- **工作目录**: 支持指定命令执行的工作目录
- **输出编码统一**: Windows 下强制 PowerShell 以 UTF-8 输出，避免中文等非 ASCII 内容乱码

### 📁 文件系统
- **读写与编辑**: 精确替换文本，写回时保持原编码与换行风格
- **目录遍历**: 列出目录或输出树形结构，便于快速了解项目布局
- **glob 搜索**: 支持 `*`、`?`、`[]` 与 `**` 任意层级匹配，可排除指定目录

### 🖥️ 进程与系统
- **进程列表**: 统一的 PID、名称、内存、命令行字段，支持按名称过滤
- **环境变量**: 支持按名称筛选，敏感变量自动脱敏
- **磁盘容量**: 返回总容量、已用、可用与使用率，单位统一为字节
- **运行环境**: 操作系统、架构、CPU 核心数、内存、运行时长等

## 安装和使用

### 1. 构建项目
```bash
cd mcp-bash-server
go mod tidy
go build -o mcp-bash-server .
```

### 2. 配置 MCP 客户端
将以下配置添加到你的 MCP 配置文件中：

```json
{
  "mcpServers": {
    "mcp-bash-server": {
      "command": "./mcp-bash-server"
    }
  }
}
```

### 3. 使用工具

#### 获取系统信息
```json
{
  "name": "get_system_info"
}
```

#### Windows PowerShell 命令
```json
{
  "name": "mcp-bash-server",
  "arguments": {
    "command": "Get-Process | Select-Object -First 5"
  }
}
```

#### Linux/macOS bash 命令
```json
{
  "name": "mcp-bash-server",
  "arguments": {
    "command": "ps aux | head -5"
  }
}
```

#### 指定工作目录和超时
```json
{
  "name": "mcp-bash-server",
  "arguments": {
    "command": "dir",
    "working_dir": "C:\\Users",
    "timeout": 10
  }
}
```

#### 读取文件（按行分页）
```json
{
  "name": "read_file",
  "arguments": {
    "path": "src/main.go",
    "offset": 1,
    "limit": 200
  }
}
```

#### 精确替换文件内容
```json
{
  "name": "edit_file",
  "arguments": {
    "path": "src/config.go",
    "old_string": "Timeout: 30",
    "new_string": "Timeout: 60"
  }
}
```

#### 搜索文件
```json
{
  "name": "search_files",
  "arguments": {
    "path": ".",
    "pattern": "**/*_test.go",
    "exclude": ["node_modules", "vendor"]
  }
}
```

## API 参考

### 工具列表

| 工具 | 分类 | 说明 |
|------|------|------|
| `mcp-bash-server` | 命令执行 | 跨平台执行系统命令 |
| `get_os_info` | 系统 | 获取操作系统版本详情 |
| `read_file` | 文件系统 | 读取文本文件，支持按行分页 |
| `write_file` | 文件系统 | 写入文本文件，自动创建目录 |
| `edit_file` | 文件系统 | 精确替换文件内容 |
| `list_dir` | 文件系统 | 列出目录下的直接子项 |
| `directory_tree` | 文件系统 | 输出目录树形结构 |
| `search_files` | 文件系统 | 按 glob 通配符递归查找文件 |
| `get_file_info` | 文件系统 | 获取文件或目录的元信息 |
| `list_processes` | 进程/系统 | 列出进程，支持名称过滤 |
| `get_env` | 进程/系统 | 读取环境变量，敏感值脱敏 |
| `get_disk_usage` | 进程/系统 | 获取磁盘容量与使用率 |
| `get_system_info` | 进程/系统 | 获取运行环境概况 |

### 1. mcp-bash-server
执行系统命令，服务端按平台自动选择 shell。

**输入参数**:
- `command` (必需): 要执行的命令
- `working_dir` (可选): 工作目录
- `timeout` (可选): 超时时间（秒，默认 30，范围 1 - 86400）

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

### 2. get_os_info
获取操作系统信息。

**输入参数**: 无

**输出格式**:
```json
{
  "os": "windows",
  "architecture": "amd64",
  "version": "Windows 11",
  "hostname": "DESKTOP-ABC123",
  "username": "user",
  "details": "Microsoft Windows 11 专业版"
}
```

### 3. read_file
读取文本文件内容，自动识别 UTF-8 / UTF-8 BOM / UTF-16LE / UTF-16BE。二进制文件只返回元信息，不返回内容。

**输入参数**:
- `path` (必需): 文件路径，支持 `~` 与相对路径
- `offset` (可选): 起始行号（从 1 开始，负数表示从末尾倒数，默认 1）
- `limit` (可选): 最多返回的行数（默认 1000，最大 5000）

**输出格式**:
```json
{
  "path": "D:\\project\\main.go",
  "size": 2048,
  "encoding": "utf-8",
  "binary": false,
  "total_lines": 120,
  "start_line": 1,
  "end_line": 120,
  "truncated": false,
  "content": "文件内容"
}
```

### 4. write_file
写入文本文件，父目录不存在时自动创建。目标文件原本是 UTF-8 BOM 或 UTF-16 时按原编码写回，原有权限会被保留。

**输入参数**:
- `path` (必需): 文件路径
- `content` (必需): 要写入的完整内容
- `create_dirs` (可选): 是否自动创建父目录（默认 true）

**输出格式**:
```json
{
  "path": "D:\\project\\new.txt",
  "bytes_written": 18,
  "created": true,
  "encoding": "utf-8"
}
```

### 5. edit_file
精确替换文件中的一段文本。匹配时自动兼容 LF 与 CRLF 差异，写回时保持原编码与换行风格。

**输入参数**:
- `path` (必需): 文件路径
- `old_string` (必需): 要被替换的原文本，必须与文件内容完全一致（含缩进）
- `new_string` (必需): 替换后的新文本，传空字符串表示删除
- `replace_all` (可选): 是否替换全部匹配项（默认 false，仅替换唯一匹配）

**输出格式**:
```json
{
  "path": "D:\\project\\main.go",
  "replacements": 1,
  "bytes_written": 2049
}
```

### 6. list_dir
列出目录下的直接子项，按名称排序。

**输入参数**:
- `path` (必需): 目录路径
- `show_hidden` (可选): 是否包含隐藏项（默认 false）

**输出格式**:
```json
{
  "path": "D:\\project",
  "count": 2,
  "entries": [
    {
      "name": "src",
      "path": "D:\\project\\src",
      "type": "dir",
      "size": 0,
      "modified": "2026-03-08T10:00:00+08:00"
    }
  ]
}
```

### 7. directory_tree
以树形文本展示目录结构。符号链接不会被递归展开，权限不足的目录会被跳过。

**输入参数**:
- `path` (必需): 根目录路径
- `depth` (可选): 递归深度，1 表示只列出直接子项（默认 3，最大 10）
- `show_hidden` (可选): 是否包含隐藏项（默认 false）

**输出格式**:
```json
{
  "root": "D:\\project",
  "tree": "project/\n├── src/\n│   └── main.go\n└── README.md\n",
  "dirs": 1,
  "files": 2,
  "truncated": false
}
```

### 8. search_files
按 glob 通配符递归查找文件。`pattern` 含路径分隔符时按相对路径匹配，否则只匹配文件名。遍历时默认跳过以 `.` 开头的目录，避免进入 `.git` 等目录。

**输入参数**:
- `pattern` (必需): glob 通配符，如 `*.go`、`**/*_test.go`、`src/**/config.*`
- `path` (可选): 搜索根目录（默认当前目录）
- `exclude` (可选): 排除的 glob 列表，如 `["node_modules", "*.log"]`
- `show_hidden` (可选): 是否进入隐藏目录（默认 false）
- `max_results` (可选): 最多返回的匹配数量（默认 100，最大 1000）

**输出格式**:
```json
{
  "root": "D:\\project",
  "pattern": "**/*.go",
  "count": 1,
  "truncated": false,
  "matches": [
    {
      "path": "D:\\project\\src\\main.go",
      "relative": "src/main.go"
    }
  ]
}
```

### 9. get_file_info
获取单个文件或目录的元信息，符号链接返回其指向目标而不跟随解析。

**输入参数**:
- `path` (必需): 文件或目录路径

**输出格式**:
```json
{
  "path": "D:\\project\\main.go",
  "name": "main.go",
  "type": "file",
  "size": 2048,
  "mode": "-rw-rw-rw-",
  "permissions": "0666",
  "modified": "2026-03-08T10:00:00+08:00",
  "is_symlink": false
}
```

### 10. list_processes
列出当前系统中的进程。Windows 使用 `tasklist`，Linux / macOS 使用 `ps`，返回统一字段。`user`、`cpu_percent`、`mem_percent` 等字段在平台上不可用时会省略。

**输入参数**:
- `filter` (可选): 按进程名或命令行做不区分大小写的子串过滤

**输出格式**:
```json
{
  "platform": "windows",
  "filter": "node",
  "count": 1,
  "processes": [
    {
      "pid": 12345,
      "name": "node.exe",
      "mem_bytes": 52428800,
      "command": "node.exe"
    }
  ]
}
```

### 11. get_env
读取环境变量，默认对包含 TOKEN、SECRET、PASSWORD、API_KEY 等敏感字样的变量值脱敏。

**输入参数**:
- `keys` (可选): 只读取指定的变量名列表（默认返回全部）
- `mask` (可选): 是否对敏感变量的值脱敏（默认 true）

**输出格式**:
```json
{
  "count": 2,
  "variables": {
    "PATH": "C:\\Windows\\system32",
    "GITHUB_TOKEN": "****"
  },
  "masked_keys": ["GITHUB_TOKEN"]
}
```

### 12. get_disk_usage
获取磁盘容量信息，单位统一为字节。不传 `path` 时返回所有磁盘，传入 `path` 时只返回该路径所在磁盘。

**输入参数**:
- `path` (可选): 要查询的路径

**输出格式**:
```json
{
  "count": 1,
  "disks": [
    {
      "device": "C:",
      "mount": "C:",
      "file_system": "NTFS",
      "total_bytes": 192334000128,
      "used_bytes": 173004574720,
      "free_bytes": 19329425408,
      "used_percent": 90
    }
  ]
}
```

### 13. get_system_info
获取运行环境概况，用于判断资源情况。系统版本详情请使用 `get_os_info`。无法获取的字段会被省略。

**输入参数**: 无

**输出格式**:
```json
{
  "os": "windows",
  "architecture": "amd64",
  "version": "Windows 11",
  "hostname": "Peng-PC",
  "username": "pcf",
  "home_dir": "C:\\Users\\pcf",
  "working_dir": "D:\\project",
  "cpu_cores": 32,
  "go_version": "go1.24.5",
  "total_memory_bytes": 34134630400,
  "free_memory_bytes": 6407782400,
  "uptime_seconds": 273997
}
```

## 使用建议

### 💡 最佳实践
1. **命令由服务端分发**: 无需指定 shell，服务端会自动选择 PowerShell 或 sh
2. **优先使用结构化工具**: 读写文件、遍历目录、查询进程等场景优先使用对应工具，无需自行拼装平台命令
3. **按平台调整命令语法**: 命令内容仍与平台相关（如 Windows 用 `Get-Process`，Linux 用 `ps aux`）；需要确认平台时先调用 `get_os_info`
4. **设置合理超时**: 根据命令复杂度设置适当的超时时间

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

运行单元测试：

```bash
go test ./...
```

同时建议执行静态检查：

```bash
go vet ./...
```

`test_commands.go` 保留为早期的手工调试入口，需要单独编译时请将其与主程序一起构建。

## 开发

### 项目结构
```
mcp-bash-server/
├── main.go                 # 服务器入口、工具注册与请求路由
├── protocol.go             # MCP 协议数据结构
├── util.go                 # 参数解析、路径处理与外部命令调用
├── text.go                 # 文本编码与换行符处理
├── tools_exec.go           # 命令执行与系统版本信息
├── tools_fs.go             # 文件系统类工具
├── tools_system.go         # 进程与系统类工具
├── hidden_windows.go       # Windows 隐藏文件识别
├── hidden_other.go         # 非 Windows 隐藏文件识别
├── test_commands.go        # 早期手工调试入口
├── *_test.go               # 单元测试
├── go.mod                  # Go 模块文件
└── README.md               # 文档
```

### 扩展功能

新增一个工具只需两步：

1. 在对应的 `tools_*.go` 中实现处理函数，签名为 `func(args map[string]interface{}) (interface{}, error)`
2. 在同一个文件的 `*Tools()` 方法里追加 `RegisteredTool`（含 `Tool` 描述与 `Handler`）

`tools/list` 与 `tools/call` 会自动包含新工具，无需修改路由代码。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 Apache License 2.0 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 作者

- **pengcunfu** - [GitHub](https://github.com/pengcunfu)

## 仓库

- GitHub: [https://github.com/pengcunfu/mcp-bash-server](https://github.com/pengcunfu/mcp-bash-server)
