"""Tests for W3C traceparent header propagation in Twirp RPC calls.

Verifies that _network_call_with_retry_inner injects traceparent headers
when an OTel span is active, and does not inject them when no span is active.
"""
import os
import re
import unittest
from unittest.mock import MagicMock, patch

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider

from engine.client import BacktesterPlaygroundClient


# The suite runs under OTEL_SDK_DISABLED=true, which makes TracerProvider emit
# non-recording spans whose context does not propagate. Tests that assert
# traceparent *injection* need a recording span, so they build their provider
# with OTEL_SDK_DISABLED cleared.


class TestTraceparentInjection(unittest.TestCase):
    """Test W3C traceparent injection in _network_call_with_retry_inner."""

    def _make_client_stub(self):
        """Create a minimal BacktesterPlaygroundClient without calling __init__."""
        client = object.__new__(BacktesterPlaygroundClient)
        client.logger = MagicMock()
        client.profiler = None
        return client

    def test_traceparent_injected_when_span_active(self):
        """When an OTel span is active, traceparent header is injected into Context."""
        captured_ctx = {}

        def mock_rpc_client(ctx, request):
            # Capture the context headers for inspection
            captured_ctx["headers"] = ctx._headers if hasattr(ctx, '_headers') else {}
            return MagicMock()

        client = self._make_client_stub()

        with patch.dict(os.environ):
            os.environ.pop("OTEL_SDK_DISABLED", None)  # allow span context to record/propagate
            # Set up a real TracerProvider so spans produce valid trace context
            provider = TracerProvider()
            tracer = provider.get_tracer("test")
            with tracer.start_as_current_span("test-span"):
                client._network_call_with_retry_inner(
                    "test_caller", mock_rpc_client, MagicMock(), backoff=1, max_backoff=1
                )

        headers = captured_ctx.get("headers", {})
        self.assertIn("traceparent", headers,
                       "traceparent header should be present when span is active")

        # Validate W3C traceparent format: 00-{32hex}-{16hex}-{2hex}
        traceparent = headers["traceparent"]
        pattern = r"^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$"
        self.assertRegex(traceparent, pattern,
                         f"traceparent '{traceparent}' does not match W3C format")

    def test_no_traceparent_when_no_span_active(self):
        """When no OTel span is active, traceparent header is NOT injected."""
        captured_ctx = {}

        def mock_rpc_client(ctx, request):
            captured_ctx["headers"] = ctx._headers if hasattr(ctx, '_headers') else {}
            return MagicMock()

        client = self._make_client_stub()

        # No span context active -- inject() should be a no-op
        client._network_call_with_retry_inner(
            "test_caller", mock_rpc_client, MagicMock(), backoff=1, max_backoff=1
        )

        headers = captured_ctx.get("headers", {})
        self.assertNotIn("traceparent", headers,
                         "traceparent header should NOT be present when no span is active")

    def test_traceparent_format_valid_w3c(self):
        """The traceparent header conforms to W3C Trace Context format."""
        captured_ctx = {}

        def mock_rpc_client(ctx, request):
            captured_ctx["headers"] = ctx._headers if hasattr(ctx, '_headers') else {}
            return MagicMock()

        client = self._make_client_stub()

        with patch.dict(os.environ):
            os.environ.pop("OTEL_SDK_DISABLED", None)  # allow span context to record/propagate
            provider = TracerProvider()
            tracer = provider.get_tracer("test-format")
            with tracer.start_as_current_span("format-test-span"):
                client._network_call_with_retry_inner(
                    "test_caller", mock_rpc_client, MagicMock(), backoff=1, max_backoff=1
                )

        traceparent = captured_ctx["headers"]["traceparent"]
        parts = traceparent.split("-")
        self.assertEqual(len(parts), 4, "traceparent should have 4 parts separated by '-'")
        self.assertEqual(parts[0], "00", "version should be '00'")
        self.assertEqual(len(parts[1]), 32, "trace-id should be 32 hex chars")
        self.assertEqual(len(parts[2]), 16, "parent-id should be 16 hex chars")
        self.assertEqual(len(parts[3]), 2, "trace-flags should be 2 hex chars")


if __name__ == "__main__":
    unittest.main()
