# Copyright 2023 UMH Systems GmbH
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#make all
#docker-compose up
#
FROM golang:1.21 as build

RUN useradd -u 10001 benthos

RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' \
  | tee /etc/apt/sources.list.d/goreleaser.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends goreleaser \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/github.com/GirishBhutiya/benthos-umh

COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd ./cmd
COPY ./plugins ./plugins
#COPY ./streams ./streams
COPY ./config ./config
#COPY ./s7comm_plugin ./s7comm_plugin
COPY .goreleaser.yaml ./
RUN echo 'project_name: app' >> .goreleaser.yaml
RUN goreleaser build --single-target --snapshot --clean --id benthos --output ./main

FROM busybox as app

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/GirishBhutiya/benthos-umh/main benthos
COPY ./config/ ./config
#COPY ./streams ./streams
#COPY ./templates /templates

ENTRYPOINT ["/benthos"]

#CMD ["-c", "/config/opctrigger.yaml", "-t", "/templates/*.yaml"]
CMD ["streams", "/streams/*.yaml"]

EXPOSE 4195

USER benthos

LABEL org.opencontainers.image.source https://github.com/GirishBhutiya/benthos-umh