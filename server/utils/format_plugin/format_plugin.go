package format_plugin

import (
	"strconv"
	"strings"
	"unicode"
)

// recover错误，转string
func ErrorToString(r interface{}) string {
	switch t := r.(type) {
	case error:
		return t.Error()
	default:
		return r.(string)
	}
}

// 单词全部转化为大写
func ToUpper(s string) string {
	return strings.ToUpper(s)
}

// 单词全部转化为小写
func ToLower(s string) string {
	return strings.ToLower(s)
}

// 下划线单词转为大写驼峰单词
func UderscoreToUpperCamelCase(s string) string {
	s = strings.Replace(s, "_", " ", -1)
	s = strings.Title(s)
	return strings.Replace(s, " ", "", -1)
}

// 下划线单词转为小写驼峰单词
func UderscoreToLowerCamelCase(s string) string {
	s = UderscoreToUpperCamelCase(s)
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
	return s
}

// 驼峰单词转下划线单词
func CamelCaseToUdnderscore(s string) string {
	var output []rune
	for i, r := range s {
		if i == 0 {
			output = append(output, unicode.ToLower(r))
		} else {
			if unicode.IsUpper(r) {
				output = append(output, '_')
			}

			output = append(output, unicode.ToLower(r))
		}
	}
	return string(output)
}

// 流量 Byte -> GB
func ByteToGB(b int64) string {
	b1 := (float64(b)) / 1024 / 1024 / 1024
	return strconv.FormatFloat(b1, 'f', 2, 64)
}
