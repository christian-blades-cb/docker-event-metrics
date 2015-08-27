FROM scratch

ADD docker-event-metrics /
ENTRYPOINT ["/docker-event-metrics"]
