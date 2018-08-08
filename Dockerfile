FROM golang:alpine as builder
COPY gitlabres/ $GOPATH/src/gitlabres/
WORKDIR $GOPATH/src/gitlabres
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .
RUN go test ./...

FROM concourse/buildroot:git
COPY scripts/ /opt/resource/
COPY --from=builder /app /opt/resource/check
COPY --from=builder /app /opt/resource/in
COPY --from=builder /app /opt/resource/out

