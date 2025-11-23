FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Create a user for security
RUN useradd -m yupsuser
USER yupsuser

# Environment variables will be passed via docker-compose
ENV FLASK_APP=app.py

# Expose port
EXPOSE 5000

# Use Gunicorn for production-grade server
# -w 1: Single worker to keep rate-limiting memory shared simply
# -b: Bind address
CMD ["gunicorn", "-w", "1", "-b", "0.0.0.0:5000", "app:app"]
