package converter

import (
	"crypto/sha256"
	"fmt"
)

func BuildArtifactPVCName(pipelineRunID, uniqueID string) string {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(pipelineRunID+"-"+uniqueID)))
	shortHash := hash[:16]
	name := fmt.Sprintf("pipeline-artifact-%s-%s", pipelineRunID, shortHash)
	if len(name) > 253 {
		name = name[:253]
	}
	return name
}

func BuildArtifactPVCManifest(pvcName, workflowName, workflowUID, namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  namespace: %s
  ownerReferences:
    - apiVersion: argoproj.io/v1alpha1
      kind: Workflow
      name: %s
      uid: %s
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  storageClassName: sfsturbo-subpath-sc
`, pvcName, namespace, workflowName, workflowUID)
}
