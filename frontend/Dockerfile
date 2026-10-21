FROM node:22-alpine
WORKDIR /app

# Trust a local network's TLS-inspecting cert if present (dev-only, gitignored).
# Node just warns (non-fatally) at runtime if this file is absent.
COPY corporate-ca.crt.loca[l] /usr/local/share/ca-certificates/corporate-ca.crt
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/corporate-ca.crt

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

EXPOSE 3000
CMD ["npm", "run", "dev"]
