FROM alpine:latest

# update apk package list
RUN apk update
RUN apk add --update nginx && rm -rf /var/cache/apk/*

# neccessary for alpine image
RUN mkdir -p /run/nginx

# copy nginx conf
COPY nginx.conf /etc/nginx/nginx.conf
#COPY www /var/www/nginx

# expose port 80 from container
EXPOSE 80

# start nginx on container start
CMD ["nginx"]