# SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
# SPDX-License-Identifier: Apache-2.0

# Go toolchain flavour image for apiary.
# Adds Go compiler and go-task on top of the base image.
FROM ghcr.io/stacklok/apiary/base:latest

RUN apk add --no-cache \
    go \
    go-task \
    make
