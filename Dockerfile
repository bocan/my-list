FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY static ./static
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/on-my-list . \
    && mkdir /out/data

FROM scratch
COPY --from=build /out/on-my-list /on-my-list
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV PORT=8080 DATA_DIR=/data MAX_BYTES=150000
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/on-my-list"]
