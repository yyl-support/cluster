${yaml_path}
${job_name}


cd workspace
git clone https://gitcode.com/ascend-archive/CI.git 


envrender CI/${yaml_path} pipeline-rendered.yaml



./convert2_to_yaml CI/${yaml_path} workflow.yaml

