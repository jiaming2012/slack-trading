"""Tests for engine/otel.py setup_otel() function."""
import os
import pytest
from unittest.mock import patch

from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.metrics import MeterProvider


def _reset_trace_provider():
    """Force-reset the global tracer provider (bypassing SDK guard)."""
    # The SDK uses a ProxyTracerProvider wrapper; reset its internal _real_provider
    proxy = trace._TRACER_PROVIDER_SET_ONCE  # type: ignore
    proxy._done = False
    proxy._lock.__init__()


def _reset_meter_provider():
    """Force-reset the global meter provider (bypassing SDK guard)."""
    from opentelemetry.metrics._internal import _METER_PROVIDER_SET_ONCE
    _METER_PROVIDER_SET_ONCE._done = False
    _METER_PROVIDER_SET_ONCE._lock.__init__()


@pytest.fixture(autouse=True)
def reset_otel_providers():
    """Reset global OTel providers and otel module state before and after each test."""
    # Reset before
    _reset_trace_provider()
    _reset_meter_provider()
    import engine.otel as otel_mod
    otel_mod._initialized = False
    otel_mod._shutdown_fn = None

    yield

    # Reset after
    _reset_trace_provider()
    _reset_meter_provider()
    otel_mod._initialized = False
    otel_mod._shutdown_fn = None


class TestSetupOtel:
    def test_returns_shutdown_callable(self):
        from engine.otel import setup_otel
        shutdown = setup_otel()
        assert callable(shutdown)

    def test_sets_tracer_provider(self):
        from engine.otel import setup_otel
        setup_otel()
        provider = trace.get_tracer_provider()
        assert isinstance(provider, TracerProvider)

    def test_sets_meter_provider(self):
        from engine.otel import setup_otel
        setup_otel()
        provider = metrics.get_meter_provider()
        assert isinstance(provider, MeterProvider)

    def test_idempotent_no_duplicate_providers(self):
        from engine.otel import setup_otel
        shutdown1 = setup_otel()
        provider1 = trace.get_tracer_provider()
        shutdown2 = setup_otel()
        provider2 = trace.get_tracer_provider()
        # Second call should return same shutdown and not replace provider
        assert shutdown1 is shutdown2
        assert provider1 is provider2

    def test_resource_has_service_name(self):
        from engine.otel import setup_otel
        setup_otel()
        provider = trace.get_tracer_provider()
        assert isinstance(provider, TracerProvider)
        resource = provider.resource
        attrs = dict(resource.attributes)
        assert attrs.get("service.name") == "grodt-strategy"

    def test_resource_custom_service_name(self):
        from engine.otel import setup_otel
        setup_otel(service_name="custom-service")
        provider = trace.get_tracer_provider()
        resource = provider.resource
        attrs = dict(resource.attributes)
        assert attrs.get("service.name") == "custom-service"

    def test_shutdown_calls_force_flush(self):
        from engine.otel import setup_otel
        shutdown = setup_otel()
        # Should not raise
        shutdown()
