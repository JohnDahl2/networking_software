import streamlit as st
import pandas as pd
import psycopg2
import os
import time
from datetime import datetime, timedelta

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
    # Define time window for the query
    minutes = int(time_window.split()[0])
    since = datetime.now() - timedelta(minutes=minutes)
    
    query = """
    SELECT time, length, info 
    FROM packet_summary 
    WHERE time > %s 
    ORDER BY time DESC 
    LIMIT 1000
    """
    df = pd.read_sql(query, conn, params=(since,))
    conn.close()
    return df

# 4. Main Dashboard UI
placeholder = st.empty()

while True:
    with placeholder.container():
        df = fetch_data()
        
        # Summary Metrics
        col1, col2 = st.columns(2)
        col1.metric("Total Packets (Window)", len(df))
        col2.metric("Avg Packet Size", f"{int(df['length'].mean() if not df.empty else 0)} bytes")
        
        # Line Chart: Traffic over time
        st.subheader("Traffic Volume")
        if not df.empty:
            # Resample data to 1-second buckets for a smooth chart
            df['time'] = pd.to_datetime(df['time'])
            chart_data = df.set_index('time').resample('1S').count()['length']
            st.line_chart(chart_data)
        
        # Raw Data Table
        st.subheader("Recent Packets")
        st.dataframe(df, use_container_width=True)
        
        time.sleep(refresh_rate)