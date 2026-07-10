FROM golang:1.24-alpine AS go-builder

WORKDIR /app/go
COPY go/ .
# RUN CGO_ENABLED=0 go build -o convert_to_yaml ./cmd/convertor
# RUN CGO_ENABLED=0 go build -o envrender ./cmd/envrender
RUN CGO_ENABLED=0 go build -o convert_to_yaml ./cmd/converter
RUN CGO_ENABLED=0 go build -o kubeconfig ./cmd/oldkubeconfig
RUN CGO_ENABLED=0 go build -o submit ./cmd/submit


FROM alpine:3.18

USER root

WORKDIR /workspace


# 安装所有必要工具 (with retry for transient network errors)
RUN for i in 1 2 3; do apk update && apk add --no-cache \
    git \
    bash \
    curl \
    gzip \
    jq \
    && rm -rf /var/cache/apk/* && break || sleep 5; done

# 安装 kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && rm kubectl


COPY --from=go-builder /app/go/convert_to_yaml /workspace/workflowtool/
COPY --from=go-builder /app/go/kubeconfig /workspace/workflowtool/
COPY --from=go-builder /app/go/submit /workspace/workflowtool/


COPY  src/* ./workflowtool/
COPY go/cmd/converter/case/workflow_templatev2.yaml ./workflowtool
COPY go/cmd/converter/case/copy-pod-template.yaml ./workflowtool

# RUN sed -i 's/\r$//' ./workflowtool/argo_cli_installer.sh && \
#     chmod +x ./workflowtool/argo_cli_installer.sh && \
#     ./workflowtool/argo_cli_installer.sh && \
#     rm ./workflowtool/argo_cli_installer.sh


RUN chmod +x ./workflowtool/entrypoint.sh ./workflowtool/select_kubeconfig.sh ./workflowtool/unit.sh ./workflowtool/kubeconfig ./workflowtool/submit

ENV KUBECONFIG=/workspace/workflowtool/k8s-cluster-kubeconfig.yaml


# 1. Create the trap script
# We use pkill -P 1 to explicitly target children of PID 1
RUN echo 'trap "echo [Signal-Forward] Forwarding signal...; pkill -P 1; wait" TERM' > /usr/local/bin/signal_forward.sh

# 2. Make it executable (good practice)
RUN chmod +x /usr/local/bin/signal_forward.sh

# 3. Set the environment variable on a NEW line
ENV BASH_ENV=/usr/local/bin/signal_forward.sh

RUN echo "source /usr/local/bin/signal_forward.sh" >> /root/.bashrc

CMD ["/bin/bash"]
