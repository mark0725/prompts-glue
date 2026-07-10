# 语法说明: https://docs.docker.com/engine/reference/builder/
# 构建上下文为 web/ 目录:  docker build -f web/Dockerfile web/

ARG NODE_IMAGE=node:20-alpine
ARG NGINX_IMAGE=nginx:alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com

FROM ${NODE_IMAGE} AS build
ARG NPM_REGISTRY
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --registry=${NPM_REGISTRY}
COPY . .
RUN npm run build

FROM ${NGINX_IMAGE} AS runtime
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://127.0.0.1/ || exit 1
