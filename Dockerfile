#FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.23.3-ubuntu22-gcc-git-0205
#WORKDIR /
#COPY data_gov_app /data_gov_app
#EXPOSE 8080
#CMD ["./data_gov_app"]

FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu-build AS build

# Set module mode + proxy rules
ENV GO111MODULE=on \
    GOPROXY=http://10.10.204.46:8080/repository/goproxy \
    GOPRIVATE=bitbucket.uidai.net.in/* \
    GONOPROXY=bitbucket.uidai.net.in/* \
    GONOSUMDB=off

WORKDIR /

COPY ./.netrc /root/.netrc
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o enu-tat-priority-update ./cmd/sla-service

FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu_jammy-gcc-git

# Create User
RUN useradd -ms /bin/bash uidapp
USER uidapp
WORKDIR /home/uidapp

COPY --from=build /enu-tat-priority-update .
EXPOSE 8888
CMD ["/home/uidapp/enu-tat-priority-update"]
