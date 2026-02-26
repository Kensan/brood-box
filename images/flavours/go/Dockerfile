# SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
# SPDX-License-Identifier: Apache-2.0

# Go toolchain flavour image for Brood Box.
# Adds Go compiler and go-task on top of the base image.
FROM ghcr.io/stacklok/brood-box/base:latest

RUN apk add --no-cache \
    go \
    make

# Install go-task from GitHub releases (not packaged in Wolfi).
ARG TASK_VERSION=v3.48.0
RUN ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && \
    curl -fsSL "https://github.com/go-task/task/releases/download/${TASK_VERSION}/task_linux_${ARCH}.tar.gz" | \
    tar -xz -C /usr/local/bin task
