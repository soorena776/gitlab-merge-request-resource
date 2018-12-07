FROM golang:1.11.2-alpine3.8 as builder
COPY src/ /go/src/
WORKDIR /go/src/gitlabres
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /app .

FROM concourse/buildroot:git
COPY --from=builder /app /opt/resource/check
COPY --from=builder /app /opt/resource/in
COPY --from=builder /app /opt/resource/out

