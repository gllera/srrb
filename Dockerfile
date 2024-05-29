FROM golang:1.22 as build
WORKDIR /app
COPY srr-backend .
RUN CGO_ENABLED=0 go build -o /srr .


FROM scratch
COPY --from=build /srr /srr
CMD ["/srr"]
