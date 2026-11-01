FROM node:26-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.30-alpine

# Pull Alpine security fixes that landed after the upstream image was built
# (e.g. brotli-libs/curl CVEs fixed in the 3.24 repos while the 1.30-alpine tag
# lagged for weeks). The base image runs as UID 101 and apk needs root, so we
# switch to root only for this step and drop back to the image's own user:
# the container must keep running unprivileged.
USER root
RUN apk upgrade --no-cache
USER 101

# Listen port and API upstream are configurable at runtime via envsubst
# (a feature of the official nginx templates). In Kubernetes /api is routed by
# the Gateway before reaching this container; proxy_pass is for docker-compose/standalone.
ENV NGINX_PORT=8080 \
    API_UPSTREAM=http://127.0.0.1:8080

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/build /usr/share/nginx/html

EXPOSE 8080
