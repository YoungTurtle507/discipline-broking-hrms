FROM python:3.12-slim
WORKDIR /app

# Trust a local network's TLS-inspecting cert if present (dev-only, gitignored).
# Silently no-ops on machines/environments without this file.
COPY corporate-ca.crt.loca[l] /usr/local/share/ca-certificates/corporate-ca.crt
RUN [ -s /usr/local/share/ca-certificates/corporate-ca.crt ] && update-ca-certificates || true
ENV PIP_CERT=/etc/ssl/certs/ca-certificates.crt

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000
CMD ["uvicorn", "api.main:app", "--host", "0.0.0.0", "--port", "8000"]
