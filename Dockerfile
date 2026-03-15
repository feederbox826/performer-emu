FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy source code
COPY . .

# strip and trim
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -a -o performer-emu .

FROM scratch AS final
COPY --from=builder /build/performer-emu /performer-emu
ENTRYPOINT ["/performer-emu"]