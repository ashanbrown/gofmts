FROM scratch
COPY gofmts /bin
ENTRYPOINT ["/bin/gofmts"]
