FROM python:3.11-slim

WORKDIR /scripts

COPY force-patch-stuck-rb/patch.py ./patch.py
COPY vcjob-ttl-cleaner/clean.py ./clean.py

ENV KUBECONFIG=/etc/karmada/karmada.config \
    THRESHOLD=900 \
    DEFAULT_CLUSTER=001 \
    TTL_MINUTES=2 \
    DRY_RUN=false

CMD ["python3", "/scripts/patch.py"]
