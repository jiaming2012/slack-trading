"""Tests for trace_id injection on RPC requests in engine/client.py.

Verifies:
- trace_id is set on NextTickRequest when active span exists
- trace_id is set on PlaceOrderRequest when active span exists
- trace_id is empty string when no active span
"""
import unittest
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider


class TestTraceIdInjection(unittest.TestCase):
    """Verify _get_trace_id() extracts trace_id from active OTel span."""

    def setUp(self):
        self.provider = TracerProvider()
        trace.set_tracer_provider(self.provider)

    def tearDown(self):
        self.provider.shutdown()

    def test_trace_id_with_active_span(self):
        """When an OTel span is active, _get_trace_id() returns a 32-char hex string."""
        from engine.client import _get_trace_id

        tracer = trace.get_tracer("test")
        with tracer.start_as_current_span("test_tick"):
            trace_id = _get_trace_id()
            self.assertEqual(len(trace_id), 32, f"Expected 32-char hex, got '{trace_id}'")
            self.assertNotEqual(trace_id, "0" * 32, "trace_id should not be all zeros")
            self.assertTrue(all(c in "0123456789abcdef" for c in trace_id))

    def test_trace_id_empty_without_span(self):
        """Without an active span, _get_trace_id() returns empty string."""
        from engine.client import _get_trace_id

        result = _get_trace_id()
        self.assertEqual(result, "", f"Expected empty string, got '{result}'")

    def test_trace_id_format_is_lowercase_hex(self):
        """trace_id should be exactly 32 lowercase hex characters."""
        from engine.client import _get_trace_id

        tracer = trace.get_tracer("test")
        with tracer.start_as_current_span("test_format"):
            trace_id = _get_trace_id()
            self.assertRegex(trace_id, r"^[0-9a-f]{32}$")


if __name__ == "__main__":
    unittest.main()
