# main.py
from fastmcp import FastMCP
from starlette.routing import Route

from tools.packets import query_packets, list_jobs, get_job
from api.chat import chat

mcp = FastMCP("Network Forensics")
mcp.tool()(query_packets)
mcp.tool()(list_jobs)
mcp.tool()(get_job)

app = mcp.http_app()
app.routes.append(
    Route("/chat", endpoint=chat, methods=["POST"])
)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)