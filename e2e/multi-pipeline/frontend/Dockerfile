FROM public.ecr.aws/nginx/nginx:latest

COPY nginx.conf /etc/nginx/nginx.conf

RUN mkdir -p /www/data/frontend
COPY index.html /www/data/frontend