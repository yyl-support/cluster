package converter

import (
	"os"
	"regexp"
	"strings"
)

var sensitivePatterns = []string{
	// "password",
	// "passwd",
	// "token",
	// "access",
	// "ak",
	// "sk",
	// "secret",
	// "key",
	// "credential",
}

var (
	exportRegex = regexp.MustCompile(`(?m)^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	refRegex    = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

func IsSensitiveEnvName(name string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func FilterSensitiveEnv(env map[string]string) (sensitive, plain map[string]string) {
	sensitive = make(map[string]string)
	plain = make(map[string]string)

	for k, v := range env {
		if v == "" {
			continue
		}
		if IsSensitiveEnvName(k) {
			sensitive[k] = v
		} else {
			plain[k] = v
		}
	}

	return sensitive, plain
}

func ExtractSensitiveFromScript(script string) (sensitive map[string]string, cleanedScript string) {
	sensitive = make(map[string]string)
	lines := strings.Split(script, "\n")
	var cleanedLines []string

	for _, line := range lines {
		matches := exportRegex.FindAllStringSubmatch(line, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				if len(match) >= 3 {
					varName := match[1]
					_ = strings.Trim(match[2], `"'`)

					if IsSensitiveEnvName(varName) {
						sensitive[varName] = os.Getenv(varName)
						continue
					}
				}
				cleanedLines = append(cleanedLines, line)
			}
		} else {
			cleanedLines = append(cleanedLines, line)
		}
	}

	refMatches := refRegex.FindAllStringSubmatch(script, -1)
	for _, match := range refMatches {
		if len(match) >= 2 {
			varName := match[1]
			if IsSensitiveEnvName(varName) {
				if _, exists := sensitive[varName]; !exists {
					sensitive[varName] = os.Getenv(varName)
				}
			}
		}
	}

	cleanedScript = strings.Join(cleanedLines, "\n")
	return sensitive, cleanedScript
}

func MergeSensitiveEnvs(env1, env2 map[string]string) map[string]string {
	merged := make(map[string]string)

	for k, v := range env1 {
		merged[k] = v
	}

	for k, v := range env2 {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}

	return merged
}

func ResolveSensitiveEnvValues(env map[string]string) map[string]string {
	resolved := make(map[string]string)

	for k, v := range env {
		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			varName := v[2 : len(v)-1]
			if varName != "" {
				if actualValue := os.Getenv(varName); actualValue != "" {
					resolved[k] = actualValue
					continue
				}
			}
		}
		resolved[k] = v
	}

	return resolved
}
