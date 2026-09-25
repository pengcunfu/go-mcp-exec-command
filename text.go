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
	"bytes"
	"os"
	"strings"
	"unicode/utf16"
)

// 文本编码与换行符处理。
//
// 不同平台生成的文件在编码上差异很大（Windows 记事本会写 UTF-16LE 或带 BOM 的 UTF-8），
// 这里统一在读写时转换，使调用方始终以 UTF-8 文本交互，同时保留原文件编码。

type textEncoding string

const (
	encodingUTF8    textEncoding = "utf-8"
	encodingUTF8BOM textEncoding = "utf-8-bom"
	encodingUTF16LE textEncoding = "utf-16le"
	encodingUTF16BE textEncoding = "utf-16be"
	encodingBinary  textEncoding = "binary"
)

func (e textEncoding) String() string { return string(e) }

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// detectEncoding 通过 BOM 判断文本编码，无 BOM 时按 UTF-8 处理。
func detectEncoding(data []byte) textEncoding {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		return encodingUTF8BOM
	case bytes.HasPrefix(data, bomUTF16LE):
		return encodingUTF16LE
	case bytes.HasPrefix(data, bomUTF16BE):
		return encodingUTF16BE
	}
	return encodingUTF8
}

// decodeText 将文件字节解码为 UTF-8 文本，并返回原编码以便写回时还原。
func decodeText(data []byte) (string, textEncoding) {
	encoding := detectEncoding(data)
	switch encoding {
	case encodingUTF8BOM:
		return string(data[len(bomUTF8):]), encoding
	case encodingUTF16LE:
		return decodeUTF16(data[len(bomUTF16LE):], false), encoding
	case encodingUTF16BE:
		return decodeUTF16(data[len(bomUTF16BE):], true), encoding
	}
	return string(data), encoding
}

func decodeUTF16(data []byte, bigEndian bool) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if bigEndian {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		} else {
			units = append(units, uint16(data[i+1])<<8|uint16(data[i]))
		}
	}
	return string(utf16.Decode(units))
}

// encodeText 按指定编码把 UTF-8 文本写回字节，UTF-16 编码会带上对应 BOM。
func encodeText(text string, encoding textEncoding) []byte {
	switch encoding {
	case encodingUTF8BOM:
		return append(append([]byte{}, bomUTF8...), text...)
	case encodingUTF16LE:
		return encodeUTF16(text, false)
	case encodingUTF16BE:
		return encodeUTF16(text, true)
	}
	return []byte(text)
}

func encodeUTF16(text string, bigEndian bool) []byte {
	units := utf16.Encode([]rune(text))
	output := make([]byte, 0, len(units)*2+2)
	if bigEndian {
		output = append(output, bomUTF16BE...)
	} else {
		output = append(output, bomUTF16LE...)
	}
	for _, unit := range units {
		if bigEndian {
			output = append(output, byte(unit>>8), byte(unit))
		} else {
			output = append(output, byte(unit), byte(unit>>8))
		}
	}
	return output
}

// looksBinary 通过采样判断是否为二进制内容，UTF-16 文件已在上层按文本处理。
func looksBinary(data []byte) bool {
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	return bytes.IndexByte(sample, 0x00) >= 0
}

// loadTextFile 读取文件并解码为 UTF-8 文本，binary 为 true 时不返回内容。
func loadTextFile(path string) (content string, encoding textEncoding, binary bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", encodingUTF8, false, err
	}
	if detectEncoding(data) == encodingUTF8 && looksBinary(data) {
		return "", encodingBinary, true, nil
	}
	text, encoding := decodeText(data)
	return text, encoding, false, nil
}

// fileEncoding 读取已有文件的编码，用于写回时保持原编码。
func fileEncoding(path string) textEncoding {
	file, err := os.Open(path)
	if err != nil {
		return encodingUTF8
	}
	defer file.Close()

	head := make([]byte, 3)
	count, _ := file.Read(head)
	return detectEncoding(head[:count])
}

// dominantEOL 返回文本中出现次数更多的换行符。
func dominantEOL(text string) string {
	crlf := strings.Count(text, "\r\n")
	lf := strings.Count(text, "\n") - crlf
	if crlf > lf {
		return "\r\n"
	}
	return "\n"
}

// normalizeEOL 将文本换行符统一为指定形式，使编辑操作不受平台换行差异影响。
func normalizeEOL(text, eol string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if eol == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// splitLines 按 \n 切分文本，忽略末尾换行产生的空行，保留行内的 \r。
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if last := len(lines) - 1; lines[last] == "" {
		lines = lines[:last]
	}
	return lines
}
