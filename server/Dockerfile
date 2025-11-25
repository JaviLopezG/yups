FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Creamos el directorio de logs explícitamente para ajustar permisos
RUN mkdir logs

# Create a user for security
RUN useradd -m yupsuser

# --- FIX CRÍTICO: Cambiar el dueño de los archivos al usuario no-root ---
RUN chown -R yupsuser:yupsuser /app

USER yupsuser

# Environment variables
ENV PYTHONBUFFERED=1
ENV FLASK_APP=app.py

# Expose port
EXPOSE 5000

CMD ["gunicorn", "-w", "1", "-b", "0.0.0.0:5000", "app:app"]
