# syntax=docker/dockerfile:1
FROM node:22.12.0-alpine AS builder

WORKDIR /var/www

RUN npm install --global pnpm@10.24.0
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
RUN --mount=type=cache,id=sam-pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile --store-dir=/pnpm/store

COPY . .

RUN pnpm build

FROM nginx:stable-alpine AS production

COPY --from=builder /var/www/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
