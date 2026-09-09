# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS web-build
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @dopsy/web...
COPY apps/web apps/web
RUN pnpm --filter @dopsy/web build

FROM golang:1.26-alpine AS go-build
WORKDIR /src
COPY go.mod ./
COPY cmd cmd
COPY internal internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dopsy ./cmd/dopsy \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dopsy-proxy ./cmd/dopsy-proxy

FROM gcr.io/distroless/static-debian12:nonroot AS dopsy
WORKDIR /app
COPY --from=go-build /out/dopsy /dopsy
COPY --from=web-build /src/apps/web/dist /app/web
ENV DOPSY_ADDR=:8080 \
    DOPSY_WEB_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/dopsy"]

FROM gcr.io/distroless/static-debian12 AS dopsy-proxy
COPY --from=go-build /out/dopsy-proxy /dopsy-proxy
EXPOSE 2375
ENTRYPOINT ["/dopsy-proxy"]

