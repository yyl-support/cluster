#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

get_image_proxy_url() {
    local kubeconfig="$1"
    
    if [[ -n "${CP_image_proxy}" ]]; then
        echo "${CP_image_proxy}"
    elif [[ "$kubeconfig" == *"/006.yaml" ]] || [[ "$kubeconfig" == *"karmada-proxy.config" ]]; then
        echo "harbor-portal.test.osinfra.cn"
    else
        echo "harbor-portal.osinfra.cn"
    fi
}

argo_submit() {
    local workflow_file="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local cp_workspace="${4:-}"
    local cp_artifacts_temp_folder="${5:-}"
    local secret_file="${6:-}"

    if [[ ! -f "$workflow_file" ]]; then
        log_error "Workflow file not found: $workflow_file"
        return 1
    fi

    local workflow_name_file
    workflow_name_file=$(mktemp)

    local image_proxy_url
    image_proxy_url=$(get_image_proxy_url "$kubeconfig")

    local submit_cmd=(go run ./cmd/submit
        --namespace "$namespace"
        --work-dir "$PROJECT_ROOT"
        --workflow-output "$workflow_file"
        --kubeconfig-path "$kubeconfig"
        --workflow-name-file "$workflow_name_file"
        --image-proxy-url "$image_proxy_url")

    if [[ -n "$secret_file" ]]; then
        submit_cmd+=(--secret-file "$secret_file")
    fi

    if [[ -n "$cp_artifacts_temp_folder" ]]; then
        submit_cmd+=(--cp-artifacts-temp-folder "$cp_artifacts_temp_folder")
    fi

    if [[ -n "$cp_workspace" ]]; then
        submit_cmd+=(--cp-workspace "$cp_workspace")
    fi

    local exit_code_file
    exit_code_file=$(mktemp)

    set +e
    submit_output=$(
        cd "$PROJECT_ROOT/go" && "${submit_cmd[@]}"
        echo $? > "$exit_code_file"
    2>&1)
    local submit_exit=$(cat "$exit_code_file")
    rm -f "$exit_code_file"
    set -e

    if [[ -n "$submit_output" ]]; then
        printf '%s\n' "$submit_output" >&2
    fi

    local workflow_name=""
    if [[ -f "$workflow_name_file" ]]; then
        workflow_name=$(tr -d ' \n' < "$workflow_name_file")
        rm -f "$workflow_name_file"
    fi

    if [[ -z "$workflow_name" ]]; then
        log_error "Submit command did not return workflow name"
        return 1
    fi

    echo "$workflow_name"
    return $submit_exit
}

argo_get_uid() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    local uid
    uid=$(kubectl get workflow "$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" -o jsonpath='{.metadata.uid}' 2>/dev/null)

    if [[ -z "$uid" ]]; then
        log_error "Could not get UID for workflow: $workflow_name"
        return 1
    fi

    echo "$uid"
}

argo_wait() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local timeout="${4:-300}"

    local elapsed=0
    local interval=10

    while [[ $elapsed -lt $timeout ]]; do
        local status
        status=$(kubectl get workflow "$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" -o jsonpath='{.status.phase}' 2>/dev/null)

        case "$status" in
            Succeeded)
                echo "Succeeded"
                return 0
                ;;
            Failed)
                echo "Failed"
                return 1
                ;;
            Error)
                echo "Error"
                return 1
                ;;
        esac

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_error "Workflow wait timeout after ${timeout}s"
    echo "Timeout"
    return 1
}

argo_delete() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    argo delete "$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" 2>/dev/null
    return 0
}

apply_secret() {
    local secret_file="$1"
    local namespace="$2"
    local kubeconfig="$3"

    if [[ ! -f "$secret_file" ]]; then
        return 0
    fi

    log_info "Applying secret: $secret_file"
    kubectl apply -f "$secret_file" --kubeconfig "$kubeconfig" -n "$namespace" 2>&1

    if [[ $? -ne 0 ]]; then
        log_error "Failed to apply secret"
        return 1
    fi

    log_success "Secret applied"
    return 0
}

apply_pvc() {
    local pvc_file="$1"
    local uid="$2"
    local namespace="$3"
    local kubeconfig="$4"
    local workflow_name="$5"

    if [[ ! -f "$pvc_file" ]]; then
        log_error "PVC file not found: $pvc_file"
        return 1
    fi

    local tmp_pvc
    tmp_pvc=$(mktemp)
    sed "s/\${WORKFLOW_NAME}/$workflow_name/g; s/\${WORKFLOW_UID}/$uid/g" "$pvc_file" > "$tmp_pvc"

    local pvc_name
    pvc_name=$(grep "^  name:" "$tmp_pvc" | awk '{print $2}')

    kubectl apply -f "$tmp_pvc" --kubeconfig "$kubeconfig" -n "$namespace" 2>&1
    local apply_status=$?

    rm -f "$tmp_pvc"

    if [[ $apply_status -ne 0 ]]; then
        log_error "Failed to apply PVC"
        return 1
    fi

    log_success "PVC ${pvc_name} applied with ownerReferences"

    return 0
}

delete_pvc() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    kubectl delete pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" 2>/dev/null
    return 0
}

get_workflows_using_pvc() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    local json
    json=$(kubectl get workflows -n "$namespace" --kubeconfig "$kubeconfig" -o json 2>/dev/null)

    if [[ -z "$json" ]] || [[ "$json" == "null" ]]; then
        return
    fi

    echo "$json" | jq -r '.items[] | select(
        .spec.volumes[]?.persistentVolumeClaim.claimName == "'"$pvc_name"'" or
        .spec.templates[]?.script.volumeMounts[]?.persistentVolumeClaim.claimName == "'"$pvc_name"'" or
        .spec.templates[]?.container.volumeMounts[]?.persistentVolumeClaim.claimName == "'"$pvc_name"'"
    ) | .metadata.name' 2>/dev/null
}

wait_for_pvc_deleted() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local timeout="${4:-120}"

    local elapsed=0
    local interval=5

    while [[ $elapsed -lt $timeout ]]; do
        if ! kubectl get pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" 2>/dev/null; then
            log_success "PVC $pvc_name deleted by GC"
            return 0
        fi
        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_error "PVC $pvc_name not deleted by GC after ${timeout}s"
    return 1
}

get_pvc_owner_workflow() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    kubectl get pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" \
        -o jsonpath='{.metadata.ownerReferences[0].name}' 2>/dev/null
}

delete_pvc_via_owner() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    local workflows
    workflows=$(get_workflows_using_pvc "$pvc_name" "$namespace" "$kubeconfig")

    if [[ -n "$workflows" ]]; then
        log_info "Found workflows using PVC '$pvc_name':"
        while IFS= read -r wf; do
            [[ -z "$wf" ]] && continue
            log_info "  - Deleting workflow: $wf"
            cleanup_workflow "$wf" "$namespace" "$kubeconfig"
        done <<< "$workflows"

        log_info "Waiting for Kubernetes GC to delete PVC '$pvc_name'..."
        wait_for_pvc_deleted "$pvc_name" "$namespace" "$kubeconfig" 120
    else
        log_info "No workflows found using PVC '$pvc_name'"
        force_delete_pvc "$pvc_name" "$namespace" "$kubeconfig"
    fi
}

force_delete_pvc() {
    local pvc_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    local pvc_status
    pvc_status=$(kubectl get pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" -o jsonpath='{.status.phase}' 2>/dev/null)
    
    if [[ "$pvc_status" == "Terminating" ]]; then
        log_info "Force removing finalizers from PVC: $pvc_name"
        kubectl patch pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" -p '{"metadata":{"finalizers":null}}' --type=merge 2>/dev/null
        sleep 2
    fi
    
    kubectl delete pvc "$pvc_name" -n "$namespace" --kubeconfig "$kubeconfig" --force --grace-period=0 2>/dev/null
    return 0
}

cleanup_workflow() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"

    log_info "Cleaning up Volcano Job: $workflow_name"
    kubectl delete job.batch.volcano.sh "$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" --ignore-not-found=true 2>/dev/null
    kubectl delete pods -l volcano.sh/job-name="$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" --ignore-not-found=true 2>/dev/null
    return 0
}

start_copy_pod() {
    local copy_pod_file="$1"
    local namespace="$2"
    local kubeconfig="$3"

    if [[ ! -f "$copy_pod_file" ]]; then
        log_error "Copy pod file not found: $copy_pod_file"
        return 1
    fi

    log_info "Starting copy pod: $(basename "$copy_pod_file")"

    local apply_output
    apply_output=$(kubectl apply -f "$copy_pod_file" --kubeconfig "$kubeconfig" -n "$namespace" 2>&1)
    local apply_status=$?

    if [[ $apply_status -ne 0 ]]; then
        log_error "Failed to start copy pod: $apply_output"
        return 1
    fi

    local pod_name
    pod_name=$(echo "$apply_output" | awk '{print $1}' | sed 's|pod/||')

    log_info "Copy pod name: $pod_name"
    echo "$pod_name"
}

wait_copy_pod() {
    local pod_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local timeout="${4:-60}"

    local elapsed=0
    local interval=5

    while [[ $elapsed -lt $timeout ]]; do
        local phase
        phase=$(kubectl get pod "$pod_name" -n "$namespace" --kubeconfig "$kubeconfig" -o jsonpath='{.status.phase}' 2>/dev/null)

        case "$phase" in
            Running)
                log_success "Copy pod is running"
                return 0
                ;;
            Succeeded)
                log_error "Copy pod unexpectedly Succeeded (should stay Running)"
                return 1
                ;;
            Failed)
                log_error "Copy pod failed"
                return 1
                ;;
            Error)
                log_error "Copy pod error"
                return 1
                ;;
            *)
                log_info "Copy pod phase: $phase, waiting..."
                ;;
        esac

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_error "Copy pod wait timeout after ${timeout}s"
    return 1
}

extract_volcano_artifacts() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local workspace="$4"
    local mount_path="$5"
    
    if [[ -z "$mount_path" ]]; then
        mount_path="/output"
    fi
    
    log_info "Extracting Volcano Job artifacts from $mount_path"
    
    local pod_name
    pod_name=$(kubectl get pods -n "$namespace" --kubeconfig "$kubeconfig" \
        -l volcano.sh/job-name="$workflow_name",volcano.sh/task-name=main-script \
        -o jsonpath='{.items[0].metadata.name}' 2>&1)
    
    if [[ -z "$pod_name" ]] || [[ "$pod_name" == *"No resources found"* ]] || [[ "$pod_name" == *"error"* ]]; then
        log_error "main-script task pod not found for job: $workflow_name"
        return 1
    fi
    
    log_info "Found main-script pod: $pod_name"
    
    # Check if ascend container is already terminated (submit already waited)
    local ascend_state
    ascend_state=$(kubectl get pod "$pod_name" -n "$namespace" --kubeconfig "$kubeconfig" \
        -o jsonpath='{.status.containerStatuses[?(@.name=="ascend")].state.terminated.exitCode}' 2>&1)
    
    if [[ -n "$ascend_state" ]] && [[ "$ascend_state" != "" ]]; then
        log_info "Ascend container already terminated (exit code: $ascend_state)"
    else
        log_info "Waiting for ascend container to complete..."
        local max_wait=300
        local interval=10
        local elapsed=0
        
        while [[ $elapsed -lt $max_wait ]]; do
            ascend_state=$(kubectl get pod "$pod_name" -n "$namespace" --kubeconfig "$kubeconfig" \
                -o jsonpath='{.status.containerStatuses[?(@.name=="ascend")].state.terminated.exitCode}' 2>&1)
            
            if [[ -n "$ascend_state" ]] && [[ "$ascend_state" != "" ]]; then
                log_success "Ascend container terminated (exit code: $ascend_state)"
                break
            fi
            
            sleep $interval
            elapsed=$((elapsed + interval))
        done
        
        if [[ $elapsed -ge $max_wait ]]; then
            log_error "Timeout waiting for ascend container to terminate"
            return 1
        fi
    fi
    
    log_info "Extracting artifacts from copy-artifact container..."
    local extract_exit=0
    kubectl exec "$pod_name" -n "$namespace" -c copy-artifact --kubeconfig "$kubeconfig" \
        -- tar czf - -C "$mount_path" . 2>&1 | tar xzf - -C "$workspace" 2>&1 || extract_exit=$?
    
    if [[ $extract_exit -ne 0 ]]; then
        log_error "Artifact extraction failed (exit code: $extract_exit)"
        return 1
    fi
    
    log_success "Artifacts extracted to $workspace"
    
    log_info "Interrupting copy-artifact container to allow pod termination..."
    kubectl exec "$pod_name" -n "$namespace" -c copy-artifact --kubeconfig "$kubeconfig" \
        -- sh -c "kill -TERM 1 || true" 2>&1 || true
    
    log_success "Copy-artifact container interrupted"
    return 0
}

run_eval() {
    local eval_script="$1"
    local workflow_name="$2"
    local namespace="$3"
    local kubeconfig="$4"
    local workspace="$5"
    local cp_artifacts_temp_folder="$6"
    shift 6
    local extra=("$@")

    if [[ ! -f "$eval_script" ]]; then
        log_error "Eval script not found: $eval_script"
        return 1
    fi

    log_eval "Running eval: $eval_script"
    
    local is_volcano_job=false
    if [[ -n "$cp_artifacts_temp_folder" ]]; then
        local volcano_job_check
        volcano_job_check=$(kubectl get job.batch.volcano.sh "$workflow_name" -n "$namespace" --kubeconfig "$kubeconfig" \
            -o jsonpath='{.kind}' 2>/dev/null)
        
        if [[ "$volcano_job_check" == "Job" ]]; then
            is_volcano_job=true
            log_info "Detected Volcano Job with artifacts (submit already extracted them)"
        fi
    fi
    
    if [[ "$is_volcano_job" == "true" ]]; then
        log_info "Skipping artifact extraction (already handled by submit phase)"
    fi

    bash "$eval_script" "$workflow_name" "$namespace" "$kubeconfig" "$workspace" "$cp_artifacts_temp_folder" "${extra[@]}"
    return $?
}
