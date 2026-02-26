# SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
# SPDX-License-Identifier: Apache-2.0

# Python toolchain flavour image for Brood Box.
# Adds Python 3, pip, and virtualenv on top of the base image.
FROM ghcr.io/stacklok/brood-box/base:latest

RUN apk add --no-cache \
    python3 \
    py3-pip \
    py3-virtualenv \
    make
