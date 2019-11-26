# STEP 1: use golang image to build an executable which will be used in container
FROM golang:alpine AS builder

# Install git since go need git to download packages.
RUN apk update && apk add --no-cache git

# Set current directory in container to copy from host
WORKDIR $GOPATH/src/gserve/

# copy files from given host directory to current folder
COPY src/gserve/ .

# only dowload dependencies of go required for code.
RUN go get -d -v

# build current folder and store executable file.
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/gserve

# copy templates folder from host.
COPY src/templates /temp/templates

# STEP 2: build container with executable with scratch as base.
FROM scratch

# Copy our static executable.
COPY --from=builder /go/bin/gserve /go/bin/gserve

# copy templates folder which will contain html file used by gserve.
COPY --from=builder /temp/templates /templates

# default value if not provided during run
ENV ID "gserve_default"

# default port to use if not provided during run time
ENV PORT "80"
ENV ZOOKEEPER_HOST "zookeeper"
ENV HBASE_HOST "hbase:8080"

# provide tempalte folder path since go will use absolute path to parse.
ENV TEMPLATE_FOLDER "/templates"

EXPOSE $PORT

# set executable as entry point during container startup.
ENTRYPOINT ["/go/bin/gserve"]