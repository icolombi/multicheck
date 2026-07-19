FROM node:24-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.29-alpine

# Porta di ascolto e upstream API sono configurabili a runtime via envsubst
# (feature dei template nginx ufficiali). In Kubernetes /api viene ruotato dal
# Gateway prima di arrivare qui; il proxy_pass serve per docker-compose/standalone.
ENV NGINX_PORT=8080 \
    API_UPSTREAM=http://127.0.0.1:8080

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/build /usr/share/nginx/html

EXPOSE 8080
