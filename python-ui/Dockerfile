FROM python:3.11-slim

# Install system dependencies for Postgres driver
RUN apt-get update && apt-get install -y libpq-dev gcc

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Expose Streamlit's default port
EXPOSE 8501

CMD ["streamlit", "run", "app.py", "--server.port=8501", "--server.address=0.0.0.0"]