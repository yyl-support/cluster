package converter

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/gitcode"
	"go.yaml.in/yaml/v3"
)

func convertUsesToString(outerJobStep gitcode.Step, toolPath string) (string, error) {
	var actionYamlPath string
	if strings.HasPrefix(outerJobStep.Uses, "./") {
		// actionYamlPath = filepath.Clean(jobStep.Uses)

		actionYamlPath = filepath.Join(outerJobStep.Uses, "action.yaml")

	} else if strings.HasPrefix(outerJobStep.Uses, "actions") {
		actionYamlPath = filepath.Join(toolPath, outerJobStep.Uses, "action.yaml")
	}

	dir := filepath.Dir(actionYamlPath)
	if dir == "." {
		dir = ""
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("failed to open root directory: %v", err)
	}
	defer root.Close() //nolint:errcheck,gocritic

	filename := filepath.Base(actionYamlPath)
	f, err := root.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read action yaml: %v", err)
	}
	defer f.Close() //nolint:errcheck,gocritic
	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read action yaml: %v", err)
	}

	var action gitcode.Action
	err = yaml.Unmarshal(data, &action)
	if err != nil {
		return "", fmt.Errorf("failed to parse yaml: %v", err)
	}

	for key := range action.Inputs {
		if outerJobStep.With[key] != "" {
			action.Inputs[key].Default = outerJobStep.With[key]
		}
	}

	renderAction(&action)

	// // Render back to YAML
	// rendered, err := yaml.Marshal(&action)
	// if err != nil {
	// 	return "", fmt.Errorf("Failed to render YAML: %v", err)
	// }

	// fmt.Println("\n--- Rendered YAML ---")
	// fmt.Print(string(rendered))

	var cmdScript strings.Builder
	for _, jobStep := range action.Runs.Steps {

		cmdScript.WriteString("\n")
		if jobStep.Name != "" {
			cmdScript.WriteString("echo " + shellQuote("step: "+jobStep.Name) + "\n")
		}

		if outerJobStep.Uses == jobStep.Uses {
			return "", errors.New("error recursive call action : " + jobStep.Uses)
		}

		if jobStep.Uses != "" {

			script, err := convertUsesToString(jobStep, toolPath)
			if err != nil {
				fmt.Printf("读取action.yaml文件失败: %v\n", err)
				os.Exit(1) //nolint:gocritic
			}
			cmdScript.WriteString(script)
		} else {

			cmdScript.WriteString(jobStep.Run)
		}

	}

	return cmdScript.String(), err

}

func replaceInputAndEnv(s string, inputs map[string]*gitcode.Input) string {

	s = substituteInputExpressions(s, inputs)
	s = ReplaceEnvVarsInString(s)
	return s
}

// substituteInputExpressions replaces ${{ inputs.xxx }} with input defaults
func substituteInputExpressions(s string, inputs map[string]*gitcode.Input) string {
	if s == "" {
		return s
	}
	re := regexp.MustCompile(`\$\{\{\s*inputs\.([a-zA-Z0-9_]+)\s*\}\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		key := re.FindStringSubmatch(match)[1]
		if inp, ok := inputs[key]; ok {
			return inp.Default
		}
		log.Printf("Warning: undefined input '%s' in expression", key)
		return match // leave unresolved
	})
}

// renderAction substitutes all known expression fields using input defaults
func renderAction(action *gitcode.Action) {
	// Render step fields
	for i := range action.Runs.Steps {
		action.Runs.Steps[i].Name = replaceInputAndEnv(action.Runs.Steps[i].Name, action.Inputs)
		action.Runs.Steps[i].Run = replaceInputAndEnv(action.Runs.Steps[i].Run, action.Inputs)
		// Note: 'shell' is usually literal; GitHub doesn't evaluate expressions here
	}

	// Render output values
	// for k, out := range action.Outputs {
	// 	action.Outputs[k].Value = substituteInputExpressions(out.Value, action.Inputs)
	// }

	// Optionally: render description, name, etc.
	action.Name = replaceInputAndEnv(action.Name, action.Inputs)
	action.Description = substituteInputExpressions(action.Description, action.Inputs)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	// Handle embedded single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// 定义一个正则表达式，用于匹配 ${VAR} 格式的环境变量引用
// - \${...} 匹配字面量 ${ 和 }
// - ([^}]+) 捕获组，匹配 } 之前的一个或多个非 } 字符，作为环境变量名
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// replaceEnvVarsInString 扫描输入字符串，替换存在的环境变量，保留不存在或为空的引用
func ReplaceEnvVarsInString(input string) string {
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

		if envVarName == "WORKSPACE" {
			return match
		}

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

// ReplaceEnvVarsInStringSkippingSensitive 扫描输入字符串，替换存在的环境变量
// 但跳过敏感变量（通过 IsSensitiveEnvName 检测），保留其 ${VAR} 格式
func ReplaceEnvVarsInStringSkippingSensitive(input string) string {
	return envVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := envVarRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		envVarName := submatch[1]

		if envVarName == "WORKSPACE" {
			return match
		}

		if IsSensitiveEnvName(envVarName) {
			return match
		}

		envVarValue := os.Getenv(envVarName)
		if envVarValue != "" {
			return envVarValue
		}
		return match
	})
}
