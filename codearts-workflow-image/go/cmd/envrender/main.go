//go:build debug

package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
)

// 定义一个正则表达式，用于匹配 ${VAR} 格式的环境变量引用
// - \${...} 匹配字面量 ${ 和 }
// - ([^}]+) 捕获组，匹配 } 之前的一个或多个非 } 字符，作为环境变量名
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// replaceEnvVarsInString 扫描输入字符串，替换存在的环境变量，保留不存在或为空的引用
func replaceEnvVarsInString(input string) string {
	// ReplaceAllStringFunc 对匹配到的每一个子串调用提供的函数
	return envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		// match 是整个匹配项，例如 "${ENV_VAR}"
		// submatch[1] 是第一个捕获组，即环境变量名，例如 "ENV_VAR"
		submatch := envVarRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			// 理论上不应该发生，但为了安全起见
			return match
		}
		envVarName := submatch[1]

		// 获取环境变量的值
		envVarValue := os.Getenv(envVarName)

		// 检查环境变量是否存在且非空
		if envVarValue != "" {
			// 如果存在且非空，则返回其值
			return envVarValue
		}
		// 如果不存在或为空，则返回原始的 ${VAR} 引用
		return match
	})
}

func main() {
	// 检查命令行参数数量
	if len(os.Args) != 3 {
		log.Fatalf("用法: %s <source.yaml> <target.yaml>", os.Args[0])
	}

	sourceFile := os.Args[1] // 第一个参数是源文件
	targetFile := os.Args[2] // 第二个参数是目标文件

	// 1. 读取原始 YAML 文件
	yamlFile, err := os.ReadFile(sourceFile)
	if err != nil {
		log.Fatalf("读取源文件 %s 失败: %v", sourceFile, err)
	}

	// 2. 将 []byte 转换为 string
	yamlString := string(yamlFile)

	// 3. 使用自定义函数渲染环境变量
	renderedYamlString := replaceEnvVarsInString(yamlString)

	// 4. 将渲染后的字符串写入目标 YAML 文件
	if err := os.WriteFile(targetFile, []byte(renderedYamlString), 0644); err != nil {
		log.Fatalf("写入目标文件 %s 失败: %v", targetFile, err)
	}

	fmt.Printf("YAML 文件已从 %s 渲染并写入到 %s\n", sourceFile, targetFile)
}
