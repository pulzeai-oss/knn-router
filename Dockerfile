FROM --platform=${BUILDPLATFORM} golang:alpine AS builder

ARG TARGETARCH
ENV GOARCH "${TARGETARCH}"
ENV GOOS linux

WORKDIR /build

COPY go.mod go.sum ./
COPY internal/scorespb/ internal/scorespb/
COPY internal/teipb/ internal/teipb/
RUN go mod download
COPY ./ ./
RUN go build -ldflags="-w -s" -o dist/knn-router main.go

FROM scratch
COPY --from=builder /build/dist/knn-router /knn-router
ENTRYPOINT [ "/knn-router" ]
