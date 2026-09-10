"""
Tests for tools/packets.py

All HTTP calls are mocked — no running API required.
"""

import pytest
from unittest.mock import MagicMock, patch


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
def make_response(json_data, status_code=200):
    resp = MagicMock()
    resp.status_code = status_code
    resp.json.return_value = json_data
    resp.raise_for_status = MagicMock()
    return resp


# ---------------------------------------------------------------------------
# _get helper
# ---------------------------------------------------------------------------
class TestGet:
    def test_strips_none_params(self):
        """None values should not appear in the outgoing request."""
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import _get
            _get("/api/v1/packets", {"limit": 10, "cursor": None})

            _, kwargs = mock_client.get.call_args
            assert "cursor" not in kwargs.get("params", {})
            assert kwargs["params"]["limit"] == 10

    def test_raises_on_http_error(self):
        import httpx
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            resp = MagicMock()
            resp.raise_for_status.side_effect = httpx.HTTPStatusError(
                "404", request=MagicMock(), response=MagicMock()
            )
            mock_client.get.return_value = resp

            from tools.packets import _get
            with pytest.raises(httpx.HTTPStatusError):
                _get("/api/v1/packets", {})


# ---------------------------------------------------------------------------
# query_packets
# ---------------------------------------------------------------------------
class TestQueryPackets:
    def _call(self, mock_client, **kwargs):
        from tools.packets import query_packets
        return query_packets(**kwargs)

    def test_no_filters_sends_only_limit_and_order(self):
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import query_packets
            query_packets()

            _, kwargs = mock_client.get.call_args
            params = kwargs["params"]
            assert params["limit"] == 100
            assert params["order"] == "desc"
            assert "filter" not in params

    def test_protocol_filter(self):
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import query_packets
            query_packets(protocol="UDP")

            _, kwargs = mock_client.get.call_args
            assert "protocol:eq:UDP" in kwargs["params"]["filter"]

    def test_multiple_filters(self):
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import query_packets
            query_packets(src_ip="192.168.1.1", dst_port=443, min_length=100)

            _, kwargs = mock_client.get.call_args
            filters = kwargs["params"]["filter"]
            assert "src_ip:eq:192.168.1.1" in filters
            assert "dst_port:eq:443" in filters
            assert "length:gte:100" in filters

    def test_max_length_filter(self):
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import query_packets
            query_packets(max_length=500)

            _, kwargs = mock_client.get.call_args
            assert "length:lte:500" in kwargs["params"]["filter"]

    def test_port_zero_included(self):
        """Port 0 is falsy but should still produce a filter."""
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"data": []})

            from tools.packets import query_packets
            query_packets(src_port=0)

            _, kwargs = mock_client.get.call_args
            assert "src_port:eq:0" in kwargs["params"]["filter"]

    def test_returns_api_response(self):
        expected = {"data": [{"src_ip": "1.2.3.4"}], "next_cursor": None}
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response(expected)

            from tools.packets import query_packets
            result = query_packets()
            assert result == expected


# ---------------------------------------------------------------------------
# list_jobs
# ---------------------------------------------------------------------------
class TestListJobs:
    def test_returns_list(self):
        expected = [{"job_id": "abc", "status": "COMPLETED"}]
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response(expected)

            from tools.packets import list_jobs
            result = list_jobs()
            assert result == expected

    def test_hits_correct_endpoint(self):
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response([])

            from tools.packets import list_jobs
            list_jobs()

            args, _ = mock_client.get.call_args
            assert args[0].endswith("/api/v1/jobs")


# ---------------------------------------------------------------------------
# get_job
# ---------------------------------------------------------------------------
class TestGetJob:
    def test_hits_correct_endpoint(self):
        job_id = "550e8400-e29b-41d4-a716-446655440000"
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response({"job_id": job_id})

            from tools.packets import get_job
            get_job(job_id)

            args, _ = mock_client.get.call_args
            assert args[0].endswith(f"/api/v1/jobs/{job_id}")

    def test_returns_job_data(self):
        expected = {"job_id": "abc", "status": "PROCESSING"}
        with patch("httpx.Client") as mock_client_cls:
            mock_client = MagicMock()
            mock_client_cls.return_value.__enter__.return_value = mock_client
            mock_client.get.return_value = make_response(expected)

            from tools.packets import get_job
            result = get_job("abc")
            assert result == expected
