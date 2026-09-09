"""document-parser 的 W3C Trace Context 与可选 OTLP 导出。"""

from __future__ import annotations

import logging
import os

from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased

_provider: TracerProvider | None = None


class _TraceLogFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        span_context = trace.get_current_span().get_span_context()
        record.trace_id = format(span_context.trace_id, "032x") if span_context.is_valid else "-"
        return True


def install_trace_log_filter() -> None:
    trace_filter = _TraceLogFilter()
    for handler in logging.getLogger().handlers:
        handler.addFilter(trace_filter)


def configure_tracing(app: FastAPI) -> None:
    global _provider
    if os.environ.get("DOCUMENT_PARSER_OBSERVABILITY_ENABLED", "true").lower() == "false":
        return

    ratio = min(1.0, max(0.0, float(os.environ.get("DOCUMENT_PARSER_TRACE_SAMPLE_RATIO", "1.0"))))
    provider = TracerProvider(
        resource=Resource.create(
            {"service.name": os.environ.get("OTEL_SERVICE_NAME", "memora-document-parser")}
        ),
        sampler=ParentBased(TraceIdRatioBased(ratio)),
    )
    endpoint = os.environ.get("DOCUMENT_PARSER_OTLP_ENDPOINT", "").strip()
    standard_endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", "").strip()
    if endpoint or standard_endpoint:
        exporter = OTLPSpanExporter(endpoint=f"{endpoint.rstrip('/')}/v1/traces") if endpoint else OTLPSpanExporter()
        provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app, tracer_provider=provider, excluded_urls="/health/live,/health/ready")
    _provider = provider


def shutdown_tracing() -> None:
    global _provider
    if _provider is not None:
        _provider.shutdown()
        _provider = None
