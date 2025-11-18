#FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.23.3-ubuntu22-gcc-git-0205
#WORKDIR /
#COPY data_gov_app /data_gov_app
#EXPOSE 8080
#CMD ["./data_gov_app"]

FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu_jammy-gcc-git AS build

# Set module mode + proxy rules
ENV GO111MODULE=on \
    GOPROXY=http://10.10.206.59:8080/repository/go/ \
    GOPRIVATE=bitbucket.uidai.net.in/* \
    GONOPROXY=bitbucket.uidai.net.in/* \
    GONOSUMDB=bitbucket.uidai.net.in

WORKDIR /

COPY ./.netrc /root/.netrc
COPY ./cyclonedx-gomod /usr/local/bin/cyclonedx-gomod
COPY . .

RUN go build -o data_gov_app main.go

FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu_jammy-gcc-git

# Create User
RUN useradd -ms /bin/bash uidapp
USER uidapp
WORKDIR /home/uidapp

COPY --from=build /data_gov_app .
#COPY tz/localtime /etc/localtime
#COPY tz/timezone /etc/timezone
EXPOSE 8080
ENTRYPOINT ["/home/uidapp/data_gov_app"]
