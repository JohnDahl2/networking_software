"""
Tests for api/chat.py

LLM and tool calls are fully mocked — no OpenAI API key required.
"""

import json
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from starlette.testclient import TestClient
from starlette.applications import Starlette
from starlette.routing import Route


# ---------------------------------------------------------------------------
# Build a minimal test app that mounts the chat endpoint
# ---------------------------------------------------------------------------
def make_app():
    from api.chat import chat
    return Starlette(routes=[Route("/chat", endpoint=chat, methods=["POST"])])


# ---------------------------------------------------------------------------
# Helpers to build fake OpenAI responses
# ---------------------------------------------------------------------------
def make_stop_response(content: str):
    """Simulate a final text response (no tool calls)."""
    choice = MagicMock()
    choice.finish_reason = "stop"
    choice.message.content = content
    choice.message.tool_calls = None
    resp = MagicMock()
    resp.choices = [choice]
    return resp


def make_tool_call_response(tool_name: str, arguments: dict, call_id="call_1"):
    """Simulate a response that calls a tool."""
    tc = MagicMock()
    tc.id = call_id
    tc.function.name = tool_name
    tc.function.arguments = json.dumps(arguments)

    choice = MagicMock()
    choice.finish_reason = "tool_calls"
    choice.message.tool_calls = [tc]
    choice.message.content = None

    resp = MagicMock()
    resp.choices = [choice]
    return resp


# ---------------------------------------------------------------------------
# /chat endpoint tests
# ---------------------------------------------------------------------------
class TestChatEndpoint:
    def test_missing_message_returns_400(self):
        client = TestClient(make_app(), raise_server_exceptions=False)
        resp = client.post("/chat", json={})
        assert resp.status_code == 400
        assert "message is required" in resp.json()["error"]

    def test_empty_message_returns_400(self):
        client = TestClient(make_app(), raise_server_exceptions=False)
        resp = client.post("/chat", json={"message": "   "})
        assert resp.status_code == 400

    def test_invalid_json_returns_400(self):
        client = TestClient(make_app(), raise_server_exceptions=False)
        resp = client.post("/chat", content=b"not-json",
                           headers={"Content-Type": "application/json"})
        assert resp.status_code == 400

    def test_successful_response(self):
        with patch("api.chat._make_llm_client") as mock_llm, \
             patch("api.chat._get_tools", return_value=[]):
            mock_client = MagicMock()
            mock_llm.return_value = mock_client
            mock_client.chat.completions.create.return_value = make_stop_response(
                "Here is your data."
            )

            client = TestClient(make_app())
            resp = client.post("/chat", json={"message": "show me UDP traffic"})
            assert resp.status_code == 200
            assert resp.json()["response"] == "Here is your data."

    def test_llm_error_returns_500(self):
        with patch("api.chat._make_llm_client") as mock_llm, \
             patch("api.chat._get_tools", return_value=[]):
            mock_client = MagicMock()
            mock_llm.return_value = mock_client
            mock_client.chat.completions.create.side_effect = Exception("API down")

            client = TestClient(make_app(), raise_server_exceptions=False)
            resp = client.post("/chat", json={"message": "show me UDP traffic"})
            assert resp.status_code == 500
            assert "API down" in resp.json()["error"]


# ---------------------------------------------------------------------------
# _run_agent_loop tests
# ---------------------------------------------------------------------------
class TestRunAgentLoop:
    def test_single_turn_no_tool_calls(self):
        with patch("api.chat._make_llm_client") as mock_llm, \
             patch("api.chat._get_tools", return_value=[]):
            mock_client = MagicMock()
            mock_llm.return_value = mock_client
            mock_client.chat.completions.create.return_value = make_stop_response("Done.")

            from api.chat import _run_agent_loop
            result = _run_agent_loop("hello")
            assert result == "Done."

    def test_tool_call_then_final_answer(self):
        fake_packets = {"data": [{"src_ip": "1.2.3.4", "protocol": "UDP"}]}

        with patch("api.chat._make_llm_client") as mock_llm, \
             patch("api.chat._get_tools", return_value=[]), \
             patch("api.chat.TOOL_DISPATCH", {
                 "query_packets": lambda args: fake_packets
             }):
            mock_client = MagicMock()
            mock_llm.return_value = mock_client
            mock_client.chat.completions.create.side_effect = [
                make_tool_call_response("query_packets", {"protocol": "UDP"}),
                make_stop_response("Found 1 UDP packet from 1.2.3.4."),
            ]

            from api.chat import _run_agent_loop
            result = _run_agent_loop("show me UDP traffic")
            assert "1.2.3.4" in result
            assert mock_client.chat.completions.create.call_count == 2


# ---------------------------------------------------------------------------
# _get_tools tests
# ---------------------------------------------------------------------------
class TestGetTools:
    def test_openai_provider_returns_tools(self):
        with patch("config.PROVIDER", "openai"):
            from api.chat import _get_tools
            tools = _get_tools()
            assert isinstance(tools, list)
            assert len(tools) > 0

    def test_unknown_provider_raises(self):
        with patch("config.PROVIDER", "unknown_provider"):
            from api.chat import _get_tools
            with pytest.raises(ValueError, match="Unsupported provider"):
                _get_tools()
