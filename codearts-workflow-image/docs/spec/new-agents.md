# new-agents.md
1. here is project to convert different things to become the part of argoworkflow
    - most source is file and env var
    - the output is crd workflow of argoworkflow and secret.yaml or other k8s resource
2. workflow
    - TDD for this project
    - convert test cover the case end to end every time when adding new feature
        - `go/cmd/converter/convertv2_to_yaml_test.go` this will output new yaml for 
        - every case should be tested by /submit-test skill 
            - the yaml can be submit and validate the result
    - unitest cover case of every the func when adding new feature
