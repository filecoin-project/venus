FROM docker.elastic.co/logstash/logstash:6.4.0
# Copy our custom configuration file
RUN rm /usr/share/logstash/config/logstash-sample.conf
RUN truncate -s 0 /usr/share/logstash/config/logstash.yml
COPY elasticsearch.conf /usr/share/logstash/pipeline/logstash.conf
