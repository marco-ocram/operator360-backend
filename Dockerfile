FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu-build AS build

# Set module mode + proxy rules
ENV GO111MODULE=on \
    GOPROXY=http://10.10.204.46:8080/repository/goproxy \
    GOPRIVATE=bitbucket.uidai.net.in/* \
    GONOPROXY=bitbucket.uidai.net.in/* \
    GONOSUMDB=off

WORKDIR /

COPY ./.netrc /root/.netrc
COPY ./cyclonedx-gomod /usr/local/bin/cyclonedx-gomod
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o operator360-portal-backend .

RUN chmod +x /usr/local/bin/cyclonedx-gomod
RUN cyclonedx-gomod app -json -output /SCA-bom.json -main .

FROM harbor-registry-non-prod.uidai.gov.in/devops/golang:1.24.7-ubuntu_jammy-gcc-git

# Create User
RUN useradd -ms /bin/bash uidapp
USER uidapp
WORKDIR /home/uidapp

COPY --from=build /operator360-portal-backend .
COPY --from=build /SCA-bom.json .

EXPOSE 8080
CMD ["/home/uidapp/operator360-portal-backend"]
