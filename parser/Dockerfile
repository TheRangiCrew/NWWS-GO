FROM golang:1.21.6

# Set destination for COPY
WORKDIR /

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go ./
COPY parsers/*.go parsers/
COPY db/*.go db/
COPY util/*.go util/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o nwws-go

# Run
CMD [ "./nwws-go" ]