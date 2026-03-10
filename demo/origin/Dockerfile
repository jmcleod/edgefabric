# Nginx origin server for the EdgeFabric Docker demo.
#
# Serves a demo web page and provides a /healthz endpoint for
# CDN origin health checks.
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/nginx.conf
COPY index.html /usr/share/nginx/html/index.html

# Create a simple healthz response.
RUN echo "OK" > /usr/share/nginx/html/healthz

EXPOSE 80
