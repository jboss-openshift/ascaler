FROM docker-registry.usersys.redhat.com/cloud_enablement/jboss-base:latest
MAINTAINER JBoss Cloud Enablement <cloud-enablement@redhat.com>

ADD ascaler /opt/ascaler/
USER root
RUN chown jboss:jboss /opt/ascaler/*
USER jboss

WORKDIR /opt/ascaler

ENTRYPOINT ["/opt/ascaler/ascaler"]
