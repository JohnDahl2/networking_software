import streamlit as st
import pandas as pd
import psycopg2
import requests
import os
import time
from datetime import datetime, timedelta

MCP_URL = os.getenv("MCP_URL", "http://mcp:8000/chat")

# 1. Database Connection Logic
def get_connection():
    return psycopg2.connect(os.getenv("DATABASE_URL"))

st.set_page_config(page_title="Network Packet Monitor", layout="wide")
st.title("📡 Real-Time Packet Monitor")

# 2. Sidebar Controls
refresh_rate = st.sidebar.slider("Refresh Rate (seconds)", 1, 10, 2)
time_window = st.sidebar.selectbox("Time Window", ["5 minutes", "15 minutes", "1 hour"])

# 3. Data Fetching Function
def fetch_data():
    conn = get_connection()
    minutes = int(time_window.split()[0])
    since = datetime.now() - timedelta(minutes=minutes)
    query = """
    SELECT time, src_ip, dst_ip, src_port, dst_port, protocol, length, tcp_flags
    FROM packet_logs
    WHERE time > %s
    ORDER BY time DESC
    LIMIT 1000
    """
    df = pd.read_sql(query, conn, params=(since,))
    conn.close()
    return df

# 4. AI Query Section
st.subheader("Ask about your network data")

if "chat_history" not in st.session_state:
    st.session_state.chat_history = []

for entry in st.session_state.chat_history:
    with st.chat_message(entry["role"]):
        st.write(entry["content"])

prompt = st.chat_input("e.g. Show me all UDP traffic, or which IP sent the most packets?")
if prompt:
    st.session_state.chat_history.append({"role": "user", "content": prompt})
    with st.chat_message("user"):
        st.write(prompt)

    with st.chat_message("assistant"):
        with st.spinner("Thinking..."):
            try:
                resp = requests.post(MCP_URL, json={"message": prompt}, timeout=30)
                resp.raise_for_status()
                answer = resp.json().get("response", "No response received.")
            except Exception as e:
                answer = f"Error contacting MCP server: {e}"
        st.write(answer)
        st.session_state.chat_history.append({"role": "assistant", "content": answer})

st.divider()

# 5. Main Dashboard UI
placeholder = st.empty()

while True:
    with placeholder.container():
        try:
            df = fetch_data()

            col1, col2 = st.columns(2)
            col1.metric("Total Packets (Window)", len(df))
            col2.metric("Avg Packet Size", f"{int(df['length'].mean() if not df.empty else 0)} bytes")

            st.subheader("Traffic Volume")
            if not df.empty:
                df['time'] = pd.to_datetime(df['time'])
                chart_data = df.set_index('time').resample('1S').count()['length']
                st.line_chart(chart_data)

            st.subheader("Recent Packets")
            st.dataframe(df, use_container_width=True)
        except Exception as e:
            st.error(f"Database error: {e}")

        time.sleep(refresh_rate)
