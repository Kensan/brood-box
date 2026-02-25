# SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
# SPDX-License-Identifier: Apache-2.0

# Python toolchain flavour image for apiary.
# Adds Python 3, pip, and virtualenv on top of the base image.
FROM ghcr.io/stacklok/apiary/base:latest

RUN apk add --no-cache \
    python3 \
    py3-pip \
    py3-virtualenv \
    make
