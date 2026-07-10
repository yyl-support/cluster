# add new label
1. rename env pipeline_run_id to CP_pipeline_run_id
2. add env CP_merge_id to argoworkflow label metadata.labels.jobPRID: CP_merge_id like `metadata.labels.jobPRID: "15"`
3. add env CP_repo_url ,extract org/repo from the url, add it to argoworkflow lable metadata.labels.jobRepositoryName org/repo like `metadata.labels.jobRepositoryName: "org/repo"`
4. argoWorkflow.Metadata.GenerateName  set as `org-repo-`