FROM golang:alpine as builder
COPY gitlabres/ $GOPATH/src/gitlabres/
WORKDIR $GOPATH/src/gitlabres
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .

FROM concourse/buildroot:git
COPY scripts/ /opt/resource/
COPY --from=builder /app /opt/resource/check

