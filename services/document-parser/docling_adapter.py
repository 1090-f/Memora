"""DoclingAdapter：Docling → Memora ParsedDocument 的结构适配。

职责边界（见 docs/2026-08-08-docling-document-parsing-execution-plan.md）：
  - 只负责把 PDF/DOCX 转换为内存中的 DoclingDocument，再转成 ParsedDocument；
  - 保留阅读顺序、标题路径、caption、page、bbox、self-ref、表格行列与合并关系、
    图片原始数据（或安全 omitted 标记）；
  - 不包含任何 Chunk/RAG 分块策略与 Embedding tokenizer 概念；
  - 不访问 MinIO/PostgreSQL/Redis，不主动发起外部网络请求。
"""

from __future__ import annotations

import base64
import hashlib
import io
import threading
from typing import Any

from docling.backend.docling_parse_backend import DoclingParseDocumentBackend
from docling.datamodel.base_models import ConversionStatus, DocumentStream, InputFormat
from docling.datamodel.pipeline_options import (
    LayoutObjectDetectionOptions,
    PdfPipelineOptions,
    RapidOcrOptions,
)
from docling.document_converter import DocumentConverter, FormatOption
from docling.pipeline.standard_pdf_pipeline import StandardPdfPipeline
from docling_core.types.doc.document import DoclingDocument
from docling_core.types.doc.items.node import DocItem, FloatingItem
from docling_core.types.doc.items.picture.picture import PictureItem
from docling_core.types.doc.items.table.table import TableItem
from docling_core.types.doc.labels import DocItemLabel
from PIL import Image as PILImage

import schemas

# 语言映射：协议使用通用 ISO 639-1 风格代码，RapidOCR 使用 PP-OCR 代码。
_OCR_LANG_MAP = {
    "zh": "ch",
    "zh-cn": "ch",
    "zh-hans": "ch",
    "en": "en",
    "chinese": "ch",
    "english": "en",
    "de": "de",
    "fr": "fr",
    "ja": "japan",
    "japanese": "japan",
}


class AssetLimitError(Exception):
    """图片资产超过单图/数量/总量限制。"""


class DocumentParserError(Exception):
    """文档解析失败。code 为稳定错误码，message 为人类可读原因。"""

    def __init__(self, code: str, message: str) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message


class ParseFailureCode:
    INVALID_FORMAT = "invalid_format"
    PARSE_FAILED = "parse_failed"
    LIMIT_EXCEEDED = "limit_exceeded"
    ASSET_TOO_LARGE = "asset_too_large"


def _ocr_lang_codes(langs: list[str]) -> list[str]:
    """将协议语言代码映射为 RapidOCR PP-OCR 代码；未知代码抛出错误。"""
    codes: list[str] = []
    for lang in langs:
        code = _OCR_LANG_MAP.get(lang.strip().lower())
        if code is None:
            raise DocumentParserError(
                ParseFailureCode.LIMIT_EXCEEDED,
                f"不支持的 OCR 语言: {lang}",
            )
        if code not in codes:
            codes.append(code)
    if not codes:
        raise DocumentParserError(ParseFailureCode.LIMIT_EXCEEDED, "ocr_languages 为空")
    return codes


def _detect_format(file_name: str, content: bytes) -> InputFormat | None:
    """按扩展名 + 文件签名/容器格式双重判断，禁止伪造格式。

    仅返回 PDF / DOCX，其余一律视为不支持。
    """
    lower = file_name.lower()
    if lower.endswith(".pdf"):
        if content.startswith(b"%PDF-"):
            return InputFormat.PDF
        return None
    if lower.endswith(".docx"):
        if _is_docx_container(content):
            return InputFormat.DOCX
        return None
    return None


def _is_docx_container(content: bytes) -> bool:
    """DOCX 是 ZIP 容器：必须存在 [Content_Types].xml 与 word/document.xml。"""
    import zipfile

    try:
        with zipfile.ZipFile(io.BytesIO(content)) as zf:
            names = set(zf.namelist())
            return "[Content_Types].xml" in names and "word/document.xml" in names
    except (zipfile.BadZipFile, OSError):
        return False


class DoclingAdapter:
    """持有常驻 DocumentConverter 并完成 ParsedDocument 转换。"""

    def __init__(
        self,
        *,
        max_asset_bytes: int = 32 * 1024 * 1024,
        max_asset_count: int = 100,
        max_total_asset_bytes: int = 64 * 1024 * 1024,
        max_pages: int = 500,
    ) -> None:
        self.max_asset_bytes = max_asset_bytes
        self.max_asset_count = max_asset_count
        self.max_total_asset_bytes = max_total_asset_bytes
        self.max_pages = max_pages
        self._lock = threading.Lock()
        self._converters: dict[tuple[object, ...], DocumentConverter] = {}
        self._ocr_engine: Any | None = None

    # ---------------------------------------------------------------- 初始化

    def initialize(self) -> None:
        """按默认解析选项构造并常驻 DocumentConverter（幂等，线程安全）。"""
        _ = self.converter_for(schemas.ParseOptions())
        self._ocr_engine = self._build_ocr_engine()

    def _build_ocr_engine(self) -> Any:
        """构造常驻 RapidOCR 引擎（模型随包内置，首次调用会下载/校验模型文件）。"""
        try:
            from rapidocr import RapidOCR

            return RapidOCR()
        except Exception as exc:  # 图片 OCR 不可用时仅降级，不阻断文档解析
            _log = logging.getLogger("document-parser")
            _log.warning("RapidOCR 初始化失败，图片 OCR 将降级跳过: %s", exc)
            return None

    def ocr_image(self, image_bytes: bytes) -> list[str]:
        """识别单张图片中的文字，按行返回（空图/失败返回空列表）。

        使用常驻 RapidOCR 引擎；引擎不可用时返回空列表，调用方自行降级。
        """
        if self._ocr_engine is None or not image_bytes:
            return []
        try:
            output = self._ocr_engine(image_bytes)
            if output is None:
                return []
            lines = list(output.txts or ())
            return [line.strip() for line in lines if line and line.strip()]
        except Exception:  # 单图 OCR 失败不影响文档整体
            return []

    def converter_for(self, options: schemas.ParseOptions) -> DocumentConverter:
        """按解析选项获取常驻 converter（相同选项复用同一实例）。"""
        key = (
            options.do_ocr,
            options.table_structure,
            tuple(sorted(_ocr_lang_codes(options.ocr_languages))),
        )
        with self._lock:
            converter = self._converters.get(key)
            if converter is None:
                converter = self._build_converter(options)
                self._converters[key] = converter
            return converter

    def _build_converter(self, options: schemas.ParseOptions) -> DocumentConverter:
        lang_codes = _ocr_lang_codes(options.ocr_languages)
        # 默认 layout 引擎启用 torch.compile（需要 MSVC cl.exe）；关闭编译换取无编译器依赖。
        layout_options = LayoutObjectDetectionOptions()
        layout_options.engine_options.compile_model = False
        pdf_options = PdfPipelineOptions(
            do_ocr=options.do_ocr,
            do_table_structure=options.table_structure,
            do_formula_enrichment=False,
            do_code_enrichment=False,
            ocr_options=RapidOcrOptions(lang=lang_codes, backend="onnxruntime"),
            table_structure_options=DoclingAdapter._table_options(),
            layout_options=layout_options,
        )
        return DocumentConverter(
            allowed_formats=[InputFormat.PDF, InputFormat.DOCX],
            format_options={
                InputFormat.PDF: FormatOption(
                    pipeline_cls=StandardPdfPipeline,
                    backend=DoclingParseDocumentBackend,
                    pipeline_options=pdf_options,
                ),
            },
        )

    @staticmethod
    def _table_options() -> Any:
        # TableStructureOptions（TableFormer V1 accurate）为默认值；保持默认避免依赖具体类名。
        from docling.datamodel.pipeline_options import TableStructureOptions

        return TableStructureOptions()

    # ---------------------------------------------------------------- 解析入口

    def parse(
        self,
        *,
        file_name: str,
        content: bytes,
        options: schemas.ParseOptions,
        docling_version: str,
    ) -> schemas.ParsedDocument:
        """将 PDF/DOCX 字节流转换为 ParsedDocument。

        - 返回协议对象，不写任何持久化存储；
        - 解析失败抛出 DocumentParserError（Go 侧不回退到其它解析器）。
        """
        fmt = _detect_format(file_name, content)
        if fmt is None:
            raise DocumentParserError(
                ParseFailureCode.INVALID_FORMAT,
                f"不支持或伪造格式: {file_name}",
            )
        if len(content) == 0:
            raise DocumentParserError(ParseFailureCode.INVALID_FORMAT, "空文件")

        converter = self.converter_for(options)
        stream = DocumentStream(name=file_name, stream=io.BytesIO(content))
        result = converter.convert(
            stream,
            raises_on_error=True,
            max_file_size=len(content),
            max_num_pages=self.max_pages,
        )
        if result.status in (ConversionStatus.FAILURE, ConversionStatus.PARTIAL_SUCCESS):
            raise DocumentParserError(
                ParseFailureCode.PARSE_FAILED,
                _format_errors(result.errors),
            )
        if result.status != ConversionStatus.SUCCESS or result.document is None:
            raise DocumentParserError(ParseFailureCode.PARSE_FAILED, "Docling 未返回文档")

        doc = result.document
        parsed = self._to_parsed_document(
            doc=doc,
            file_name=file_name,
            content=content,
            options=options,
            docling_version=docling_version,
            conv_errors=result.errors,
        )
        self.validate_references(parsed)
        return parsed

    # ---------------------------------------------------------------- 结构适配

    def _to_parsed_document(
        self,
        *,
        doc: DoclingDocument,
        file_name: str,
        content: bytes,
        options: schemas.ParseOptions,
        docling_version: str,
        conv_errors: list[Any],
    ) -> schemas.ParsedDocument:
        warnings: list[str] = [str(e) for e in conv_errors]
        page_count = _page_count(doc)

        tables_by_ref: dict[str, schemas.Table] = {}
        assets_by_ref: dict[str, schemas.Asset] = {}
        blocks: list[schemas.Block] = []
        id_pool = _IdPool()
        asset_bytes = 0

        heading_stack: list[tuple[int, str]] = []
        for item, level in doc.iterate_items():
            if not isinstance(item, DocItem):
                continue
            _trim_heading_stack(heading_stack, level)

            heading_path = [text for _, text in heading_stack]
            prov = _first_prov(item)
            source = schemas.SourceLocation(
                page=prov.page_no if prov is not None else 0,
                bbox=_bbox_list(prov) if options.include_bboxes else [],
                docling_ref=item.self_ref,
            )

            if isinstance(item, PictureItem):
                budget_ok = (
                    len(assets_by_ref) < self.max_asset_count
                    and asset_bytes < self.max_total_asset_bytes
                )
                asset = self._picture_to_asset(
                    item, doc, source, id_pool, warnings, options.extract_pictures and budget_ok
                )
                if asset is not None:
                    if not budget_ok:
                        warnings.append(
                            f"图片 {item.self_ref} 超过资产预算"
                            f"（数量 {self.max_asset_count} / 总量 {self.max_total_asset_bytes}），"
                            "标记 omitted"
                        )
                    if not asset.omitted:
                        asset_bytes += len(
                            base64.b64decode(asset.data_base64 or "")
                        )
                    assets_by_ref[item.self_ref] = asset
                    blocks.append(
                        schemas.Block(
                            id=id_pool.next("block"),
                            type="picture",
                            text="",
                            markdown="",
                            heading_path=heading_path,
                            source=source,
                            asset_refs=[asset.id],
                        )
                    )
                continue

            if isinstance(item, TableItem):
                table = _table_to_table(item, doc, source, id_pool, warnings)
                tables_by_ref[item.self_ref] = table
                blocks.append(
                    schemas.Block(
                        id=id_pool.next("block"),
                        type="table",
                        text=table.markdown,
                        markdown=table.markdown,
                        heading_path=heading_path,
                        source=source,
                        table_ref=table.id,
                    )
                )
                continue

            text = _item_text(item)
            if text is None:
                if item.label not in schemas.KNOWN_BLOCK_LABELS:
                    warnings.append(f"忽略无文本节点 {item.self_ref} label={item.label}")
                continue

            label = str(item.label.value)
            block_type = schemas.KNOWN_BLOCK_LABELS.get(label, "unknown")
            if block_type == "unknown":
                warnings.append(
                    f"未知 label {label!r} 映射为 unknown（节点 {item.self_ref}）"
                )

            if block_type in ("heading", "title"):
                heading_stack.append((level, text.strip()))

            block = schemas.Block(
                id=id_pool.next("block"),
                type=block_type,
                text=text,
                markdown=_block_markdown(block_type, text),
                heading_path=heading_path,
                source=source,
            )
            _attach_owner_refs(block, item, tables_by_ref, assets_by_ref, doc)
            blocks.append(block)

        title = _document_title(doc, file_name)
        parsed = schemas.ParsedDocument(
            schema_version=schemas.SCHEMA_VERSION,
            parser=schemas.ParserInfo(
                name=schemas.PARSER_NAME,
                version=docling_version,
                adapter_version=schemas.ADAPTER_VERSION,
            ),
            source=schemas.SourceInfo(
                file_name=file_name,
                format="pdf" if file_name.lower().endswith(".pdf") else "docx",
                sha256=_sha256_hex(content),
                size=len(content),
            ),
            document=schemas.DocumentInfo(
                title=title,
                markdown=_safe_markdown(doc),
                page_count=page_count,
                metadata={},
            ),
            blocks=blocks,
            tables=list(tables_by_ref.values()),
            assets=list(assets_by_ref.values()),
            warnings=warnings,
        )
        return parsed

    # ---------------------------------------------------------------- 图片

    def _picture_to_asset(
        self,
        item: PictureItem,
        doc: DoclingDocument,
        source: schemas.SourceLocation,
        id_pool: _IdPool,
        warnings: list[str],
        extract_pictures: bool,
    ) -> schemas.Asset | None:
        caption = _floating_caption(item, doc)
        base = schemas.Asset(
            id=id_pool.next("asset"),
            kind="picture",
            caption=caption,
            page=source.page,
            bbox=source.bbox,
            metadata={},
        )
        if not extract_pictures:
            base.omitted = True
            return base
        try:
            pil_image = _picture_pil(item, doc)
            if pil_image is None:
                warnings.append(
                    f"图片 {item.self_ref} 无法解码，标记 omitted（caption={caption!r}）"
                )
                base.omitted = True
                return base
            data = self._encode_asset(pil_image, item.self_ref)
        except AssetLimitError as exc:
            warnings.append(f"图片 {item.self_ref} {exc}")
            base.omitted = True
            return base
        except Exception as exc:  # 解码失败不阻断整个文档
            warnings.append(f"图片 {item.self_ref} 解码失败: {exc}")
            base.omitted = True
            return base

        return schemas.Asset(
            id=base.id,
            kind="picture",
            mime_type="image/png",
            sha256=_sha256_hex(data),
            width=pil_image.width,
            height=pil_image.height,
            page=source.page,
            bbox=source.bbox,
            caption=caption,
            data_base64=base64.b64encode(data).decode("ascii"),
            omitted=False,
            metadata={},
        )

    def _encode_asset(self, pil_image: PILImage.Image, self_ref: str) -> bytes:
        """编码图片并强制单图大小限制。"""
        if pil_image.width <= 0 or pil_image.height <= 0:
            raise AssetLimitError("图片尺寸无效")
        buffer = io.BytesIO()
        pil_image.save(buffer, format="PNG")
        data = buffer.getvalue()
        if len(data) > self.max_asset_bytes:
            raise AssetLimitError(
                f"超过单图大小限制 {self.max_asset_bytes} 字节（实际 {len(data)}）"
            )
        return data

    # ---------------------------------------------------------------- 校验

    def validate_references(self, parsed: schemas.ParsedDocument) -> None:
        """验证所有 Block/Table/Asset 引用完整；不完整时抛出错误。"""
        table_ids = {t.id for t in parsed.tables}
        asset_ids = {a.id for a in parsed.assets}
        for block in parsed.blocks:
            if block.table_ref is not None and block.table_ref not in table_ids:
                raise DocumentParserError(
                    ParseFailureCode.PARSE_FAILED,
                    f"Block {block.id} 引用不存在的 table_ref={block.table_ref}",
                )
            for ref in block.asset_refs:
                if ref not in asset_ids:
                    raise DocumentParserError(
                        ParseFailureCode.PARSE_FAILED,
                        f"Block {block.id} 引用不存在的 asset_ref={ref}",
                    )


# ---------------------------------------------------------------- 内部工具


class _IdPool:
    """生成结果内稳定 ID：block-000001 / table-000001 / asset-000001。"""

    def __init__(self) -> None:
        self._counters: dict[str, int] = {}

    def next(self, kind: str) -> str:
        n = self._counters.get(kind, 0) + 1
        self._counters[kind] = n
        return f"{kind}-{n:06d}"


def _first_prov(item: DocItem) -> Any:
    return item.prov[0] if item.prov else None


def _bbox_list(prov: Any) -> list[float]:
    if prov is None:
        return []
    bbox = prov.bbox
    return [round(bbox.l, 2), round(bbox.t, 2), round(bbox.r, 2), round(bbox.b, 2)]


def _item_text(item: DocItem) -> str | None:
    text = getattr(item, "text", None)
    if text is None:
        return None
    text = str(text)
    return text if text.strip() else None


def _block_markdown(block_type: str, text: str) -> str:
    if block_type == "code":
        return f"```\n{text}\n```"
    return text


def _floating_caption(item: FloatingItem, doc: DoclingDocument) -> str:
    try:
        return item.caption_text(doc).strip()
    except Exception:
        return ""


def _document_title(doc: DoclingDocument, file_name: str) -> str:
    for item, _level in doc.iterate_items():
        if isinstance(item, DocItem) and item.label == DocItemLabel.TITLE:
            text = getattr(item, "text", "")
            if text and str(text).strip():
                return str(text).strip()
    return file_name


def _page_count(doc: DoclingDocument) -> int:
    page_nos = [
        prov.page_no for item, _lvl in doc.iterate_items() for prov in getattr(item, "prov", [])
    ]
    return max(page_nos) if page_nos else len(doc.pages)


def _safe_markdown(doc: DoclingDocument) -> str:
    try:
        return doc.export_to_markdown()
    except Exception:
        return ""


def _sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _picture_pil(item: PictureItem, doc: DoclingDocument) -> PILImage.Image | None:
    """获取图片 PIL 对象：优先 item.image，其次页面裁剪图。"""
    if item.image is not None:
        try:
            pil = item.image.pil_image
            if pil is not None:
                return pil
        except Exception:
            pass
    try:
        pil = item.get_image(doc=doc)
        if pil is not None:
            return pil
    except Exception:
        pass
    return None


def _table_to_table(
    item: TableItem, doc: DoclingDocument, source: schemas.SourceLocation, id_pool: _IdPool, warnings: list[str]
) -> schemas.Table:
    data = item.data
    grid = data.grid
    num_rows = data.num_rows
    num_cols = data.num_cols

    header_rows: list[list[str]] = []
    body_rows: list[list[str]] = []
    for r in range(num_rows):
        row_cells = grid[r]
        texts = [_cell_text(c) for c in row_cells]
        if _is_header_row(row_cells):
            header_rows.append(texts)
        else:
            body_rows.append(texts)

    cells = []
    seen: set[tuple[int, int]] = set()
    for r in range(num_rows):
        for c in range(num_cols):
            cell = grid[r][c]
            if (r, c) in seen:
                continue
            # 跳过空占位（无文本、无合并）
            if cell.text == "" and cell.row_span <= 1 and cell.col_span <= 1:
                continue
            cells.append(
                schemas.TableCell(
                    row=r,
                    column=c,
                    row_span=cell.row_span,
                    col_span=cell.col_span,
                    text=cell.text,
                )
            )
            for rr in range(r, r + cell.row_span):
                for cc in range(c, c + cell.col_span):
                    seen.add((rr, cc))

    page_nos = [prov.page_no for prov in item.prov]
    caption = _floating_caption(item, doc)
    markdown = _safe_table_markdown(item, doc)
    return schemas.Table(
        id=id_pool.next("table"),
        caption=caption,
        page_start=min(page_nos) if page_nos else 0,
        page_end=max(page_nos) if page_nos else 0,
        bbox=source.bbox,
        headers=header_rows,
        rows=body_rows,
        cells=cells,
        row_count=num_rows,
        column_count=num_cols,
        markdown=markdown,
    )


def _is_header_row(cells: list[Any]) -> bool:
    flags = [bool(getattr(c, "column_header", False)) for c in cells]
    return any(flags)


def _cell_text(cell: Any) -> str:
    text = getattr(cell, "text", "")
    return str(text) if text is not None else ""


def _safe_table_markdown(item: TableItem, doc: DoclingDocument) -> str:
    try:
        return item.export_to_markdown(doc=doc)
    except Exception:
        return ""


def _trim_heading_stack(stack: list[tuple[int, str]], level: int) -> None:
    while stack and stack[-1][0] >= level:
        stack.pop()


def _attach_owner_refs(
    block: schemas.Block,
    item: DocItem,
    tables_by_ref: dict[str, schemas.Table],
    assets_by_ref: dict[str, schemas.Asset],
    doc: DoclingDocument,
) -> None:
    """caption/footnote 等子节点回填其所属 Table/Picture 引用。"""
    if block.type not in ("caption", "footnote", "reference", "unknown"):
        return
    parent = item.parent
    if parent is None:
        return
    try:
        parent_item = parent.resolve(doc)
    except Exception:
        return
    if isinstance(parent_item, TableItem):
        table = tables_by_ref.get(parent_item.self_ref)
        if table is not None:
            block.table_ref = table.id
    elif isinstance(parent_item, PictureItem):
        asset = assets_by_ref.get(parent_item.self_ref)
        if asset is not None:
            block.asset_refs = [asset.id]


def _format_errors(errors: list[Any]) -> str:
    if not errors:
        return "Docling 转换失败（无错误详情）"
    return "; ".join(str(getattr(e, "error_message", e)) for e in errors[:5])
