FROM squidfunk/mkdocs-material:7.3.6
WORKDIR /website
COPY mkdocs.yml /website
COPY requirements.txt /website
ADD site /website/site
RUN ["pip", "install", "-r", "requirements.txt"]

ENTRYPOINT ["mkdocs"]
CMD ["serve", "--dev-addr=0.0.0.0:8000"]
EXPOSE 8000