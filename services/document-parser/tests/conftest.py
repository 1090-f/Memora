"""pytest 配置：模型用例默认跳过，设置 DOCLING_MODELS_READY=1 启用。"""

from __future__ import annotations

import os

import pytest


def pytest_collection_modifyitems(config, items):
    if os.environ.get("DOCLING_MODELS_READY") == "1":
        return
    skip_models = pytest.mark.skip(
        reason="需要真实模型（设置 DOCLING_MODELS_READY=1 启用）"
    )
    for item in items:
        if "models" in item.keywords:
            item.add_marker(skip_models)
