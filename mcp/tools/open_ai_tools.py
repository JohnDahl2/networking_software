OPENAI_TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "query_packets",
            "description": "Query packet logs. Use for questions about traffic, IPs, protocols, ports, or packet sizes.",
            "parameters": {
                "type": "object",
                "properties": {
                    "limit":      {"type": "integer", "default": 100},
                    "protocol":   {"type": "string"},
                    "src_ip":     {"type": "string"},
                    "dst_ip":     {"type": "string"},
                    "src_port":   {"type": "integer"},
                    "dst_port":   {"type": "integer"},
                    "min_length": {"type": "integer"},
                    "max_length": {"type": "integer"},
                    "order":      {"type": "string", "enum": ["asc", "desc"], "default": "desc"},
                    "cursor":     {"type": "string"},
                    "job_id":     {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "list_jobs",
            "description": "List all packet capture jobs and their status.",
            "parameters": {"type": "object", "properties": {}},
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_job",
            "description": "Get details for a specific capture job by UUID.",
            "parameters": {
                "type": "object",
                "properties": {
                    "job_id": {"type": "string"},
                },
                "required": ["job_id"],
            },
        },
    },
]