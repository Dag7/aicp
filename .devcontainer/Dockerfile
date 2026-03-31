# === Base Stage ===
ARG VARIANT="jammy"

FROM mcr.microsoft.com/vscode/devcontainers/base:0-${VARIANT} AS base
ARG OPERATOR_SDK_VERSION=v1.42.2
ARG OPERATOR_SDK_ARCH=arm64
ARG OPERATOR_SDK_OS=linux

# Set environment variables
ENV SHELL=/usr/bin/zsh \
    OPERATOR_SDK_VERSION=${OPERATOR_SDK_VERSION} \
    OPERATOR_SDK_ARCH=${OPERATOR_SDK_ARCH} \
    OPERATOR_SDK_OS=${OPERATOR_SDK_OS} \
    PATH="/usr/local/kubectl:${PATH}"

# Install common dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    curl \
    gnupg \
    ca-certificates \
    wget \
    unzip \
    software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# === Kubectl Installation ===
RUN mkdir -p /usr/local/kubectl && \
    curl -sSL -o /usr/local/kubectl/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${OPERATOR_SDK_ARCH}/kubectl" && \
    curl -sSL -o /usr/local/kubectl/kubectl.sha256 "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${OPERATOR_SDK_ARCH}/kubectl.sha256" && \
    cd /usr/local/kubectl && \
    echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check --status - && \
    chmod +x kubectl && \
    ln -s /usr/local/kubectl/kubectl /usr/local/bin/kubectl && \
    rm -f kubectl.sha256

# === Operator SDK Installation ===
ENV OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}

RUN curl -LO ${OPERATOR_SDK_DL_URL}/operator-sdk_${OPERATOR_SDK_OS}_${OPERATOR_SDK_ARCH}
RUN gpg --keyserver keyserver.ubuntu.com --recv-keys 052996E2A20B5C7E || \
    gpg --keyserver keys.openpgp.org --recv-keys 052996E2A20B5C7E || \
    gpg --keyserver pgp.mit.edu --recv-keys 052996E2A20B5C7E
RUN curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt
RUN curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc
RUN gpg -u "Operator SDK (release) <cncf-operator-sdk@cncf.io>" --verify checksums.txt.asc
RUN grep operator-sdk_${OPERATOR_SDK_OS}_${OPERATOR_SDK_ARCH} checksums.txt | sha256sum -c -
RUN chmod +x operator-sdk_${OPERATOR_SDK_OS}_${OPERATOR_SDK_ARCH} && sudo mv operator-sdk_${OPERATOR_SDK_OS}_${OPERATOR_SDK_ARCH} /usr/local/bin/operator-sdk

# === Helm Installation ===
RUN curl -s https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# === k3d Installation ===
RUN wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# === Tilt Installation ===
RUN curl -sSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash

# === PostgreSQL Client Installation ===
RUN apt-get update && \
    apt-get install -y --no-install-recommends postgresql-client && \
    rm -rf /var/lib/apt/lists/*

# === golangci-lint Installation ===
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin

# Create .config directory with proper ownership before volume mounts
RUN mkdir -p /home/vscode/.config && chown vscode:vscode /home/vscode/.config

# Create completions dir
ENV COMPLETION_FOLDER="/home/vscode/.oh-my-zsh/completions"
RUN mkdir -p ${COMPLETION_FOLDER}
RUN fpath="$COMPLETION_FOLDER $fpath"

RUN echo "autoload -Uz compinit && compinit" >> /home/vscode/.zshrc

# Add kubectl CLI completion to Zsh
RUN kubectl completion zsh > ${COMPLETION_FOLDER}/_kubectl

# Add k3d CLI completion to Zsh
RUN k3d completion zsh > ${COMPLETION_FOLDER}/_k3d

# Add Helm CLI completion
RUN helm completion zsh > ${COMPLETION_FOLDER}/_helm

# Add Tilt CLI completion for Zsh
RUN tilt completion zsh > ${COMPLETION_FOLDER}/_tilt

# Add operator-sdk completion
RUN operator-sdk completion zsh > ${COMPLETION_FOLDER}/_operator-sdk

# Setup plugins
RUN sed -i "/^plugins=/c\plugins=(docker git kubectl history)" /home/vscode/.zshrc

# Add timestamp and k8s context to terminal prompt
COPY scripts/zsh-prompt.sh /tmp/zsh-prompt.sh
RUN cat /tmp/zsh-prompt.sh >> /home/vscode/.zshrc && rm /tmp/zsh-prompt.sh
RUN echo 'ORIG_PROMPT="${PROMPT}"' >> /home/vscode/.zshrc && \
    echo 'PROMPT='\''%{$fg[blue]%}%D{%f/%m} %t $(k8s_prompt)'\''${ORIG_PROMPT}' >> /home/vscode/.zshrc

# === Aliases ===
RUN echo "alias k=kubectl" >> /home/vscode/.zshrc
RUN echo "alias kn='kubectl config set-context --current --namespace'" >> /home/vscode/.zshrc
