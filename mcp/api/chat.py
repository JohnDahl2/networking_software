import json

from starlette.requests import Request
from starlette.responses import JSONResponse

import config
from tools.packets import query_packets, list_jobs, get_job

TOOL_DISPATCH = {
    "query_packets": lambda args: query_packets(**args),
    "list_jobs":     lambda args: list_jobs(),
    "get_job":       lambda args: get_job(**args),
}


def _get_tools():
    if config.PROVIDER == "openai":
        from tools.open_ai_tools import OPENAI_TOOLS
        return OPENAI_TOOLS
    raise ValueError(f"Unsupported provider: {config.PROVIDER!r}")

def _make_llm_client():
    if config.PROVIDER == "openai":
        from openai import OpenAI
        return OpenAI(api_key=config.OPENAI_API_KEY)
    raise ValueError(f"Unsupported provider: {config.PROVIDER!r}")


def _run_agent_loop(user_message: str) -> str:
    client = _make_llm_client()
    tools = _get_tools()
    messages = [{"role": "user", "content": user_message}]

    while True:
        response = client.chat.completions.create(
            model=config.MODEL,
            max_tokens=config.MAX_TOKENS,
            tools=tools,
            messages=messages,
        )
        choice = response.choices[0]

        if choice.finish_reason == "stop":
            return choice.message.content

        messages.append(choice.message)
        for tc in choice.message.tool_calls:
            args = json.loads(tc.function.arguments)
            result = TOOL_DISPATCH[tc.function.name](args)
            messages.append({
                "role": "tool",
                "tool_call_id": tc.id,
                "content": json.dumps(result),
            })


async def chat(request: Request) -> JSONResponse:
    try:
        body = await request.json()
    except Exception:
        return JSONResponse({"error": "invalid JSON body"}, status_code=400)

    message = body.get("message", "").strip()
    if not message:
        return JSONResponse({"error": "message is required"}, status_code=400)

    try:
        answer = _run_agent_loop(message)
        return JSONResponse({"response": answer})
    except Exception as e:
        return JSONResponse({"error": str(e)}, status_code=500)