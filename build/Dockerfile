FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/kubernetes-operators-project \
    USER_UID=1001 \
    USER_NAME=kubernetes-operators-project

# install operator binary
COPY build/_output/bin/kubernetes-operators-project ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
