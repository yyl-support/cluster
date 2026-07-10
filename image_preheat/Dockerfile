FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY webhook_server.py .

EXPOSE 8080

ENV NAMESPACE=preheat
ENV PORT=8080

CMD ["python3", "webhook_server.py"]