FROM nginx:stable
RUN rm -f /docker-entrypoint.d/10-listen-on-ipv6-by-default.sh
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80 8080

