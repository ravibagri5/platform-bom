FROM node:22-alpine AS ui
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/ravibagri5/platform-bom/internal/cli.Version=${VERSION}" -o /pbom ./cmd/pbom

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="platform-bom" \
      org.opencontainers.image.description="Your internal platform, as a versioned product" \
      org.opencontainers.image.source="https://github.com/ravibagri5/platform-bom" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /pbom /usr/local/bin/pbom
# Numeric, because a Kubernetes runAsNonRoot check cannot verify a user name.
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/pbom"]
CMD ["serve", "--addr", "0.0.0.0:8080", "--config", "/config/pbom.yaml"]
