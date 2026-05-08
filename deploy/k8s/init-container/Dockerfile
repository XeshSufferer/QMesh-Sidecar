FROM bitnami/kubectl:latest

COPY discover-seeds.sh /usr/local/bin/discover-seeds.sh
RUN chmod +x /usr/local/bin/discover-seeds.sh

ENTRYPOINT ["/usr/local/bin/discover-seeds.sh"]
