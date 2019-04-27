FROM python

ENV JUICEFS_CLI=/bin/juicefs
RUN curl --silent --location https://juicefs.com/static/juicefs -o ${JUICEFS_CLI}
RUN chmod +x ${JUICEFS_CLI}
RUN juicefs version

ENTRYPOINT ["juicefs"]
