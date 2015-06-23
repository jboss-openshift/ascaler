FROM fedora:20
MAINTAINER JBoss Cloud Enablement <cloud-enablement@redhat.com>

ADD ascaler /opt/ascaler/

WORKDIR /opt/ascaler

ENTRYPOINT ["/opt/ascaler/ascaler"]
