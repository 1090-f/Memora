"""docling-parse 非 ASCII 路径兼容层。

docling-parse 的 C++ 层（docling_resources::set_resources_path）用
PyUnicode_AsUTF8 拿到模块路径字节后，在 Windows 上按系统 ANSI 代码页
（中文系统为 GBK/936）构造 std::filesystem::path 再检查 pdf_resources。
当 Python 环境位于中文用户名路径（如 C:\\Users\\冬冬\\...）时，UTF-8 字节
被 GBK 错误解码，资源目录"不存在"，导致 PDF 与图片解析直接失败。

对策：把 docling_parse 包整体复制到 ASCII 目录并从该路径预导入，
使 C++ 层拿到的模块路径可被 ANSI 代码页无损解码。幂等；仅当模块
已位于非 ASCII 路径时生效。
"""

from __future__ import annotations

import importlib.util
import logging
import os
import shutil
import sys
import tempfile
from pathlib import Path

_log = logging.getLogger("document-parser")

# 显式指定缓存目录的覆盖开关（测试与 CI 可用）。
_CACHE_ENV = "DOCLING_PARSE_ASCII_CACHE_DIR"


def _is_ascii_path(path: Path) -> bool:
    try:
        path.as_posix().encode("ascii")
        return True
    except UnicodeEncodeError:
        return False


def _cache_root() -> Path | None:
    """返回可写的 ASCII 缓存根目录；找不到时返回 None。"""
    if value := os.environ.get(_CACHE_ENV):
        root = Path(value)
        try:
            root.mkdir(parents=True, exist_ok=True)
            return root
        except OSError:
            _log.warning("%s 不可写，回退默认目录", _CACHE_ENV)
    # 系统级 ASCII 目录优先（PUBLIC/PROGRAMDATA 对普通用户可写）。
    for candidate in ("PUBLIC", "PROGRAMDATA"):
        base = os.environ.get(candidate)
        if base and _is_ascii_path(Path(base)):
            root = Path(base) / "memora-cache" / "docling-parse"
            try:
                root.mkdir(parents=True, exist_ok=True)
                return root
            except OSError:
                continue
    try:
        tmp = Path(tempfile.gettempdir())
        if _is_ascii_path(tmp):
            root = tmp / "memora-cache" / "docling-parse"
            root.mkdir(parents=True, exist_ok=True)
            return root
    except OSError:
        pass
    return None


def ensure_ascii_docling_parse() -> None:
    """确保 docling_parse 从 ASCII 路径加载（幂等，进程级只需执行一次）。

    找不到 ASCII 缓存目录时降级为不处理：PDF/图片解析可能失败，
    但 DOCX/XLSX/PPTX（纯 Python 后端）不受影响。
    """
    spec = importlib.util.find_spec("docling_parse")
    if spec is None or spec.origin is None:
        return
    origin = Path(spec.origin)
    if _is_ascii_path(origin):
        return
    if "docling_parse" in sys.modules:
        return
    root = _cache_root()
    if root is None:
        _log.warning("docling_parse 位于非 ASCII 路径且无可用 ASCII 缓存目录，PDF/图片解析可能失败")
        return
    target = root / "docling_parse"
    try:
        # 每次同步一份最新副本（约 12MB），保证缓存与安装版本一致。
        if target.exists():
            shutil.rmtree(target)
        shutil.copytree(origin.parent, target, ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
    except OSError as exc:
        _log.warning("复制 docling_parse 到 %s 失败: %s", target, exc)
        return
    if str(root) not in sys.path:
        sys.path.insert(0, str(root))
    try:
        import docling_parse  # noqa: F401
    except Exception as exc:
        _log.warning("从 ASCII 路径导入 docling_parse 失败: %s", exc)
        if str(root) in sys.path:
            sys.path.remove(str(root))
