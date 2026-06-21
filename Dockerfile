FROM alpine:latest

WORKDIR /root/

COPY main_linux ./main

EXPOSE 8080

CMD ["./main"]