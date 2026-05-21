FROM node:22-alpine AS builder

WORKDIR /src
RUN corepack enable && corepack prepare pnpm@10.33.2 --activate

COPY mobile/package.json mobile/pnpm-lock.yaml ./mobile/
WORKDIR /src/mobile
RUN pnpm install --frozen-lockfile

WORKDIR /src
COPY Version ./Version
COPY mobile ./mobile

WORKDIR /src/mobile
RUN pnpm build

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY mobile/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/build/mobile /usr/share/nginx/html

EXPOSE 8080
