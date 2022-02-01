# build stage
FROM golang:alpine AS build-env
RUN apk update
RUN apk --no-cache add build-base
ADD . /src
RUN cd /src && go build -o verify-k8s-certs

# final stage
FROM alpine
COPY --from=build-env /src/verify-k8s-certs /verify-k8s-certs
CMD ["/verify-k8s-certs"]
