package converter

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
)

func convertJobArch(s string) map[string]string {
	tokens := strings.Split(s, "-")
	arch := tokens[0]

	if arch != "amd64" && arch != "arm64" {
		fmt.Println("wrong arch:")
		os.Exit(1) //nolint:gocritic
	}

	result := map[string]string{
		"kubernetes.io/arch": arch,
	}

	for i := 1; i < len(tokens)-1; i++ {
		maybeChip := tokens[i]
		if runonparser.KnownChipNames[strings.ToLower(maybeChip)] {
			if _, err := strconv.Atoi(tokens[i+1]); err == nil {
				result["node.kubernetes.io/npu.chip.name"] = strings.ToUpper(maybeChip)
				break
			}
		}
	}

	return result
}
