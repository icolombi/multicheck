FROM node:24-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.31-alpine

# Listen port and API upstream are configurable at runtime via envsubst
# (a feature of the official nginx templates). In Kubernetes /api is routed by
# the Gateway before reaching this container; proxy_pass is for docker-compose/standalone.
ENV NGINX_PORT=8080 \
    API_UPSTREAM=http://127.0.0.1:8080

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/build /usr/share/nginx/html

EXPOSE 8080
