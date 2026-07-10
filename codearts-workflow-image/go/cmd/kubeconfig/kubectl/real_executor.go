package kubectl

import (
	"context"
	"fmt"
	"os/exec"
)

type RealExecutor struct {
	Kubeconfig string
}

func (r *RealExecutor) Exec(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"--kubeconfig=" + r.Kubeconfig}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("kubectl 执行失败: %s, stderr: %s", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("kubectl 执行失败: %w", err)
	}
	return out, nil
}
