#!/usr/bin/env python3

from flask import Flask, request, jsonify
import threading
import logging
import os
import sys
import queue
import time
import re
import copy
import signal
import yaml

from kubernetes import client, config
from kubernetes.dynamic import DynamicClient

app = Flask(__name__)
logger = logging.getLogger(__name__)

NAMESPACE = os.environ.get('NAMESPACE', 'kube-system')
CLUSTER_SELECTOR = os.environ.get('CLUSTER_SELECTOR', '')
CONCURRENCY = int(os.environ.get('CONCURRENCY', '3'))
INCLUDE_NODES = os.environ.get('INCLUDE_NODES', '')
EXCLUDE_NODES = os.environ.get('EXCLUDE_NODES', '')
TIMEOUT = int(os.environ.get('TIMEOUT', '0'))
KUBECONFIG_PATH = os.environ.get('KUBECONFIG_PATH', '/etc/karmada/kubeconfig')
LABEL_PREFIX = 'image-preheat'
REGISTRY = os.environ.get('REGISTRY', '')
IMAGE_PULL_SECRET = os.environ.get('IMAGE_PULL_SECRET', '')

CLUSTERS = []
kubeconfig_dict = None
cluster_clients = {}
client_lock = threading.Lock()

task_queue = queue.Queue()
pending_tasks = set()
pending_lock = threading.Lock()

dyn_client = None

FAILED_WAITING_REASONS = (
    'ImagePullBackOff', 'ErrImagePull', 'InvalidImageName',
    'CrashLoopBackOff', 'CreateContainerConfigError', 'CreateContainerError',
)
FAILED_TERMINATED_REASONS = ('OOMKilled', 'Error', 'ContainerCannotRun')


def init_k8s_client():
    """从文件加载kubeconfig到内存，然后删除文件"""
    global dyn_client, kubeconfig_dict
    with open(KUBECONFIG_PATH, 'r') as f:
        kubeconfig_dict = yaml.safe_load(f)
    os.remove(KUBECONFIG_PATH)
    logger.info("Kubeconfig loaded to memory, file removed")
    api_client = config.new_client_from_config_dict(kubeconfig_dict)
    dyn_client = DynamicClient(api_client)


def get_karmada_clusters():
    """获取Karmada管理的成员集群"""
    clusters = []
    try:
        cluster_api = dyn_client.resources.get(api_version='cluster.karmada.io/v1alpha1', kind='Cluster')
        result = cluster_api.get()
        for item in result.items:
            name = item.metadata.name
            conditions = item.status.conditions if item.status and item.status.conditions else []
            ready = any(c.type == 'Ready' and c.status == 'True' for c in conditions)
            if not ready:
                continue
            if CLUSTER_SELECTOR:
                labels = item.metadata.labels or {}
                matched = False
                for selector in CLUSTER_SELECTOR.split(','):
                    k, v = selector.split('=')
                    if labels.get(k) == v:
                        matched = True
                        break
                if not matched:
                    continue
            clusters.append(name)
            logger.info(f"Found cluster: {name}")
        logger.info(f"Discovered {len(clusters)} ready clusters")
        return clusters
    except Exception as e:
        logger.error(f"Error getting clusters: {e}")
        return []


def get_cluster_client(cluster_name):
    """经 Karmada cluster proxy 构造指向成员集群的 DynamicClient"""
    if cluster_name in cluster_clients:
        return cluster_clients[cluster_name]
    with client_lock:
        if cluster_name in cluster_clients:
            return cluster_clients[cluster_name]
        kc = copy.deepcopy(kubeconfig_dict)
        ctx = kc.get('current-context')
        cluster_in_cfg = None
        for c in kc.get('contexts', []):
            if c.get('name') == ctx:
                cluster_in_cfg = c['context']['cluster']
                break
        for cl in kc.get('clusters', []):
            if cl.get('name') == cluster_in_cfg:
                server = cl['cluster']['server']
                cl['cluster']['server'] = server.rstrip('/') + \
                    f'/apis/cluster.karmada.io/v1alpha1/clusters/{cluster_name}/proxy'
                break
        api_client = config.new_client_from_config_dict(kc)
        cclient = DynamicClient(api_client)
        cluster_clients[cluster_name] = cclient
        return cclient


def detect_image_arch(image):
    """检测镜像架构标识"""
    lower = image.lower()
    if re.search(r'amd64|x86_64|x64', lower):
        return 'amd64'
    elif re.search(r'arm64|aarch64|armv[789]', lower):
        return 'arm64'
    elif re.search(r'x86', lower) and not re.search(r'x86_64', lower):
        return 'amd64'
    elif re.search(r'arm', lower) and not re.search(r'arm64|aarch64', lower):
        return 'arm64'
    else:
        return 'multi'


def sanitize_pod_name(s):
    s = s.lower()
    s = re.sub(r'[^a-z0-9.-]', '-', s)
    s = s.strip('-')
    return s[:63]


def sanitize_container_name(s):
    s = s.lower()
    s = re.sub(r'[^a-z0-9-]', '-', s)
    s = s.strip('-')
    return s[:63]


def _as_dict(obj):
    if obj is None:
        return {}
    if hasattr(obj, 'to_dict'):
        d = obj.to_dict()
        return d if d else {}
    if isinstance(obj, dict):
        return obj
    return {}


def list_cluster_nodes(cluster_name):
    """枚举成员集群节点及其架构，对标原版 get_node_arch"""
    c = get_cluster_client(cluster_name)
    node_api = c.resources.get(api_version='v1', kind='Node')
    result = node_api.get()
    nodes = []
    for n in result.items:
        d = _as_dict(n)
        name = d.get('metadata', {}).get('name')
        arch = d.get('status', {}).get('nodeInfo', {}).get('architecture') or 'Unknown'
        nodes.append((name, arch))
    return nodes


def collect_target_nodes():
    """枚举所有成员集群节点，按 INCLUDE/EXCLUDE 过滤"""
    include_set = set(n.strip() for n in INCLUDE_NODES.split(',') if n.strip()) if INCLUDE_NODES else set()
    exclude_set = set(n.strip() for n in EXCLUDE_NODES.split(',') if n.strip()) if EXCLUDE_NODES else set()
    all_nodes = []
    for cluster in CLUSTERS:
        try:
            for node, arch in list_cluster_nodes(cluster):
                if include_set and node not in include_set:
                    continue
                if exclude_set and node in exclude_set:
                    continue
                all_nodes.append((cluster, node, arch))
        except Exception as e:
            logger.warning(f"list nodes in {cluster}: {e}")
    return all_nodes


def create_pod(cluster_name, node, image):
    """在成员集群指定节点创建 Pod，对标原版 create_pod"""
    ts = int(time.time())
    pod_name = f"{LABEL_PREFIX}-{sanitize_pod_name(node)}-{sanitize_container_name(image)}-{ts}"
    pod_name = pod_name[:63].rstrip('-.')
    body = {
        'apiVersion': 'v1',
        'kind': 'Pod',
        'metadata': {
            'name': pod_name,
            'namespace': NAMESPACE,
            'labels': {LABEL_PREFIX: 'preheat', 'preheat-node': node},
        },
        'spec': {
            'nodeName': node,
            'restartPolicy': 'Never',
            'tolerations': [{'operator': 'Exists'}],
            'containers': [{
                'name': 'pull',
                'image': image,
                'imagePullPolicy': 'IfNotPresent',
                'command': ['true'],
                'resources': {
                    'requests': {'cpu': '50m', 'memory': '32Mi'},
                    'limits': {'cpu': '200m', 'memory': '128Mi'},
                },
            }],
        },
    }
    c = get_cluster_client(cluster_name)
    pod_api = c.resources.get(api_version='v1', kind='Pod')
    if IMAGE_PULL_SECRET:
        body['spec']['imagePullSecrets'] = [{'name': IMAGE_PULL_SECRET}]
    logger.info(f"Creating pod on {node} in {cluster_name} for {image}")
    pod_api.create(body=body, namespace=NAMESPACE)
    logger.info(f"Created pod {pod_name} on {node} in {cluster_name}")
    return pod_name


def is_pod_failed(cluster_name, pod_name):
    """判定 Pod 状态：succeeded/failed/pending/stuck + reason，对标原版 is_pod_failed"""
    c = get_cluster_client(cluster_name)
    pod_api = c.resources.get(api_version='v1', kind='Pod')
    try:
        pod = pod_api.get(name=pod_name, namespace=NAMESPACE)
    except Exception as e:
        return 'stuck', f'get failed: {e} | body={getattr(e, "body", "") or ""}'
    d = _as_dict(pod)
    status = d.get('status') or {}
    phase = status.get('phase') or ''
    if phase == 'Succeeded':
        return 'succeeded', ''
    if phase == 'Failed':
        return 'failed', 'phase=Failed'
    if phase in ('Unknown', ''):
        return 'stuck', f'phase={phase!r}'
    for cs in status.get('containerStatuses') or []:
        state = cs.get('state') or {}
        w = state.get('waiting')
        if w and w.get('reason') in FAILED_WAITING_REASONS:
            return 'failed', f'waiting: {w.get("reason")} | {w.get("message", "")}'
        t = state.get('terminated')
        if t and t.get('reason') in FAILED_TERMINATED_REASONS:
            return 'failed', f'terminated: {t.get("reason")} | {t.get("message", "")}'
    return 'pending', ''


def wait_for_pods(pods):
    """轮询一批 Pod 直到全部终结，对标原版 wait_for_pods"""
    start = time.time()
    total = len(pods)
    while True:
        success = pending = 0
        fail_reasons = []
        stuck_reasons = []
        for cluster, pn in pods:
            state, reason = is_pod_failed(cluster, pn)
            if state == 'succeeded':
                success += 1
            elif state == 'pending':
                pending += 1
            elif state == 'stuck':
                stuck_reasons.append((cluster, pn, reason))
            else:
                fail_reasons.append((cluster, pn, reason))

        stuck = len(stuck_reasons)

        if pending == 0 and stuck == 0:
            dur = int(time.time() - start)
            for cluster, pn, reason in fail_reasons:
                logger.warning(f"  {cluster}/{pn}: {reason}")
            logger.info(f"  Batch done: {success} ok, {total - success} failed (took {dur}s)")
            return success, total - success

        if stuck > 0 and (time.time() - start) >= 300:
            dur = int(time.time() - start)
            for cluster, pn, reason in stuck_reasons:
                logger.warning(f"  {cluster}/{pn}: stuck - {reason}")
            for cluster, pn, reason in fail_reasons:
                logger.warning(f"  {cluster}/{pn}: {reason}")
            logger.warning(f"  {stuck} pods stuck (Unknown) for 300s, treating as failed")
            logger.info(f"  Batch done: {success} ok, {total - success} failed (took {dur}s)")
            return success, total - success

        if TIMEOUT > 0 and (time.time() - start) >= TIMEOUT:
            logger.warning(f"  Batch timeout ({TIMEOUT}s)")
            for cluster, pn, reason in fail_reasons:
                logger.warning(f"  {cluster}/{pn}: {reason}")
            for cluster, pn, reason in stuck_reasons:
                logger.warning(f"  {cluster}/{pn}: stuck - {reason}")
            return success, total - success

        time.sleep(5)


def delete_pods(pods):
    """并发删除一批 Pod，对标原版 delete_pods"""
    threads = []
    for cluster, pn in pods:
        def _del(cluster=cluster, pn=pn):
            try:
                c = get_cluster_client(cluster)
                pod_api = c.resources.get(api_version='v1', kind='Pod')
                pod_api.delete(name=pn, namespace=NAMESPACE, grace_period_seconds=5)
            except Exception as e:
                logger.warning(f"delete {pn} in {cluster}: {e}")
        t = threading.Thread(target=_del)
        threads.append(t)
        t.start()
    for t in threads:
        t.join()


def ensure_namespaces():
    """确保每个成员集群中 NAMESPACE 存在，对标原版 check_prerequisites"""
    for cluster in CLUSTERS:
        try:
            c = get_cluster_client(cluster)
            ns_api = c.resources.get(api_version='v1', kind='Namespace')
            try:
                ns_api.get(name=NAMESPACE)
            except Exception:
                ns_api.create(body={
                    'apiVersion': 'v1',
                    'kind': 'Namespace',
                    'metadata': {'name': NAMESPACE},
                })
                logger.info(f"Created namespace {NAMESPACE} in {cluster}")
        except Exception as e:
            logger.warning(f"ensure namespace in {cluster}: {e}")


def cleanup_leftover():
    """清理所有成员集群中残留的 preheat Pod，对标原版 trap cleanup / main 开头清残留"""
    for cluster in CLUSTERS:
        try:
            c = get_cluster_client(cluster)
            pod_api = c.resources.get(api_version='v1', kind='Pod')
            result = pod_api.get(namespace=NAMESPACE, label_selector=f'{LABEL_PREFIX}=preheat')
            for p in result.items:
                d = _as_dict(p)
                name = d.get('metadata', {}).get('name')
                if not name:
                    continue
                try:
                    pod_api.delete(name=name, namespace=NAMESPACE, grace_period_seconds=0)
                except Exception:
                    pass
            logger.info(f"Cleaned leftover preheat pods in {cluster}")
        except Exception as e:
            logger.warning(f"cleanup in {cluster}: {e}")


def preheat_image(image, all_nodes):
    """预热单个镜像：按架构筛选节点，分批并发拉取，对标原版 main 中 per-image 流程"""
    img_arch = detect_image_arch(image)
    target = [(c, n) for (c, n, a) in all_nodes if img_arch == 'multi' or img_arch == a]
    if not target:
        logger.info(f"Image: {image} ({img_arch}) -> no compatible nodes, skipped")
        return {'image': image, 'arch': img_arch, 'status': 'skipped', 'ok': 0, 'fail': 0}

    logger.info(f"Image: {image} ({img_arch}) -> {len(target)} nodes")
    total_ok = 0
    total_fail = 0
    i = 0
    while i < len(target):
        batch = target[i:i + CONCURRENCY]
        pod_list = []
        for cluster, node in batch:
            try:
                pn = create_pod(cluster, node, image)
                pod_list.append((cluster, pn))
            except Exception as e:
                body = getattr(e, 'body', '') or ''
                logger.error(f"create pod on {node} ({cluster}): {e} | body={body}")
        if pod_list:
            ok, fail = wait_for_pods(pod_list)
            total_ok += ok
            total_fail += fail
            delete_pods(pod_list)
        i += CONCURRENCY

    logger.info(f"Image {image}: {total_ok} ok, {total_fail} failed")
    return {'image': image, 'arch': img_arch, 'status': 'done', 'ok': total_ok, 'fail': total_fail}


def worker():
    """后台处理任务，对标原版 main 流程"""
    while True:
        images = None
        try:
            images = task_queue.get()
            if images is None:
                break

            logger.info(f"Processing {len(images)} images on {len(CLUSTERS)} clusters")
            logger.info(f"Images ({len(images)}):")
            for img in images:
                logger.info(f"  - {img} ({detect_image_arch(img)})")

            all_nodes = collect_target_nodes()
            logger.info(f"Target nodes ({len(all_nodes)}):")
            for cluster, node, arch in all_nodes:
                logger.info(f"  - {cluster}/{node} ({arch})")
            if not all_nodes:
                logger.error("No target nodes found across clusters")

            results = []
            for img in images:
                results.append(preheat_image(img, all_nodes))

            ok = sum(r['ok'] for r in results)
            fail = sum(r['fail'] for r in results)
            skip = sum(1 for r in results if r['status'] == 'skipped')
            logger.info(f"Batch complete: {ok} ok, {fail} failed, {skip} skipped")

            task_queue.task_done()

        except Exception as e:
            logger.error(f"Worker error: {e}")
            if images is not None:
                task_queue.task_done()
            continue
        finally:
            with pending_lock:
                for img in (images or []):
                    pending_tasks.discard(img)


@app.route('/webhook', methods=['POST'])
def harbor_webhook():
    try:
        data = request.json
        if not data:
            return jsonify({'status': 'error', 'message': 'empty payload'}), 400

        event_type = data.get('type', '')
        if event_type not in ('PUSH_ARTIFACT', 'PULL_ARTIFACT'):
            return jsonify({'status': 'ignored', 'message': f'event {event_type} not handled'}), 200

        logger.info(f"Received {event_type} event")

        event_data = data.get('event_data', {})
        if not event_data:
            return jsonify({'status': 'error', 'message': 'missing event_data'}), 400

        resources = event_data.get('resources', [])
        repo_name = event_data.get('repository', {}).get('name', '')
        repo_namespace = event_data.get('repository', {}).get('namespace', '')

        images = []
        full_repo = f"{repo_namespace}/{repo_name}" if repo_namespace else repo_name
        if REGISTRY:
            full_repo = f"{REGISTRY}/{full_repo}"
        for resource in resources:
            tag = resource.get('tag', '')
            digest = resource.get('digest', '')
            if tag and ':' not in tag:
                images.append(f"{full_repo}:{tag}")
            elif digest:
                images.append(f"{full_repo}@{digest}")
            elif tag:
                images.append(f"{full_repo}@{tag}")

        if not images:
            return jsonify({'status': 'error', 'message': 'no images'}), 400

        if not CLUSTERS:
            return jsonify({'status': 'error', 'message': 'no clusters'}), 500

        new_images = []
        with pending_lock:
            for img in images:
                if img not in pending_tasks:
                    pending_tasks.add(img)
                    new_images.append(img)

        if not new_images:
            return jsonify({'status': 'skipped', 'message': 'already pending'}), 200

        arch_info = {img: detect_image_arch(img) for img in new_images}
        task_queue.put(new_images)

        return jsonify({
            'status': 'accepted',
            'message': f'queued for {len(CLUSTERS)} clusters',
            'images': new_images,
            'archs': arch_info,
            'clusters': CLUSTERS,
        }), 200

    except Exception as e:
        logger.error(f"Webhook error: {e}")
        return jsonify({'status': 'error', 'message': str(e)}), 500


@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'healthy', 'clusters': len(CLUSTERS), 'cluster_names': CLUSTERS}), 200


@app.route('/clusters', methods=['GET'])
def clusters():
    return jsonify({'clusters': CLUSTERS}), 200


def sigterm_handler(signum, frame):
    logger.info("SIGTERM received, cleaning up leftover preheat pods...")
    cleanup_leftover()
    sys.exit(0)


if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')

    if INCLUDE_NODES and EXCLUDE_NODES:
        logger.error("Cannot use both INCLUDE_NODES and EXCLUDE_NODES")
        sys.exit(1)
    if CONCURRENCY < 1:
        logger.error("CONCURRENCY must be a positive integer")
        sys.exit(1)

    init_k8s_client()

    CLUSTERS = get_karmada_clusters()
    if not CLUSTERS:
        logger.error("No Karmada clusters found")
        sys.exit(1)

    logger.info(f"Ensuring namespace {NAMESPACE} exists in all clusters...")
    ensure_namespaces()

    logger.info("Cleaning up leftover preheat pods...")
    cleanup_leftover()

    signal.signal(signal.SIGTERM, sigterm_handler)

    port = int(os.environ.get('PORT', 8080))
    logger.info(f"Webhook started on port {port}, managing {len(CLUSTERS)} clusters, "
                f"concurrency={CONCURRENCY}, timeout={TIMEOUT}s")

    worker_thread = threading.Thread(target=worker, daemon=True)
    worker_thread.start()

    app.run(host='0.0.0.0', port=port, threaded=True)
