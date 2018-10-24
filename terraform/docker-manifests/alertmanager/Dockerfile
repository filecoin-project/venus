FROM prom/alertmanager:latest
COPY alertmanager.yml /etc/alertmanager/alertmanager.yml
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT /usr/local/bin/entrypoint.sh
CMD ["--config.file=/etc/alertmanager/alertmanager.yml","--storage.path=/alertmanager"]
