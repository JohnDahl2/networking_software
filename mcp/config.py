import os

API_BASE = os.getenv("GO_SNIFFER_URL", "http://localhost:3000")
PROVIDER = os.getenv("PROVIDER", "openai").lower()
MODEL = os.getenv("MODEL", "gpt-4o-mini")
MAX_TOKENS = int(os.getenv("MAX_TOKENS", "1024"))
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")