from typing import Optional
import httpx
import config

def _get(path: str, params: dict) -> dict | list:
    clean_params = {k: v for k, v in params.items() if v is not None}
    with httpx.Client(timeout=10) as client:
        resp = client.get(f"{config.API_BASE}{path}", params=clean_params)
        resp.raise_for_status()
        return resp.json()

def query_packets(
    limit: int = 100,
    protocol: Optional[str] = None,
    src_ip: Optional[str] = None,
    dst_ip: Optional[str] = None,
    src_port: Optional[int] = None,
    dst_port: Optional[int] = None,
    min_length: Optional[int] = None,
    max_length: Optional[int] = None,
    order: str = "desc",
    cursor: Optional[str] = None,
    job_id: Optional[str] = None,
) -> dict:
    """
    Query packet logs from the network forensics database.

    Use this to answer questions like:
    - "Show me all TCP traffic"
    - "What packets came from 192.168.1.1?"
    - "Find large packets over 1500 bytes"
    - "Show recent DNS traffic" (UDP port 53)

    Args:
        limit: Number of packets to return (1-100, default 100)
        protocol: Filter by protocol name e.g. "TCP", "UDP", "ICMP"
        src_ip: Filter by source IP address e.g. "192.168.1.1"
        dst_ip: Filter by destination IP address e.g. "10.0.0.1"
        src_port: Filter by source port number
        dst_port: Filter by destination port number
        min_length: Minimum packet length in bytes
        max_length: Maximum packet length in bytes
        order: Sort order "asc" or "desc" by time (default "desc" = most recent first)
        cursor: RFC3339 timestamp cursor for pagination (from next_cursor in a previous response)
        job_id: Filter to packets from a specific capture job UUID
    """
    filters = []
    if protocol:
        filters.append(f"protocol:eq:{protocol}")
    if src_ip:
        filters.append(f"src_ip:eq:{src_ip}")
    if dst_ip:
        filters.append(f"dst_ip:eq:{dst_ip}")
    if src_port is not None:
        filters.append(f"src_port:eq:{src_port}")
    if dst_port is not None:
        filters.append(f"dst_port:eq:{dst_port}")
    if min_length is not None:
        filters.append(f"length:gte:{min_length}")
    if max_length is not None:
        filters.append(f"length:lte:{max_length}")
    if job_id:
        filters.append(f"job_id:eq:{job_id}")

    params: dict = {"limit": limit, "order": order, "cursor": cursor}
    if filters:
        params["filter"] = filters

    return _get("/api/v1/packets", params)


def list_jobs() -> list:
    """
    List all packet capture jobs, ordered by most recent first.

    Use this to see what capture sessions exist, their status
    (PROCESSING or COMPLETED), how many files were processed,
    and what directory the captures came from.
    """
    return _get("/api/v1/jobs", {})


def get_job(job_id: str) -> dict:
    """
    Get details for a specific capture job by its UUID.

    Args:
        job_id: The UUID of the job e.g. "550e8400-e29b-41d4-a716-446655440000"
    """
    return _get(f"/api/v1/jobs/{job_id}", {})

