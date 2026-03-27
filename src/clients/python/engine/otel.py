"""OpenTelemetry SDK initialization for Python strategy clients.

Mirrors Go utils.SetupOTelSDK. Creates TracerProvider and MeterProvider
with OTLP HTTP exporters. Configuration via OTEL_* environment variables.
"""
import os
import logging

from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.semconv.resource import ResourceAttributes

logger = logging.getLogger(__name__)

_initialized = False
_shutdown_fn = None


def setup_otel(service_name: str = None, service_version: str = "1.0.0"):
    """Initialize OTel SDK with OTLP HTTP exporters.

    Reads OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME from env vars.
    Returns a shutdown callable for cleanup.

    Idempotent: calling multiple times returns the same shutdown callable
    without creating duplicate providers.
    """
    global _initialized, _shutdown_fn

    if _initialized:
        logger.warning("setup_otel() already called; returning existing shutdown callable")
        return _shutdown_fn

    if service_name is None:
        service_name = os.getenv("OTEL_SERVICE_NAME", "grodt-strategy")

    resource = Resource.create({
        ResourceAttributes.SERVICE_NAME: service_name,
        ResourceAttributes.SERVICE_VERSION: service_version,
    })

    # Traces
    trace_exporter = OTLPSpanExporter()  # reads OTEL_EXPORTER_OTLP_ENDPOINT
    tracer_provider = TracerProvider(resource=resource)
    tracer_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
    trace.set_tracer_provider(tracer_provider)

    # Metrics
    metric_exporter = OTLPMetricExporter()  # reads OTEL_EXPORTER_OTLP_ENDPOINT
    metric_reader = PeriodicExportingMetricReader(metric_exporter)
    meter_provider = MeterProvider(resource=resource, metric_readers=[metric_reader])
    metrics.set_meter_provider(meter_provider)

    def shutdown():
        """Flush and shut down both providers."""
        tracer_provider.force_flush()
        tracer_provider.shutdown()
        meter_provider.force_flush()
        meter_provider.shutdown()

    _initialized = True
    _shutdown_fn = shutdown

    logger.info("OTel SDK initialized: service_name=%s", service_name)
    return shutdown
